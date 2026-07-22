package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ProgrammeStatus string

const (
	ProgrammeDraft     ProgrammeStatus = "draft"
	ProgrammePublished ProgrammeStatus = "published"
	ProgrammeArchived  ProgrammeStatus = "archived"
)

type IntakeStatus string

const (
	IntakeDraft    IntakeStatus = "draft"
	IntakeOpen     IntakeStatus = "open"
	IntakeClosed   IntakeStatus = "closed"
	IntakeArchived IntakeStatus = "archived"
)

var (
	programmeCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]+$`)
	programmeSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

type Programme struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenant_id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Summary     string          `json:"summary"`
	Description string          `json:"description"`
	Status      ProgrammeStatus `json:"status"`
	Intakes     []Intake        `json:"intakes"`
	Version     int             `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type Intake struct {
	ID                  string       `json:"id"`
	TenantID            string       `json:"tenant_id"`
	ProgrammeID         string       `json:"programme_id"`
	Name                string       `json:"name"`
	StartsAt            time.Time    `json:"starts_at"`
	ApplicationOpensAt  time.Time    `json:"application_opens_at"`
	ApplicationClosesAt time.Time    `json:"application_closes_at"`
	Capacity            *int         `json:"capacity"`
	Status              IntakeStatus `json:"status"`
	Version             int          `json:"version"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

func NewProgramme(tenantID, code, name, slug, summary, description string, now time.Time) (Programme, error) {
	p := Programme{
		ID: uuid.NewString(), TenantID: strings.TrimSpace(tenantID), Code: strings.ToUpper(strings.TrimSpace(code)),
		Name: strings.TrimSpace(name), Slug: strings.ToLower(strings.TrimSpace(slug)), Summary: strings.TrimSpace(summary),
		Description: strings.TrimSpace(description), Status: ProgrammeDraft, Intakes: []Intake{}, Version: 1,
		CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	if err := p.Validate(); err != nil {
		return Programme{}, err
	}
	return p, nil
}

func (p Programme) Validate() error {
	if strings.TrimSpace(p.TenantID) == "" || uuid.Validate(p.ID) != nil ||
		len(p.Code) < 2 || len(p.Code) > 32 || !programmeCodePattern.MatchString(p.Code) ||
		len(p.Name) < 2 || len(p.Name) > 160 || len(p.Slug) < 2 || len(p.Slug) > 100 || !programmeSlugPattern.MatchString(p.Slug) ||
		len(p.Summary) < 2 || len(p.Summary) > 500 || len(p.Description) < 2 || len(p.Description) > 10000 ||
		!validProgrammeStatus(p.Status) || p.Version < 1 {
		return ErrValidation
	}
	return nil
}

func validProgrammeStatus(status ProgrammeStatus) bool {
	return status == ProgrammeDraft || status == ProgrammePublished || status == ProgrammeArchived
}

type ProgrammeChanges struct {
	Code, Name, Slug, Summary, Description *string
	Status                                 *ProgrammeStatus
}

func (p *Programme) Apply(changes ProgrammeChanges, now time.Time) error {
	if p.Status == ProgrammeArchived && changes.Status == nil {
		return ErrConflict
	}
	if changes.Code != nil {
		p.Code = strings.ToUpper(strings.TrimSpace(*changes.Code))
	}
	if changes.Name != nil {
		p.Name = strings.TrimSpace(*changes.Name)
	}
	if changes.Slug != nil {
		p.Slug = strings.ToLower(strings.TrimSpace(*changes.Slug))
	}
	if changes.Summary != nil {
		p.Summary = strings.TrimSpace(*changes.Summary)
	}
	if changes.Description != nil {
		p.Description = strings.TrimSpace(*changes.Description)
	}
	if changes.Status != nil {
		p.Status = *changes.Status
	}
	p.Version++
	p.UpdatedAt = now.UTC()
	return p.Validate()
}

func NewIntake(tenantID, programmeID, name string, startsAt, opensAt, closesAt time.Time, capacity *int, now time.Time) (Intake, error) {
	i := Intake{
		ID: uuid.NewString(), TenantID: strings.TrimSpace(tenantID), ProgrammeID: programmeID, Name: strings.TrimSpace(name),
		StartsAt: startsAt.UTC(), ApplicationOpensAt: opensAt.UTC(), ApplicationClosesAt: closesAt.UTC(), Capacity: capacity,
		Status: IntakeDraft, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	if err := i.Validate(); err != nil {
		return Intake{}, err
	}
	return i, nil
}

func (i Intake) Validate() error {
	if strings.TrimSpace(i.TenantID) == "" || uuid.Validate(i.ID) != nil || uuid.Validate(i.ProgrammeID) != nil ||
		len(i.Name) < 2 || len(i.Name) > 120 || i.StartsAt.IsZero() || i.ApplicationOpensAt.IsZero() || i.ApplicationClosesAt.IsZero() ||
		!i.ApplicationOpensAt.Before(i.ApplicationClosesAt) || i.ApplicationClosesAt.After(i.StartsAt) ||
		(i.Capacity != nil && (*i.Capacity < 1 || *i.Capacity > 1000000)) || !validIntakeStatus(i.Status) || i.Version < 1 {
		return ErrValidation
	}
	return nil
}

func validIntakeStatus(status IntakeStatus) bool {
	return status == IntakeDraft || status == IntakeOpen || status == IntakeClosed || status == IntakeArchived
}

func (i Intake) IsAvailable(now time.Time) bool {
	now = now.UTC()
	return i.Status == IntakeOpen && !now.Before(i.ApplicationOpensAt) && now.Before(i.ApplicationClosesAt)
}

type IntakeChanges struct {
	Name                                              *string
	StartsAt, ApplicationOpensAt, ApplicationClosesAt *time.Time
	CapacitySet                                       bool
	Capacity                                          *int
	Status                                            *IntakeStatus
}

func (i *Intake) Apply(changes IntakeChanges, now time.Time) error {
	if i.Status == IntakeArchived && changes.Status == nil {
		return ErrConflict
	}
	if changes.Name != nil {
		i.Name = strings.TrimSpace(*changes.Name)
	}
	if changes.StartsAt != nil {
		i.StartsAt = changes.StartsAt.UTC()
	}
	if changes.ApplicationOpensAt != nil {
		i.ApplicationOpensAt = changes.ApplicationOpensAt.UTC()
	}
	if changes.ApplicationClosesAt != nil {
		i.ApplicationClosesAt = changes.ApplicationClosesAt.UTC()
	}
	if changes.CapacitySet {
		i.Capacity = changes.Capacity
	}
	if changes.Status != nil {
		i.Status = *changes.Status
	}
	if i.Status == IntakeOpen && !now.UTC().Before(i.ApplicationClosesAt) {
		return ErrValidation
	}
	i.Version++
	i.UpdatedAt = now.UTC()
	return i.Validate()
}
