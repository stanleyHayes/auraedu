package unit

import (
	"testing"

	"github.com/auraedu/cbt-service/internal/domain"
)

func TestNewQuestionBank_RequiresTenant(t *testing.T) {
	if _, err := domain.NewQuestionBank("", ay1, subject1, "What is 2+2?", "multiple_choice", "4", 1, []string{"3", "4", "5"}); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewQuestionBank_RequiresAcademicYearID(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, "", subject1, "What is 2+2?", "multiple_choice", "4", 1, []string{"3", "4", "5"}); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewQuestionBank_RequiresSubjectID(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, "", "What is 2+2?", "multiple_choice", "4", 1, []string{"3", "4", "5"}); err == nil {
		t.Fatal("expected error when subject_id is empty")
	}
}

func TestNewQuestionBank_RequiresQuestionText(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, subject1, "", "multiple_choice", "4", 1, []string{"3", "4", "5"}); err == nil {
		t.Fatal("expected error when question_text is empty")
	}
}

func TestNewQuestionBank_RequiresValidType(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, subject1, "What?", "essay", "x", 1, nil); err == nil {
		t.Fatal("expected error for invalid question_type")
	}
}

func TestNewQuestionBank_RequiresCorrectAnswer(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, subject1, "What is 2+2?", "multiple_choice", "", 1, []string{"3", "4", "5"}); err == nil {
		t.Fatal("expected error when correct_answer is empty")
	}
}

func TestNewQuestionBank_RequiresPositiveMarks(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, subject1, "What is 2+2?", "multiple_choice", "4", 0, []string{"3", "4", "5"}); err == nil {
		t.Fatal("expected error when marks is zero")
	}
}

func TestNewQuestionBank_MultipleChoiceRequiresOptions(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, subject1, "What is 2+2?", "multiple_choice", "4", 1, []string{"4"}); err == nil {
		t.Fatal("expected error for multiple_choice with <2 options")
	}
}

func TestNewQuestionBank_TrueFalseRequiresTwoOptions(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, subject1, "Is water wet?", "true_false", "true", 1, []string{"true"}); err == nil {
		t.Fatal("expected error for true_false with !=2 options")
	}
}

func TestNewQuestionBank_ShortAnswerRejectsOptions(t *testing.T) {
	if _, err := domain.NewQuestionBank(tenantA, ay1, subject1, "Explain photosynthesis.", "short_answer", "plants use light", 1, []string{"a"}); err == nil {
		t.Fatal("expected error for short_answer with options")
	}
}

func TestNewQuestionBank_Valid(t *testing.T) {
	q, err := domain.NewQuestionBank(tenantA, ay1, subject1, "What is 2+2?", "multiple_choice", "4", 2, []string{"3", "4", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Status != string(domain.QuestionStatusDraft) {
		t.Fatalf("expected draft status, got %q", q.Status)
	}
	if q.Marks != 2 {
		t.Fatalf("expected marks 2, got %d", q.Marks)
	}
}

func TestQuestionBank_ApplyUpdate(t *testing.T) {
	q, err := domain.NewQuestionBank(tenantA, ay1, subject1, "What is 2+2?", "multiple_choice", "4", 1, []string{"3", "4", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := "What is 3+3?"
	marks := 3
	status := string(domain.QuestionStatusPublished)
	changed, err := q.ApplyUpdate(&text, nil, nil, &marks, nil, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if q.QuestionText != text || q.Marks != marks || q.Status != status {
		t.Fatalf("question not updated: %+v", q)
	}
}

func TestQuestionBank_ApplyUpdate_InvalidTransition(t *testing.T) {
	q, err := domain.NewQuestionBank(tenantA, ay1, subject1, "What?", "multiple_choice", "a", 1, []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q.Status = string(domain.QuestionStatusArchived)
	status := string(domain.QuestionStatusPublished)
	if _, err := q.ApplyUpdate(nil, nil, nil, nil, nil, &status); err == nil {
		t.Fatal("expected error for invalid transition archived -> published")
	}
}
