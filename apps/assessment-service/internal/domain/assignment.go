package domain

import (
	"fmt"
	"strings"
	"time"
)

// An Assignment is an Assessment with type "assignment" plus class targeting
// (class_ids) and a publish lifecycle (published_at). The OpenAPI contract
// models it as its own resource; the service stores it in the assessments
// table so recorded scores flow into the gradebook.
type Assignment = Assessment

// NewAssignment constructs an Assessment of type assignment, enforcing
// invariants. instructions maps to the assessment description.
func NewAssignment(tenantID, academicYearID, subjectID, title, instructions string, maxScore int, dueDate *time.Time, classIDs []string) (*Assignment, error) {
	a, err := NewAssessment(tenantID, academicYearID, subjectID, string(TypeAssignment), title, instructions, maxScore, dueDate)
	if err != nil {
		return nil, err
	}
	ids, err := normalizeClassIDs(classIDs)
	if err != nil {
		return nil, err
	}
	a.ClassIDs = ids
	return a, nil
}

// IsAssignment reports whether the assessment is an assignment.
func (a *Assessment) IsAssignment() bool { return a.Type == string(TypeAssignment) }

// ApplyAssignmentUpdate mutates the assignment with non-nil patch fields and
// returns the names of fields that changed. instructions maps to description;
// classIDs replaces the full class list when non-nil.
func (a *Assessment) ApplyAssignmentUpdate(title, instructions *string, maxScore *int, dueDate *time.Time, classIDs []string) ([]string, error) {
	var changed []string

	if title != nil {
		if strings.TrimSpace(*title) == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrValidation)
		}
		a.Title = strings.TrimSpace(*title)
		changed = append(changed, "title")
	}
	if instructions != nil {
		if strings.TrimSpace(*instructions) == "" {
			a.Description = nil
		} else {
			d := strings.TrimSpace(*instructions)
			a.Description = &d
		}
		changed = append(changed, "instructions")
	}
	if maxScore != nil {
		if *maxScore <= 0 {
			return nil, fmt.Errorf("%w: max_score must be greater than 0", ErrValidation)
		}
		a.MaxScore = *maxScore
		changed = append(changed, "max_score")
	}
	if dueDate != nil {
		a.DueDate = dueDate
		changed = append(changed, "due_date")
	}
	if classIDs != nil {
		ids, err := normalizeClassIDs(classIDs)
		if err != nil {
			return nil, err
		}
		a.ClassIDs = ids
		changed = append(changed, "class_ids")
	}

	if len(changed) > 0 {
		a.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// Publish transitions a draft assignment to published and stamps published_at.
// Publishing an already-published or archived assignment is a validation error.
func (a *Assessment) Publish() error {
	if !a.IsAssignment() {
		return fmt.Errorf("%w: only assignments can be published via the assignments API", ErrValidation)
	}
	if a.Status == string(StatusPublished) {
		return fmt.Errorf("%w: assignment is already published", ErrValidation)
	}
	if a.Status != string(StatusDraft) {
		return fmt.Errorf("%w: cannot publish assignment from status %s", ErrValidation, a.Status)
	}
	now := time.Now().UTC()
	a.Status = string(StatusPublished)
	a.PublishedAt = &now
	a.UpdatedAt = now
	return nil
}

// normalizeClassIDs trims, drops empties and de-duplicates class IDs,
// preserving input order.
func normalizeClassIDs(classIDs []string) ([]string, error) {
	seen := make(map[string]struct{}, len(classIDs))
	out := make([]string, 0, len(classIDs))
	for _, id := range classIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("%w: class_ids cannot contain empty values", ErrValidation)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
