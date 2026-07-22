package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

// --- Announcement use cases ---

// CreateAnnouncementRequest is the input for creating an announcement.
type CreateAnnouncementRequest struct {
	Title    string
	Body     string
	Audience string
}

// CreateAnnouncement validates and persists a new Announcement, then publishes
// it to the tenant's in-app inbox through the standard message machinery
// (pending message → deliver via the channel notifier → notification.sent.v1).
func (s *Service) CreateAnnouncement(ctx context.Context, actor auth.Actor, req CreateAnnouncementRequest) (*domain.Announcement, error) {
	tenantID, err := s.requireAnnouncementAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	a, err := domain.NewAnnouncement(tenantID, req.Title, req.Body, req.Audience)
	if err != nil {
		return nil, err
	}
	if err := s.announcementRepo.Create(ctx, tenantID, a); err != nil {
		return nil, err
	}

	// Publish to the tenant in-app inbox. The announcement row is the record of
	// truth; inbox delivery failures are logged, not rolled back.
	m, err := domain.NewMessage(tenantID, tenantID, string(domain.ChannelInApp), a.Title, a.Body, nil, map[string]any{
		"announcement_id": a.ID,
		"audience":        a.Audience,
		"source":          "announcement",
	}, nil)
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to build announcement inbox message", "announcement_id", a.ID, "err", err)
		return a, nil
	}
	if err := s.messageRepo.Create(ctx, tenantID, m); err != nil {
		slog.Default().ErrorContext(ctx, "failed to persist announcement inbox message", "announcement_id", a.ID, "err", err)
		return a, nil
	}
	if _, err := s.deliver(ctx, tenantID, m); err != nil {
		slog.Default().ErrorContext(ctx, "failed to deliver announcement inbox message", "announcement_id", a.ID, "message_id", m.ID, "err", err)
	}
	return a, nil
}

// ListAnnouncements returns a tenant-scoped page of announcements.
func (s *Service) ListAnnouncements(ctx context.Context, actor auth.Actor, filter ports.AnnouncementFilter) ([]*domain.Announcement, string, error) {
	tenantID, err := s.requireAnnouncementAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	if audience := roleAudience(actor.Role); audience != "" {
		if filter.Audience != "" && filter.Audience != "all" && filter.Audience != audience {
			return nil, "", domain.ErrForbidden
		}
		filter.Audience = ""
		filter.Audiences = []string{"all", audience}
	}
	return s.announcementRepo.List(ctx, tenantID, filter)
}

// GetAnnouncement returns a single announcement.
func (s *Service) GetAnnouncement(ctx context.Context, actor auth.Actor, id string) (*domain.Announcement, error) {
	tenantID, err := s.requireAnnouncementAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	announcement, err := s.announcementRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if audience := roleAudience(actor.Role); audience != "" && announcement.Audience != "all" && announcement.Audience != audience {
		return nil, domain.ErrNotFound
	}
	return announcement, nil
}

func roleAudience(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "teacher":
		return "staff"
	case "parent":
		return "guardians"
	case "student":
		return "students"
	default:
		return ""
	}
}

// DeleteAnnouncement removes an announcement.
func (s *Service) DeleteAnnouncement(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAnnouncementAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if _, err := s.announcementRepo.GetByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.announcementRepo.Delete(ctx, tenantID, id)
}

// requireAnnouncementAccess applies the standard tenant/RBAC/service-gate checks
// plus the announcements feature flag.
func (s *Service) requireAnnouncementAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	tenantID, err := s.requireAccess(ctx, actor, perm)
	if err != nil {
		return "", err
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureAnnouncements) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureAnnouncements)
	}
	return tenantID, nil
}
