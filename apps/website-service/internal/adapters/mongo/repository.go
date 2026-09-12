// Package mongo persists website aggregates in MongoDB.
//
// It implements the same ports as the Postgres adapter and is selected by
// platform/store, so neither driver is privileged and switching is config.
//
// Tenant isolation comes from platform/mongo.Scope rather than row-level
// security: a query that is not scoped cannot be written.
package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	PagesCollection    = "website_pages"
	SectionsCollection = "website_sections"
	outboxLease        = 5 * time.Minute
	eventSource        = "website-service"
)

type Repository struct {
	store       *pmongo.Store
	pages       *pmongo.Collection
	sections    *pmongo.Collection
	pagesOutbox *pmongo.ClaimableOutbox
	secOutbox   *pmongo.ClaimableOutbox
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

func NewRepository(store *pmongo.Store) *Repository {
	pages := store.Collection(PagesCollection)
	sections := store.Collection(SectionsCollection)
	return &Repository{store: store,
		pages:       pages,
		sections:    sections,
		pagesOutbox: pmongo.NewClaimableOutbox(pages, outboxLease),
		secOutbox:   pmongo.NewClaimableOutbox(sections, outboxLease),
	}
}

// pageDoc mirrors the website_pages table so both adapters describe the same record.
type pageDoc struct {
	ID              string     `bson:"_id"`
	TenantID        string     `bson:"tenant_id"`
	Slug            string     `bson:"slug"`
	Title           string     `bson:"title"`
	Status          string     `bson:"status"`
	MetaDescription *string    `bson:"meta_description,omitempty"`
	Layout          string     `bson:"layout"`
	CreatedAt       time.Time  `bson:"created_at"`
	UpdatedAt       time.Time  `bson:"updated_at"`
	PublishedAt     *time.Time `bson:"published_at,omitempty"`
}

func (d pageDoc) toDomain() *domain.Page {
	return &domain.Page{
		ID: d.ID, TenantID: d.TenantID, Slug: d.Slug, Title: d.Title,
		Status: d.Status, MetaDescription: d.MetaDescription, Layout: d.Layout,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, PublishedAt: d.PublishedAt,
	}
}

// sectionDoc mirrors the website_sections table so both adapters describe the same record.
type sectionDoc struct {
	ID        string         `bson:"_id"`
	TenantID  string         `bson:"tenant_id"`
	PageID    string         `bson:"page_id"`
	Type      string         `bson:"type"`
	Content   map[string]any `bson:"content,omitempty"`
	SortOrder int            `bson:"sort_order"`
	Status    string         `bson:"status"`
	CreatedAt time.Time      `bson:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at"`
}

func (d sectionDoc) toDomain() *domain.Section {
	return &domain.Section{
		ID: d.ID, TenantID: d.TenantID, PageID: d.PageID, Type: d.Type,
		Content: domain.Content(d.Content), SortOrder: d.SortOrder, Status: d.Status,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// live excludes tombstoned records. A delete keeps its document until the
// delete event has been published, so without this every read would keep
// returning records the Postgres adapter has already removed.
func live(query bson.M) bson.M {
	merged := bson.M{}
	for k, v := range query {
		merged[k] = v
	}
	merged["deleted_at"] = bson.M{"$exists": false}
	return merged
}

func pageFields(p *domain.Page) bson.M {
	return bson.M{
		"slug": p.Slug, "title": p.Title, "status": p.Status,
		"meta_description": p.MetaDescription, "layout": p.Layout,
		"created_at": p.CreatedAt, "updated_at": p.UpdatedAt, "published_at": p.PublishedAt,
	}
}

func sectionFields(s *domain.Section) bson.M {
	content := bson.M{}
	for k, v := range s.Content {
		content[k] = v
	}
	return bson.M{
		"type": s.Type, "content": content, "sort_order": s.SortOrder,
		"status": s.Status, "created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
	}
}

// Pages.

func (r *Repository) CreatePage(ctx context.Context, tenantID string, p *domain.Page) error {
	scope, err := r.pages.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("website: create page: %w", err)
	}
	doc := pageFields(p)
	doc["_id"] = p.ID
	if _, err := scope.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("website: create page: %w", err)
	}
	return nil
}

func (r *Repository) GetPageByID(ctx context.Context, tenantID, id string) (*domain.Page, error) {
	return r.getOnePage(ctx, tenantID, bson.M{"_id": id})
}

func (r *Repository) GetPageBySlug(ctx context.Context, tenantID, slug string) (*domain.Page, error) {
	return r.getOnePage(ctx, tenantID, bson.M{"slug": domain.NormalizeSlug(slug)})
}

func (r *Repository) getOnePage(ctx context.Context, tenantID string, query bson.M) (*domain.Page, error) {
	scope, err := r.pages.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("website: get page: %w", err)
	}
	var doc pageDoc
	if err := scope.FindOne(ctx, live(query)).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("website: get page: %w", err)
	}
	return doc.toDomain(), nil
}

// ListPages pages by creation time ascending then id, matching the Postgres adapter's keyset order.
func (r *Repository) ListPages(ctx context.Context, tenantID string, limit int, cursor string, filter ports.PageFilter) ([]*domain.Page, string, error) {
	scope, err := r.pages.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("website: list pages: %w", err)
	}

	// Normalize limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := bson.M{}
	if filter.Status != nil && *filter.Status != "" {
		query["status"] = *filter.Status
	}
	if filter.Layout != nil && *filter.Layout != "" {
		query["layout"] = *filter.Layout
	}

	if cursor != "" {
		// Keyset pagination: find the cursor document's created_at and id, then continue from there
		var cursorDoc pageDoc
		if err := scope.FindOne(ctx, live(bson.M{"_id": cursor})).Decode(&cursorDoc); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, "", domain.ErrNotFound
			}
			return nil, "", fmt.Errorf("website: get cursor page: %w", err)
		}
		query["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$gt": cursorDoc.CreatedAt}},
			bson.M{"created_at": cursorDoc.CreatedAt, "_id": bson.M{"$gt": cursor}},
		}
	}

	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{
			{Key: "created_at", Value: 1},
			{Key: "_id", Value: 1},
		}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("website: list pages: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.Page
	for cur.Next(ctx) {
		var doc pageDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("website: list pages decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("website: list pages rows: %w", err)
	}

	var nextCursor string
	if len(out) == limit && len(out) > 0 {
		nextCursor = out[len(out)-1].ID
	}
	return out, nextCursor, nil
}

func (r *Repository) UpdatePage(ctx context.Context, tenantID string, p *domain.Page) error {
	scope, err := r.pages.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("website: update page: %w", err)
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": p.ID}), bson.M{"$set": pageFields(p)})
	if err != nil {
		return fmt.Errorf("website: update page: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) DeletePage(ctx context.Context, tenantID, id string) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		scope, err := r.pages.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("website: delete page: %w", err)
		}
		res, err := scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}})
		if err != nil {
			return fmt.Errorf("website: delete page: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
		return r.DeleteSectionsByPage(ctx, tenantID, id)
	})
}

// Sections.

func (r *Repository) CreateSection(ctx context.Context, tenantID string, s *domain.Section) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.touchPage(ctx, tenantID, s.PageID); err != nil {
			return err
		}

		scope, err := r.sections.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("website: create section: %w", err)
		}
		doc := sectionFields(s)
		doc["_id"] = s.ID
		doc["page_id"] = s.PageID
		if _, err := scope.InsertOne(ctx, doc); err != nil {
			return fmt.Errorf("website: create section: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetSectionByID(ctx context.Context, tenantID, id string) (*domain.Section, error) {
	scope, err := r.sections.ScopeTo(tenantID)
	if err != nil {
		return nil, fmt.Errorf("website: get section: %w", err)
	}
	var doc sectionDoc
	if err := scope.FindOne(ctx, live(bson.M{"_id": id})).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("website: get section: %w", err)
	}
	return doc.toDomain(), nil
}

// ListSections pages by sort_order ascending then id, matching the Postgres adapter's keyset order.
func (r *Repository) ListSections(
	ctx context.Context,
	tenantID, pageID string,
	limit int,
	cursor string,
	filter ports.SectionFilter,
) ([]*domain.Section, string, error) {
	scope, err := r.sections.ScopeTo(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("website: list sections: %w", err)
	}

	// Normalize limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := bson.M{"page_id": pageID}
	if filter.Status != nil && *filter.Status != "" {
		query["status"] = *filter.Status
	}
	if filter.Type != nil && *filter.Type != "" {
		query["type"] = *filter.Type
	}

	if cursor != "" {
		// Keyset pagination: find the cursor document's sort_order and id, then continue from there
		var cursorDoc sectionDoc
		if err := scope.FindOne(ctx, live(bson.M{"_id": cursor})).Decode(&cursorDoc); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, "", domain.ErrNotFound
			}
			return nil, "", fmt.Errorf("website: get cursor section: %w", err)
		}
		query["$or"] = bson.A{
			bson.M{"sort_order": bson.M{"$gt": cursorDoc.SortOrder}},
			bson.M{"sort_order": cursorDoc.SortOrder, "_id": bson.M{"$gt": cursor}},
		}
	}

	cur, err := scope.Find(ctx, live(query),
		options.Find().SetSort(bson.D{
			{Key: "sort_order", Value: 1},
			{Key: "_id", Value: 1},
		}).SetLimit(int64(limit)))
	if err != nil {
		return nil, "", fmt.Errorf("website: list sections: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []*domain.Section
	for cur.Next(ctx) {
		var doc sectionDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, "", fmt.Errorf("website: list sections decode: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, "", fmt.Errorf("website: list sections rows: %w", err)
	}

	var nextCursor string
	if len(out) == limit && len(out) > 0 {
		nextCursor = out[len(out)-1].ID
	}
	return out, nextCursor, nil
}

func (r *Repository) UpdateSection(ctx context.Context, tenantID string, s *domain.Section) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		if err := r.touchPage(ctx, tenantID, s.PageID); err != nil {
			return err
		}

		scope, err := r.sections.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("website: update section: %w", err)
		}
		res, err := scope.UpdateOne(ctx, live(bson.M{"_id": s.ID}), bson.M{"$set": sectionFields(s)})
		if err != nil {
			return fmt.Errorf("website: update section: %w", err)
		}
		if res.MatchedCount != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *Repository) DeleteSection(ctx context.Context, tenantID, id string) error {
	scope, err := r.sections.ScopeTo(tenantID)
	if err != nil {
		return fmt.Errorf("website: delete section: %w", err)
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}})
	if err != nil {
		return fmt.Errorf("website: delete section: %w", err)
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteSectionsByPage(ctx context.Context, tenantID, pageID string) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		scope, err := r.sections.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("website: delete sections by page: %w", err)
		}
		_, err = scope.UpdateMany(ctx, live(bson.M{"page_id": pageID}), bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}})
		if err != nil {
			return fmt.Errorf("website: delete sections by page: %w", err)
		}
		return nil
	})
}

// Lifecycle and outbox.

func lifecycleEvent(tenantID, eventType string, payload map[string]any) (tenancy.CloudEvent, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return tenancy.CloudEvent{}, fmt.Errorf("website: encode lifecycle event: %w", err)
	}
	return tenancy.CloudEvent{
		SpecVersion: "1.0", Type: eventType, Source: eventSource,
		ID: uuid.NewString(), Time: time.Now().UTC().Format(time.RFC3339),
		TenantID: tenantID, Data: encoded,
	}, nil
}

// CommitWebsiteLifecycle applies the mutation and queues its event(s) in one write.
//
// A delete is the exception: removing the document would remove the event with it,
// so the record is tombstoned instead and the relay clears it once the event is
// published.
func (r *Repository) CommitWebsiteLifecycle(
	ctx context.Context,
	tenantID string,
	mutation string,
	page *domain.Page,
	section *domain.Section,
	events []ports.LifecycleEvent,
) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		// Build CloudEvents from LifecycleEvents
		cloudEvents := make([]tenancy.CloudEvent, 0, len(events))
		for _, e := range events {
			evt, err := lifecycleEvent(tenantID, e.EventType, e.Payload)
			if err != nil {
				return err
			}
			cloudEvents = append(cloudEvents, evt)
		}

		switch mutation {
		case ports.WebsiteMutationPageCreate:
			scope, err := r.pages.ScopeTo(tenantID)
			if err != nil {
				return fmt.Errorf("website: lifecycle: %w", err)
			}
			doc := pageFields(page)
			doc["_id"] = page.ID
			if _, err := scope.InsertWithEvents(ctx, doc, cloudEvents...); err != nil {
				return fmt.Errorf("website: lifecycle create page: %w", err)
			}

		case ports.WebsiteMutationPageUpdate:
			scope, err := r.pages.ScopeTo(tenantID)
			if err != nil {
				return fmt.Errorf("website: lifecycle: %w", err)
			}
			res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": page.ID}),
				bson.M{"$set": pageFields(page)}, cloudEvents...)
			if err != nil {
				return fmt.Errorf("website: lifecycle update page: %w", err)
			}
			if res.MatchedCount != 1 {
				return domain.ErrNotFound
			}

		case ports.WebsiteMutationPageDelete:
			scope, err := r.pages.ScopeTo(tenantID)
			if err != nil {
				return fmt.Errorf("website: lifecycle: %w", err)
			}
			res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": page.ID, "deleted_at": bson.M{"$exists": false}},
				bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, cloudEvents...)
			if err != nil {
				return fmt.Errorf("website: lifecycle delete page: %w", err)
			}
			if res.MatchedCount != 1 {
				return domain.ErrNotFound
			}

			if err := r.DeleteSectionsByPage(ctx, tenantID, page.ID); err != nil {
				return err
			}
		case ports.WebsiteMutationSectionCreate:
			if err := r.touchPage(ctx, tenantID, section.PageID); err != nil {
				return err
			}
			scope, err := r.sections.ScopeTo(tenantID)
			if err != nil {
				return fmt.Errorf("website: lifecycle: %w", err)
			}
			doc := sectionFields(section)
			doc["_id"] = section.ID
			doc["page_id"] = section.PageID
			if _, err := scope.InsertWithEvents(ctx, doc, cloudEvents...); err != nil {
				return fmt.Errorf("website: lifecycle create section: %w", err)
			}

		case ports.WebsiteMutationSectionUpdate:
			if err := r.touchPage(ctx, tenantID, section.PageID); err != nil {
				return err
			}
			scope, err := r.sections.ScopeTo(tenantID)
			if err != nil {
				return fmt.Errorf("website: lifecycle: %w", err)
			}
			res, err := scope.UpdateWithEvents(ctx, live(bson.M{"_id": section.ID}),
				bson.M{"$set": sectionFields(section)}, cloudEvents...)
			if err != nil {
				return fmt.Errorf("website: lifecycle update section: %w", err)
			}
			if res.MatchedCount != 1 {
				return domain.ErrNotFound
			}

		case ports.WebsiteMutationSectionDelete:
			scope, err := r.sections.ScopeTo(tenantID)
			if err != nil {
				return fmt.Errorf("website: lifecycle: %w", err)
			}
			res, err := scope.UpdateWithEvents(ctx, bson.M{"_id": section.ID, "deleted_at": bson.M{"$exists": false}},
				bson.M{"$set": bson.M{"deleted_at": time.Now().UTC()}}, cloudEvents...)
			if err != nil {
				return fmt.Errorf("website: lifecycle delete section: %w", err)
			}
			if res.MatchedCount != 1 {
				return domain.ErrNotFound
			}

		default:
			return fmt.Errorf("website: unsupported lifecycle mutation %q", mutation)
		}
		return nil
	})
}

// ProvisionDefaultWebsite creates initial page and section atomically.
func (r *Repository) ProvisionDefaultWebsite(
	ctx context.Context,
	tenantID string,
	page *domain.Page,
	section *domain.Section,
	events []ports.LifecycleEvent,
) error {
	return r.store.WithTransaction(ctx, func(ctx context.Context) error {
		// Build CloudEvents from LifecycleEvents
		cloudEvents := make([]tenancy.CloudEvent, 0, len(events))
		for _, e := range events {
			evt, err := lifecycleEvent(tenantID, e.EventType, e.Payload)
			if err != nil {
				return err
			}
			cloudEvents = append(cloudEvents, evt)
		}

		scope, err := r.pages.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("website: provision: %w", err)
		}

		pageDoc := pageFields(page)
		pageDoc["_id"] = page.ID

		if _, err := scope.InsertOne(ctx, pageDoc); err != nil {
			return fmt.Errorf("website: provision insert page: %w", err)
		}

		secScope, err := r.sections.ScopeTo(tenantID)
		if err != nil {
			return fmt.Errorf("website: provision: %w", err)
		}

		secDoc := sectionFields(section)
		secDoc["_id"] = section.ID
		secDoc["page_id"] = section.PageID

		if _, err := secScope.InsertWithEvents(ctx, secDoc, cloudEvents...); err != nil {
			return fmt.Errorf("website: provision insert section: %w", err)
		}

		return nil
	})
}

func claimed(events []pmongo.ClaimedEvent) []ports.OutboxEvent {
	out := make([]ports.OutboxEvent, 0, len(events))
	for _, c := range events {
		out = append(out, ports.OutboxEvent{
			ID: c.Event.ID, TenantID: c.Event.TenantID,
			EventType: c.Event.Type, Payload: json.RawMessage(c.Event.Data),
		})
	}
	return out
}

func (r *Repository) ClaimPendingWebsiteEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	fromPages, err := r.pagesOutbox.Claim(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("website: claim pages outbox: %w", err)
	}
	out := claimed(fromPages)
	if len(out) >= limit {
		return out, nil
	}
	fromSections, err := r.secOutbox.Claim(ctx, limit-len(out))
	if err != nil {
		return nil, fmt.Errorf("website: claim sections outbox: %w", err)
	}
	return append(out, claimed(fromSections)...), nil
}

// MarkWebsiteEventPublished acknowledges an event from either collection. The port
// carries only the event id, so both are asked; acknowledging an event that is
// already gone is normal for an at-least-once outbox.
func (r *Repository) MarkWebsiteEventPublished(ctx context.Context, id string) error {
	if err := r.pagesOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("website: mark published: %w", err)
	}
	if err := r.secOutbox.MarkPublishedByEventID(ctx, id); err != nil {
		return fmt.Errorf("website: mark section published: %w", err)
	}
	return r.clearPublishedTombstones(ctx)
}

func (r *Repository) MarkWebsiteEventFailed(ctx context.Context, id, reason string) error {
	if err := r.pagesOutbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("website: mark failed: %w", err)
	}
	if err := r.secOutbox.MarkFailedByEventID(ctx, id, reason); err != nil {
		return fmt.Errorf("website: mark section failed: %w", err)
	}
	return nil
}

// clearPublishedTombstones removes deleted records whose delete event has been
// delivered. Holding the tombstone until then is what lets a delete stay atomic
// with its event.
func (r *Repository) clearPublishedTombstones(ctx context.Context) error {
	_, err := r.pagesOutbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}})
	if err != nil {
		return fmt.Errorf("website: clear page tombstones: %w", err)
	}
	_, err = r.secOutbox.PurgeSettled(ctx, bson.M{"deleted_at": bson.M{"$exists": true}})
	if err != nil {
		return fmt.Errorf("website: clear section tombstones: %w", err)
	}
	return nil
}

// EnsureIndexes replaces the CREATE INDEX statements in the Postgres migrations.
func EnsureIndexes(ctx context.Context, store *pmongo.Store) error {
	specs := map[string][]bson.D{
		PagesCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "slug", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}},
			{{Key: "tenant_id", Value: 1}, {Key: "layout", Value: 1}},
			{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}},
		},
		SectionsCollection: {
			{{Key: "tenant_id", Value: 1}, {Key: "page_id", Value: 1}, {Key: "_id", Value: 1}},
			{{Key: "type", Value: 1}},
			{{Key: "status", Value: 1}},
			{{Key: "sort_order", Value: 1}, {Key: "_id", Value: 1}},
		},
	}
	for name, keys := range specs {
		models := make([]mongo.IndexModel, 0, len(keys))
		for _, k := range keys {
			models = append(models, mongo.IndexModel{Keys: k})
		}
		if name == PagesCollection {
			models = append(models,
				mongo.IndexModel{Keys: bson.D{{Key: "tenant_id",
					Value: 1},
					{Key: "slug",
						Value: 1}},
					Options: options.Index().SetName("website_live_slug_unique").SetUnique(true).SetPartialFilterExpression(bson.M{"deleted_at": nil})})
		}
		if _, err := store.Database().Collection(name).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("website: ensure indexes on %s: %w", name, err)
		}
	}
	return nil
}

func (r *Repository) touchPage(ctx context.Context, tenant, id string) error {
	scope, err := r.pages.ScopeTo(tenant)
	if err != nil {
		return err
	}
	res, err := scope.UpdateOne(ctx, live(bson.M{"_id": id}), bson.M{"$inc": bson.M{"_reference_version": 1}})
	if err != nil {
		return err
	}
	if res.MatchedCount != 1 {
		return domain.ErrNotFound
	}
	return nil
}
