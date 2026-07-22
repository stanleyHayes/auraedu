// Package application contains Admissions Service use cases and policy.
package application

import (
	"context"
	"strings"
	"time"

	"github.com/auraedu/admissions-service/internal/domain"
	"github.com/auraedu/admissions-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

const FeatureAdmissions = "admissions"
const (
	PermCreate      = "admissions.application.create"
	PermCatalogue   = "admissions.catalogue.manage"
	PermRead        = "admissions.application.read"
	PermUpdate      = "admissions.application.update"
	PermSubmit      = "admissions.application.submit"
	PermReview      = "admissions.application.review"
	PermOfferIssue  = "admissions.offer.issue"
	PermOfferAccept = "admissions.offer.accept"
)

type Service struct {
	repo      ports.Repository
	catalogue ports.CatalogueRepository
	pub       ports.EventPublisher
	gate      flags.Gate
	verifier  ports.DocumentVerifier
	now       func() time.Time
}
type Option func(*Service)
type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, string, string, map[string]any) error { return nil }
func WithPublisher(p ports.EventPublisher) Option                                   { return func(s *Service) { s.pub = p } }
func WithFeatureGate(g flags.Gate) Option                                           { return func(s *Service) { s.gate = g } }
func WithDocumentVerifier(v ports.DocumentVerifier) Option {
	return func(s *Service) { s.verifier = v }
}
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }
func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, pub: noopPublisher{}, now: time.Now}
	if catalogue, ok := repo.(ports.CatalogueRepository); ok {
		s.catalogue = catalogue
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
func (s *Service) allowed(ctx context.Context, a auth.Actor, p string) bool {
	return a.Authenticated() && a.TenantID != "" && a.Has(p) && (s.gate == nil || s.gate.IsEnabled(ctx, a.TenantID, FeatureAdmissions))
}
func isAI(a auth.Actor) bool {
	return strings.Contains(strings.ToLower(a.Role), "ai") || strings.Contains(strings.ToLower(a.Role), "service_account")
}

func (s *Service) Start(ctx context.Context, a auth.Actor, leadID *string, programmeID, intakeID string) (domain.Application, error) {
	if !s.allowed(ctx, a, PermCreate) {
		return domain.Application{}, domain.ErrForbidden
	}
	if s.catalogue == nil {
		return domain.Application{}, domain.ErrUnavailable
	}
	programme, intake, err := s.catalogue.ResolveAvailableIntake(ctx, a.TenantID, programmeID, intakeID, s.now())
	if err != nil {
		return domain.Application{}, err
	}
	item, err := domain.New(a.TenantID, a.UserID, leadID, programmeID, intakeID, programme.Name, intake.Name, s.now())
	if err != nil {
		return domain.Application{}, err
	}
	payload := ports.ApplicationEventData("application.started.v1", item, item.CreatedAt)
	if transactional, ok := s.repo.(ports.CatalogueTransactionalRepository); ok {
		err = transactional.CreateForAvailableIntake(ctx, item, s.now(), "application.started.v1", payload)
	} else if transactional, ok := s.repo.(ports.TransactionalRepository); ok {
		err = transactional.CreateWithEvent(ctx, item, "application.started.v1", payload)
	} else {
		err = s.repo.Create(ctx, item)
	}
	if err != nil {
		return domain.Application{}, err
	}
	if _, catalogueTransactional := s.repo.(ports.CatalogueTransactionalRepository); !catalogueTransactional {
		if _, transactional := s.repo.(ports.TransactionalRepository); transactional {
			return item, nil
		}
		if err := s.pub.Publish(ctx, "application.started.v1", a.TenantID, payload); err != nil {
			return domain.Application{}, err
		}
	}
	return item, nil
}

type CreateProgrammeInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

type UpdateProgrammeInput struct {
	Code        *string                 `json:"code"`
	Name        *string                 `json:"name"`
	Slug        *string                 `json:"slug"`
	Summary     *string                 `json:"summary"`
	Description *string                 `json:"description"`
	Status      *domain.ProgrammeStatus `json:"status"`
}

type CreateIntakeInput struct {
	Name                string    `json:"name"`
	StartsAt            time.Time `json:"starts_at"`
	ApplicationOpensAt  time.Time `json:"application_opens_at"`
	ApplicationClosesAt time.Time `json:"application_closes_at"`
	Capacity            *int      `json:"capacity"`
}

type UpdateIntakeInput struct {
	Name                                              *string
	StartsAt, ApplicationOpensAt, ApplicationClosesAt *time.Time
	CapacitySet                                       bool
	Capacity                                          *int
	Status                                            *domain.IntakeStatus
}

func (s *Service) PublicProgrammes(ctx context.Context, tenantID string, limit int) ([]domain.Programme, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, domain.ErrValidation
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, tenantID, FeatureAdmissions) {
		return nil, domain.ErrForbidden
	}
	if s.catalogue == nil {
		return nil, domain.ErrUnavailable
	}
	return s.catalogue.ListProgrammes(ctx, tenantID, true, s.now(), boundedLimit(limit, 50))
}

func (s *Service) ListProgrammes(ctx context.Context, actor auth.Actor, limit int) ([]domain.Programme, error) {
	if !s.allowed(ctx, actor, PermRead) {
		return nil, domain.ErrForbidden
	}
	if s.catalogue == nil {
		return nil, domain.ErrUnavailable
	}
	return s.catalogue.ListProgrammes(ctx, actor.TenantID, false, s.now(), boundedLimit(limit, 50))
}

func boundedLimit(limit, fallback int) int {
	if limit <= 0 || limit > 100 {
		return fallback
	}
	return limit
}

func (s *Service) CreateProgramme(ctx context.Context, actor auth.Actor, input CreateProgrammeInput) (domain.Programme, error) {
	if !s.allowed(ctx, actor, PermCatalogue) || isAI(actor) {
		return domain.Programme{}, domain.ErrForbidden
	}
	if s.catalogue == nil {
		return domain.Programme{}, domain.ErrUnavailable
	}
	programme, err := domain.NewProgramme(actor.TenantID, input.Code, input.Name, input.Slug, input.Summary, input.Description, s.now())
	if err != nil {
		return domain.Programme{}, err
	}
	payload := programmeEventPayload(programme, actor.UserID, "")
	if transactional, ok := s.repo.(ports.CatalogueEventRepository); ok {
		err = transactional.CreateProgrammeWithEvent(ctx, programme, "programme.created.v1", payload)
	} else if err = s.catalogue.CreateProgramme(ctx, programme); err == nil {
		err = s.pub.Publish(ctx, "programme.created.v1", actor.TenantID, payload)
	}
	if err != nil {
		return domain.Programme{}, err
	}
	return programme, nil
}

func (s *Service) UpdateProgramme(ctx context.Context, actor auth.Actor, id string, input UpdateProgrammeInput) (domain.Programme, error) {
	if !s.allowed(ctx, actor, PermCatalogue) || isAI(actor) {
		return domain.Programme{}, domain.ErrForbidden
	}
	if s.catalogue == nil {
		return domain.Programme{}, domain.ErrUnavailable
	}
	programme, err := s.catalogue.GetProgramme(ctx, actor.TenantID, id)
	if err != nil {
		return domain.Programme{}, err
	}
	expected, previousStatus := programme.Version, programme.Status
	err = programme.Apply(domain.ProgrammeChanges{
		Code: input.Code, Name: input.Name, Slug: input.Slug,
		Summary: input.Summary, Description: input.Description, Status: input.Status,
	}, s.now())
	if err != nil {
		return domain.Programme{}, err
	}
	payload := programmeEventPayload(programme, actor.UserID, string(previousStatus))
	if transactional, ok := s.repo.(ports.CatalogueEventRepository); ok {
		err = transactional.UpdateProgrammeWithEvent(ctx, programme, expected, "programme.updated.v1", payload)
	} else if err = s.catalogue.UpdateProgramme(ctx, programme, expected); err == nil {
		err = s.pub.Publish(ctx, "programme.updated.v1", actor.TenantID, payload)
	}
	if err != nil {
		return domain.Programme{}, err
	}
	return programme, nil
}

func (s *Service) CreateIntake(ctx context.Context, actor auth.Actor, programmeID string, input CreateIntakeInput) (domain.Intake, error) {
	if !s.allowed(ctx, actor, PermCatalogue) || isAI(actor) {
		return domain.Intake{}, domain.ErrForbidden
	}
	if s.catalogue == nil {
		return domain.Intake{}, domain.ErrUnavailable
	}
	programme, err := s.catalogue.GetProgramme(ctx, actor.TenantID, programmeID)
	if err != nil {
		return domain.Intake{}, err
	}
	if programme.Status == domain.ProgrammeArchived {
		return domain.Intake{}, domain.ErrConflict
	}
	intake, err := domain.NewIntake(
		actor.TenantID, programmeID, input.Name, input.StartsAt,
		input.ApplicationOpensAt, input.ApplicationClosesAt, input.Capacity, s.now(),
	)
	if err != nil {
		return domain.Intake{}, err
	}
	payload := intakeEventPayload(intake, actor.UserID, "")
	if transactional, ok := s.repo.(ports.CatalogueEventRepository); ok {
		err = transactional.CreateIntakeWithEvent(ctx, intake, "intake.created.v1", payload)
	} else if err = s.catalogue.CreateIntake(ctx, intake); err == nil {
		err = s.pub.Publish(ctx, "intake.created.v1", actor.TenantID, payload)
	}
	if err != nil {
		return domain.Intake{}, err
	}
	return intake, nil
}

func (s *Service) UpdateIntake(ctx context.Context, actor auth.Actor, id string, input UpdateIntakeInput) (domain.Intake, error) {
	if !s.allowed(ctx, actor, PermCatalogue) || isAI(actor) {
		return domain.Intake{}, domain.ErrForbidden
	}
	if s.catalogue == nil {
		return domain.Intake{}, domain.ErrUnavailable
	}
	intake, err := s.catalogue.GetIntake(ctx, actor.TenantID, id)
	if err != nil {
		return domain.Intake{}, err
	}
	if input.Status != nil && *input.Status == domain.IntakeOpen {
		programme, programmeErr := s.catalogue.GetProgramme(ctx, actor.TenantID, intake.ProgrammeID)
		if programmeErr != nil {
			return domain.Intake{}, programmeErr
		}
		if programme.Status != domain.ProgrammePublished {
			return domain.Intake{}, domain.ErrConflict
		}
	}
	expected, previousStatus := intake.Version, intake.Status
	err = intake.Apply(domain.IntakeChanges{
		Name: input.Name, StartsAt: input.StartsAt,
		ApplicationOpensAt:  input.ApplicationOpensAt,
		ApplicationClosesAt: input.ApplicationClosesAt,
		CapacitySet:         input.CapacitySet, Capacity: input.Capacity, Status: input.Status,
	}, s.now())
	if err != nil {
		return domain.Intake{}, err
	}
	payload := intakeEventPayload(intake, actor.UserID, string(previousStatus))
	if transactional, ok := s.repo.(ports.CatalogueEventRepository); ok {
		err = transactional.UpdateIntakeWithEvent(ctx, intake, expected, "intake.updated.v1", payload)
	} else if err = s.catalogue.UpdateIntake(ctx, intake, expected); err == nil {
		err = s.pub.Publish(ctx, "intake.updated.v1", actor.TenantID, payload)
	}
	if err != nil {
		return domain.Intake{}, err
	}
	return intake, nil
}

func programmeEventPayload(programme domain.Programme, actorUserID, previousStatus string) map[string]any {
	payload := map[string]any{
		"programme_id": programme.ID, "code": programme.Code,
		"status": programme.Status, "version": programme.Version, "changed_by": actorUserID,
		"changed_at": programme.UpdatedAt.Format(time.RFC3339),
	}
	if previousStatus != "" {
		payload["previous_status"] = previousStatus
	}
	return payload
}

func intakeEventPayload(intake domain.Intake, actorUserID, previousStatus string) map[string]any {
	payload := map[string]any{
		"intake_id": intake.ID, "programme_id": intake.ProgrammeID,
		"status": intake.Status, "version": intake.Version, "changed_by": actorUserID,
		"changed_at": intake.UpdatedAt.Format(time.RFC3339),
	}
	if previousStatus != "" {
		payload["previous_status"] = previousStatus
	}
	return payload
}
func (s *Service) Get(ctx context.Context, a auth.Actor, id string) (domain.Application, error) {
	if !a.Authenticated() || a.TenantID == "" || (s.gate != nil && !s.gate.IsEnabled(ctx, a.TenantID, FeatureAdmissions)) {
		return domain.Application{}, domain.ErrForbidden
	}
	item, err := s.repo.Get(ctx, a.TenantID, id)
	if err != nil {
		return item, err
	}
	if item.ApplicantUserID != a.UserID && !a.Has(PermRead) {
		return domain.Application{}, domain.ErrForbidden
	}
	return item, nil
}
func (s *Service) List(ctx context.Context, a auth.Actor, status domain.Status, limit int) ([]domain.Application, error) {
	if !a.Authenticated() || a.TenantID == "" || (s.gate != nil && !s.gate.IsEnabled(ctx, a.TenantID, FeatureAdmissions)) {
		return nil, domain.ErrForbidden
	}
	if status != "" && !validStatus(status) {
		return nil, domain.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	applicant := a.UserID
	if a.Has(PermRead) {
		applicant = ""
	}
	return s.repo.List(ctx, a.TenantID, applicant, status, limit)
}
func validStatus(v domain.Status) bool {
	switch v {
	case domain.StatusDraft, domain.StatusSubmitted, domain.StatusAdmitted, domain.StatusRejected, domain.StatusWithdrawn:
		return true
	}
	return false
}

type UpdateInput struct {
	LegalName, Email, Phone *string
	Answers                 map[string]any
}

func (s *Service) Update(ctx context.Context, a auth.Actor, id string, in UpdateInput) (domain.Application, error) {
	if !s.allowed(ctx, a, PermUpdate) {
		return domain.Application{}, domain.ErrForbidden
	}
	item, err := s.repo.Get(ctx, a.TenantID, id)
	if err != nil {
		return item, err
	}
	if item.ApplicantUserID != a.UserID || item.Status != domain.StatusDraft {
		return domain.Application{}, domain.ErrForbidden
	}
	expected := item.Status
	if in.LegalName != nil {
		item.LegalName = strings.TrimSpace(*in.LegalName)
	}
	if in.Email != nil {
		item.Email = strings.ToLower(strings.TrimSpace(*in.Email))
	}
	if in.Phone != nil {
		item.Phone = strings.TrimSpace(*in.Phone)
	}
	if in.Answers != nil {
		item.Answers = in.Answers
	}
	item.UpdatedAt = s.now().UTC()
	item.RefreshChecklist()
	if err = s.repo.Update(ctx, item, expected); err != nil {
		return domain.Application{}, err
	}
	return item, nil
}
func (s *Service) AttachDocument(ctx context.Context, a auth.Actor, id, fileID, kind, name string) (domain.Application, error) {
	if !s.allowed(ctx, a, PermUpdate) {
		return domain.Application{}, domain.ErrForbidden
	}
	item, err := s.repo.Get(ctx, a.TenantID, id)
	if err != nil {
		return item, err
	}
	if item.ApplicantUserID != a.UserID {
		return domain.Application{}, domain.ErrForbidden
	}
	if s.verifier == nil {
		return domain.Application{}, domain.ErrUnavailable
	}
	if err = s.verifier.Verify(ctx, a.TenantID, a.UserID, fileID); err != nil {
		return domain.Application{}, err
	}
	expected := item.Status
	if err = item.AttachDocument(fileID, kind, name, s.now()); err != nil {
		return domain.Application{}, err
	}
	if err = s.repo.Update(ctx, item, expected); err != nil {
		return domain.Application{}, err
	}
	return item, nil
}
func (s *Service) Submit(ctx context.Context, a auth.Actor, id string) (domain.Application, error) {
	if !s.allowed(ctx, a, PermSubmit) {
		return domain.Application{}, domain.ErrForbidden
	}
	item, err := s.repo.Get(ctx, a.TenantID, id)
	if err != nil {
		return item, err
	}
	if item.ApplicantUserID != a.UserID {
		return domain.Application{}, domain.ErrForbidden
	}
	expected := item.Status
	if err = item.Submit(s.now()); err != nil {
		return domain.Application{}, err
	}
	payload := ports.ApplicationEventData("application.submitted.v1", item, *item.SubmittedAt)
	if err = s.updateWithEvent(ctx, item, expected, "application.submitted.v1", payload); err != nil {
		return domain.Application{}, err
	}
	return item, nil
}
func (s *Service) Review(ctx context.Context, a auth.Actor, id, decision, note string) (domain.Application, error) {
	if !s.allowed(ctx, a, PermReview) || isAI(a) {
		return domain.Application{}, domain.ErrForbidden
	}
	item, err := s.repo.Get(ctx, a.TenantID, id)
	if err != nil {
		return item, err
	}
	expected := item.Status
	if err = item.Review(decision, a.UserID, note, s.now()); err != nil {
		return domain.Application{}, err
	}
	if item.Status == domain.StatusAdmitted {
		payload := ports.ApplicationEventData("application.admitted.v1", item, *item.ReviewedAt)
		if err = s.updateWithEvent(ctx, item, expected, "application.admitted.v1", payload); err != nil {
			return domain.Application{}, err
		}
	} else if err = s.repo.Update(ctx, item, expected); err != nil {
		return domain.Application{}, err
	}
	if item.Status == domain.StatusAdmitted {
		return item, nil
	}
	return item, nil
}
func (s *Service) IssueOffer(ctx context.Context, a auth.Actor, id, conditions string, expires time.Time) (domain.Application, error) {
	if !s.allowed(ctx, a, PermOfferIssue) || isAI(a) {
		return domain.Application{}, domain.ErrForbidden
	}
	item, err := s.repo.Get(ctx, a.TenantID, id)
	if err != nil {
		return item, err
	}
	expected := item.Status
	if err = item.IssueOffer(a.UserID, conditions, expires, s.now()); err != nil {
		return domain.Application{}, err
	}
	if err = s.updateWithEvent(ctx, item, expected, "offer.issued.v1", ports.ApplicationEventData("offer.issued.v1", item, item.UpdatedAt)); err != nil {
		return domain.Application{}, err
	}
	return item, nil
}
func (s *Service) AcceptOffer(ctx context.Context, a auth.Actor, id string) (domain.Application, error) {
	if !s.allowed(ctx, a, PermOfferAccept) {
		return domain.Application{}, domain.ErrForbidden
	}
	item, err := s.repo.Get(ctx, a.TenantID, id)
	if err != nil {
		return item, err
	}
	expected := item.Status
	if err = item.AcceptOffer(a.UserID, s.now()); err != nil {
		return domain.Application{}, err
	}
	payload := ports.ApplicationEventData("offer.accepted.v1", item, *item.OfferAcceptedAt)
	if err = s.updateWithEvent(ctx, item, expected, "offer.accepted.v1", payload); err != nil {
		return domain.Application{}, err
	}
	return item, nil
}
func (s *Service) updateWithEvent(ctx context.Context, item domain.Application, expected domain.Status, eventType string, payload map[string]any) error {
	if transactional, ok := s.repo.(ports.TransactionalRepository); ok {
		return transactional.UpdateWithEvent(ctx, item, expected, eventType, payload)
	}
	if err := s.repo.Update(ctx, item, expected); err != nil {
		return err
	}
	return s.pub.Publish(ctx, eventType, item.TenantID, payload)
}
