package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/adapters/memory"
	"github.com/auraedu/ai-orchestrator-service/internal/application"
	"github.com/auraedu/ai-orchestrator-service/internal/domain"
)

type knowledgeStub struct {
	results []domain.KnowledgeResult
	err     error
	calls   int
}

func (k *knowledgeStub) Search(_ context.Context, _, _, _ string, _ int, _ time.Time) ([]domain.KnowledgeResult, error) {
	k.calls++
	return k.results, k.err
}

func TestAssistantUsesRequestedLanguageAndRejectsUnsupportedLocales(t *testing.T) {
	knowledge := &knowledgeStub{}
	svc := application.NewService(memory.New(), knowledge)
	response, err := svc.Ask(context.Background(), application.AskInput{
		TenantID: "school-one", IdempotencyKey: "assistant-french-locale-0001",
		Question: "Quels programmes sont ouverts ?", Locale: "fr-GH",
	})
	if err != nil {
		t.Fatalf("ask in French: %v", err)
	}
	if response.Locale != "fr-GH" || !response.NeedsHuman || response.Answer != assistantFrenchUnsupported {
		t.Fatalf("French response=%+v", response)
	}
	if _, err := svc.Ask(context.Background(), application.AskInput{
		TenantID: "school-one", IdempotencyKey: "assistant-invalid-locale-01",
		Question: "Welche Programme sind offen?", Locale: "de",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unsupported locale should fail validation: %v", err)
	}
}

const assistantFrenchUnsupported = "Je n'ai pas trouvé suffisamment d'informations approuvées pour répondre de manière fiable. Veuillez contacter le service des admissions afin qu'il puisse vous confirmer la réponse."

func TestAssistantAnswersOnlyFromApprovedRetrievalAndCitesSource(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	knowledge := &knowledgeStub{results: []domain.KnowledgeResult{{SourceID: "e3f848b8-849d-4b69-a46e-b37a436759ac",
		Title: "2026 Fee Schedule", Passage: "The approved application fee is GHS 250.", Locale: "en", Version: 2, Score: 0.72}}}
	svc := application.NewService(memory.New(), knowledge, application.WithClock(func() time.Time { return now }))
	input := application.AskInput{TenantID: "school-one", IdempotencyKey: "assistant-idempotency-0001", Question: "What is the application fee?"}
	response, err := svc.Ask(context.Background(), input)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if response.Answer != knowledge.results[0].Passage || response.NeedsHuman || len(response.Citations) != 1 || response.Citations[0].SourceID != knowledge.results[0].SourceID {
		t.Fatalf("ungrounded response: %+v", response)
	}
	replay, err := svc.Ask(context.Background(), input)
	if err != nil || replay.MessageID != response.MessageID || knowledge.calls != 1 {
		t.Fatalf("idempotent replay: response=%+v calls=%d err=%v", replay, knowledge.calls, err)
	}
	changed := input
	changed.Question = "What is the tuition?"
	if _, err := svc.Ask(context.Background(), changed); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed replay should conflict: %v", err)
	}
}

func TestAssistantRejectsMismatchedKnowledgeLanguage(t *testing.T) {
	knowledge := &knowledgeStub{results: []domain.KnowledgeResult{{
		SourceID: "e3f848b8-849d-4b69-a46e-b37a436759ac", Title: "Guide français",
		Passage: "Les candidatures sont ouvertes.", Locale: "fr-GH", Version: 1, Score: 0.8,
	}}}
	svc := application.NewService(memory.New(), knowledge)
	response, err := svc.Ask(context.Background(), application.AskInput{
		TenantID: "school-one", IdempotencyKey: "assistant-language-guard-001",
		Question: "Which applications are open?", Locale: "en-GH",
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if !response.NeedsHuman || len(response.Citations) != 0 || response.Answer != "I could not find enough approved information to answer that safely. Please contact the admissions team so they can confirm it for you." {
		t.Fatalf("mismatched language leaked: %+v", response)
	}
}

func TestAssistantRefusesUnsupportedQuestionAndOffersHuman(t *testing.T) {
	knowledge := &knowledgeStub{}
	svc := application.NewService(memory.New(), knowledge)
	response, err := svc.Ask(context.Background(), application.AskInput{TenantID: "school-one", IdempotencyKey: "assistant-idempotency-0002", Question: "Do you guarantee admission?"})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if !response.NeedsHuman || response.Confidence != 0 || len(response.Citations) != 0 || response.EscalationMessage == nil {
		t.Fatalf("unsupported answer did not fail closed: %+v", response)
	}
}

func TestAssistantFailsWhenKnowledgeDependencyFails(t *testing.T) {
	svc := application.NewService(memory.New(), &knowledgeStub{err: errors.New("timeout")})
	_, err := svc.Ask(context.Background(), application.AskInput{TenantID: "school-one", IdempotencyKey: "assistant-idempotency-0003", Question: "What programmes are offered?"})
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("dependency error = %v", err)
	}
}
