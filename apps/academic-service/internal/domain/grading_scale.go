package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GradeRange maps an inclusive percentage interval to a school-defined grade.
type GradeRange struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Grade  string  `json:"grade"`
	Remark string  `json:"remark,omitempty"`
}

// GradingScale is a tenant-owned grading policy. Assessment and reporting
// consumers reference the policy; they do not duplicate or infer its ranges.
type GradingScale struct {
	ID        string       `json:"id"`
	TenantID  string       `json:"tenant_id"`
	Name      string       `json:"name"`
	Ranges    []GradeRange `json:"ranges"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// NewGradingScale constructs a validated tenant grading policy.
func NewGradingScale(tenantID, name string, ranges []GradeRange) (*GradingScale, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if err := validateGradeRanges(name, ranges); err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("academic: generate grading scale id: %w", err)
	}
	now := time.Now().UTC()
	return &GradingScale{
		ID:        id.String(),
		TenantID:  tenantID,
		Name:      strings.TrimSpace(name),
		Ranges:    normalizeGradeRanges(ranges),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks the persisted aggregate invariants.
func (s GradingScale) Validate() error {
	if s.TenantID == "" {
		return ErrMissingTenant
	}
	return validateGradeRanges(s.Name, s.Ranges)
}

// ApplyUpdate applies the supplied policy fields and returns the changed names.
func (s *GradingScale) ApplyUpdate(name *string, ranges *[]GradeRange) ([]string, error) {
	changed := make([]string, 0, 2)
	if name != nil {
		s.Name = strings.TrimSpace(*name)
		changed = append(changed, "name")
	}
	if ranges != nil {
		s.Ranges = normalizeGradeRanges(*ranges)
		changed = append(changed, "ranges")
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func validateGradeRanges(name string, ranges []GradeRange) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if len(ranges) == 0 {
		return fmt.Errorf("%w: at least one grade range is required", ErrValidation)
	}
	normalized := normalizeGradeRanges(ranges)
	for index, item := range normalized {
		if item.Min < 0 || item.Max > 100 || item.Min > item.Max {
			return fmt.Errorf("%w: grade ranges must be within 0-100 and min must not exceed max", ErrValidation)
		}
		if strings.TrimSpace(item.Grade) == "" {
			return fmt.Errorf("%w: every grade range requires a grade", ErrValidation)
		}
		if index > 0 && item.Min <= normalized[index-1].Max {
			return fmt.Errorf("%w: grade ranges must not overlap", ErrValidation)
		}
	}
	return nil
}

func normalizeGradeRanges(ranges []GradeRange) []GradeRange {
	normalized := append([]GradeRange(nil), ranges...)
	for index := range normalized {
		normalized[index].Grade = strings.TrimSpace(normalized[index].Grade)
		normalized[index].Remark = strings.TrimSpace(normalized[index].Remark)
	}
	sort.SliceStable(normalized, func(left, right int) bool {
		if normalized[left].Min == normalized[right].Min {
			return normalized[left].Max < normalized[right].Max
		}
		return normalized[left].Min < normalized[right].Min
	})
	return normalized
}
