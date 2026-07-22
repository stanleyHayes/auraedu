package domain

import "testing"

func TestNewFeedbackRequiresControlledTypeAndRating(t *testing.T) {
	if _, err := NewFeedback("school-a", nil, nil, "invented", nil, nil); err == nil {
		t.Fatal("expected invalid type to fail")
	}
	rating := 6
	if _, err := NewFeedback("school-a", nil, nil, "helpful", &rating, nil); err == nil {
		t.Fatal("expected invalid rating to fail")
	}
	rating = 5
	feedback, err := NewFeedback("school-a", nil, nil, "helpful", &rating, nil)
	if err != nil || feedback.ReviewStatus != "pending" {
		t.Fatalf("feedback=%+v err=%v", feedback, err)
	}
}
