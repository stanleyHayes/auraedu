package unit

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
)

func (r *fakeMessageRepo) ClaimDue(_ context.Context, _ int, _ time.Duration) ([]*domain.Message, error) {
	return nil, nil
}

func (r *fakeMessageRepo) CancelByApplication(_ context.Context, tenantID, applicationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, message := range r.messages {
		if message.TenantID == tenantID && message.Status == string(domain.MessageStatusPending) && message.Metadata["application_id"] == applicationID {
			message.Status = string(domain.MessageStatusCancelled)
		}
	}
	return nil
}

func (r *fakeMessageRepo) NextJourneyDeliveryAllowedAt(context.Context, string, string, string, time.Duration, int) (*time.Time, error) {
	return nil, nil
}

// In-memory repository fakes for application-layer unit tests. They scope every
// lookup by tenantID, mirroring the Postgres adapters' tenant contract.

type fakeMessageRepo struct {
	mu           sync.Mutex
	messages     map[string]*domain.Message
	suppressions map[string]bool
	feedback     []ports.DeliveryFeedback
	creates      int
}

func newFakeMessageRepo() *fakeMessageRepo {
	return &fakeMessageRepo{messages: map[string]*domain.Message{}, suppressions: map[string]bool{}}
}

func (r *fakeMessageRepo) IsEmailSuppressed(_ context.Context, tenantID, addressHash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.suppressions[tenantID+"/"+addressHash], nil
}

func (r *fakeMessageRepo) ApplyDeliveryFeedback(_ context.Context, feedback ports.DeliveryFeedback) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.feedback {
		if existing.ID == feedback.ID {
			return false, nil
		}
	}
	r.feedback = append(r.feedback, feedback)
	return true, nil
}

func (r *fakeMessageRepo) SuppressEmail(_ context.Context, tenantID, addressHash, _ string, _ string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suppressions[tenantID+"/"+addressHash] = true
	return nil
}

func (r *fakeMessageRepo) Create(_ context.Context, tenantID string, m *domain.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages[tenantID+"/"+m.ID] = m
	r.creates++
	return nil
}

func (r *fakeMessageRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.messages[tenantID+"/"+id]; ok {
		return m, nil
	}
	return nil, domain.ErrNotFound
}

func (r *fakeMessageRepo) List(_ context.Context, tenantID string, filter ports.MessageFilter) ([]*domain.Message, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Message
	for _, m := range r.messages {
		if m.TenantID != tenantID {
			continue
		}
		if filter.Channel != "" && m.Channel != filter.Channel {
			continue
		}
		if filter.Status != "" && m.Status != filter.Status {
			continue
		}
		if filter.RecipientID != "" && m.RecipientID != filter.RecipientID {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, "", nil
}

func (r *fakeMessageRepo) Update(_ context.Context, tenantID string, m *domain.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.messages[tenantID+"/"+m.ID]; !ok {
		return domain.ErrNotFound
	}
	r.messages[tenantID+"/"+m.ID] = m
	return nil
}

func (r *fakeMessageRepo) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.messages, tenantID+"/"+id)
	return nil
}

// all returns every message stored for the tenant, ignoring filters.
func (r *fakeMessageRepo) all(tenantID string) []*domain.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Message
	for _, m := range r.messages {
		if m.TenantID == tenantID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type fakeSubscriptionRepo struct {
	mu   sync.Mutex
	subs map[string]*domain.Subscription
}

func newFakeSubscriptionRepo() *fakeSubscriptionRepo {
	return &fakeSubscriptionRepo{subs: map[string]*domain.Subscription{}}
}

func (r *fakeSubscriptionRepo) Create(_ context.Context, tenantID string, s *domain.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs[tenantID+"/"+s.ID] = s
	return nil
}

func (r *fakeSubscriptionRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.subs[tenantID+"/"+id]; ok {
		return s, nil
	}
	return nil, domain.ErrNotFound
}

func (r *fakeSubscriptionRepo) List(_ context.Context, tenantID string, filter ports.SubscriptionFilter) ([]*domain.Subscription, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Subscription
	for _, s := range r.subs {
		if s.TenantID != tenantID {
			continue
		}
		if filter.Channel != "" && s.Channel != filter.Channel {
			continue
		}
		if filter.UserID != "" && s.UserID != filter.UserID {
			continue
		}
		out = append(out, s)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, "", nil
}

func (r *fakeSubscriptionRepo) Update(_ context.Context, tenantID string, s *domain.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.subs[tenantID+"/"+s.ID]; !ok {
		return domain.ErrNotFound
	}
	r.subs[tenantID+"/"+s.ID] = s
	return nil
}

func (r *fakeSubscriptionRepo) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subs, tenantID+"/"+id)
	return nil
}

// add seeds an enabled/disabled subscription for (tenant, user, channel).
func (r *fakeSubscriptionRepo) add(tenantID, userID, channel string, enabled bool) {
	sub, err := domain.NewSubscription(tenantID, userID, channel, enabled)
	if err != nil {
		panic(err)
	}
	r.subs[tenantID+"/"+sub.ID] = sub
}

type fakeProcessedEventRepo struct {
	mu       sync.Mutex
	claims   map[string]string
	releases int
}

func newFakeProcessedEventRepo() *fakeProcessedEventRepo {
	return &fakeProcessedEventRepo{claims: map[string]string{}}
}

func (r *fakeProcessedEventRepo) Claim(_ context.Context, tenantID, eventID, eventType string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := tenantID + "/" + eventID
	if _, ok := r.claims[key]; ok {
		return false, nil
	}
	r.claims[key] = eventType
	return true, nil
}

func (r *fakeProcessedEventRepo) Release(_ context.Context, tenantID, eventID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.claims, tenantID+"/"+eventID)
	r.releases++
	return nil
}

func (r *fakeProcessedEventRepo) claimed(eventID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.claims[workerTenant+"/"+eventID]
	return ok
}

type fakeAnnouncementRepo struct {
	mu            sync.Mutex
	announcements map[string]*domain.Announcement
}

func newFakeAnnouncementRepo() *fakeAnnouncementRepo {
	return &fakeAnnouncementRepo{announcements: map[string]*domain.Announcement{}}
}

func (r *fakeAnnouncementRepo) Create(_ context.Context, tenantID string, a *domain.Announcement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.announcements[tenantID+"/"+a.ID] = a
	return nil
}

func (r *fakeAnnouncementRepo) GetByID(_ context.Context, tenantID, id string) (*domain.Announcement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.announcements[tenantID+"/"+id]; ok {
		return a, nil
	}
	return nil, domain.ErrNotFound
}

func (r *fakeAnnouncementRepo) List(_ context.Context, tenantID string, filter ports.AnnouncementFilter) ([]*domain.Announcement, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Announcement
	for _, a := range r.announcements {
		if a.TenantID != tenantID {
			continue
		}
		if filter.Audience != "" && a.Audience != filter.Audience {
			continue
		}
		if len(filter.Audiences) > 0 {
			allowed := false
			for _, audience := range filter.Audiences {
				if a.Audience == audience {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, "", nil
}

func (r *fakeAnnouncementRepo) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.announcements, tenantID+"/"+id)
	return nil
}
