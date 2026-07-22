// Package application coordinates assistant use cases and durable persistence.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/ai-orchestrator-service/internal/ports"
	"github.com/auraedu/platform/flags"
	"github.com/google/uuid"
)

const FeatureWebsiteChat = "growth_website_chat"

var localePattern = regexp.MustCompile(`^(en|fr)(-GH)?$`)

type localizedMessages struct {
	unsupported   string
	lowConfidence string
}

func messagesFor(language string) localizedMessages {
	if language == "fr" {
		return localizedMessages{
			unsupported: "Je n'ai pas trouvé suffisamment d'informations approuvées pour répondre de manière fiable. " +
				"Veuillez contacter le service des admissions afin qu'il puisse vous confirmer la réponse.",
			lowConfidence: "Cette source approuvée peut être pertinente, " +
				"mais un responsable des admissions doit la confirmer.",
		}
	}
	return localizedMessages{
		unsupported: "I could not find enough approved information to answer that safely. " +
			"Please contact the admissions team so they can confirm it for you.",
		lowConfidence: "This approved source may be relevant, but an admissions officer should confirm it.",
	}
}

type Service struct {
	repo      ports.Repository
	knowledge ports.KnowledgeRetriever
	pub       ports.EventPublisher
	gate      flags.Gate
	now       func() time.Time
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, string, string, map[string]any) error { return nil }

type Option func(*Service)

func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }
func WithFeatureGate(gate flags.Gate) Option        { return func(s *Service) { s.gate = gate } }
func WithClock(now func() time.Time) Option         { return func(s *Service) { s.now = now } }

func NewService(repo ports.Repository, knowledge ports.KnowledgeRetriever, options ...Option) *Service {
	s := &Service{repo: repo, knowledge: knowledge, pub: noopPublisher{}, now: time.Now}
	for _, option := range options {
		option(s)
	}
	return s
}

type AskInput struct {
	TenantID, IdempotencyKey, SessionID, Question, Locale string
}

func (s *Service) Ask(ctx context.Context, input AskInput) (domain.Response, error) {
	input, err := normalizeAskInput(input)
	if err != nil {
		return domain.Response{}, err
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, input.TenantID, FeatureWebsiteChat) {
		return domain.Response{}, domain.ErrForbidden
	}
	keyHash := digest(input.IdempotencyKey)
	requestBytes, err := json.Marshal(map[string]string{
		"session_id": input.SessionID,
		"question":   input.Question,
		"locale":     input.Locale,
	})
	if err != nil {
		return domain.Response{}, err
	}
	requestHash := digest(string(requestBytes))
	if replay, storedHash, found, err := s.repo.FindReplay(ctx, input.TenantID, keyHash, requestHash); err != nil {
		return domain.Response{}, err
	} else if found {
		if storedHash != requestHash {
			return domain.Response{}, domain.ErrConflict
		}
		return replay, nil
	}
	if input.SessionID == "" {
		input.SessionID = uuid.NewString()
	}

	now := s.now().UTC()
	results, err := s.knowledge.Search(ctx, input.TenantID, input.Question, input.Locale, 5, now)
	if err != nil {
		return domain.Response{}, domain.ErrUnavailable
	}
	requestedLanguage := strings.SplitN(input.Locale, "-", 2)[0]
	results = matchingLanguageResults(results, requestedLanguage)
	response := domain.Response{
		TenantID:  input.TenantID,
		SessionID: input.SessionID,
		MessageID: uuid.NewString(),
		Question:  input.Question,
		Locale:    input.Locale,
		Citations: []domain.Citation{},
		CreatedAt: now,
	}
	messages := messagesFor(requestedLanguage)
	if len(results) == 0 {
		message := messages.unsupported
		response.Answer, response.NeedsHuman, response.EscalationMessage = message, true, &message
	} else {
		result := results[0]
		response.Answer = result.Passage
		response.Confidence = clamp(result.Score)
		response.Citations = append(response.Citations, domain.Citation{
			SourceID: result.SourceID,
			Title:    result.Title,
			Version:  result.Version,
		})
		// Low lexical confidence still exposes the approved passage, but clearly
		// asks a human to verify instead of manufacturing a synthesis.
		if response.Confidence < 0.08 {
			message := messages.lowConfidence
			response.NeedsHuman, response.EscalationMessage = true, &message
		}
	}
	durableEscalation, saveErr := s.saveResponse(ctx, response, keyHash, requestHash)
	if saveErr != nil {
		if errors.Is(saveErr, domain.ErrConflict) {
			replay, storedHash, found, replayErr := s.repo.FindReplay(
				ctx, input.TenantID, keyHash, requestHash,
			)
			if replayErr == nil && found && storedHash == requestHash {
				return replay, nil
			}
		}
		return domain.Response{}, saveErr
	}
	if response.NeedsHuman && !durableEscalation {
		if err := s.pub.Publish(ctx, "assistant.question_unanswered.v1", input.TenantID, ports.EscalationEventData(response)); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish unanswered assistant question", "err", err)
		}
	}
	return response, nil
}

func normalizeAskInput(input AskInput) (AskInput, error) {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.Question = strings.TrimSpace(input.Question)
	input.Locale = strings.TrimSpace(input.Locale)
	if input.Locale == "" {
		input.Locale = "en"
	}
	valid := input.TenantID != "" && len(input.IdempotencyKey) >= 16 && len(input.IdempotencyKey) <= 128 &&
		len(input.Question) >= 2 && len(input.Question) <= 500 && localePattern.MatchString(input.Locale)
	if !valid {
		return AskInput{}, domain.ErrValidation
	}
	if input.SessionID != "" {
		if _, err := uuid.Parse(input.SessionID); err != nil {
			return AskInput{}, domain.ErrValidation
		}
	}
	return input, nil
}

func matchingLanguageResults(results []domain.KnowledgeResult, language string) []domain.KnowledgeResult {
	matching := results[:0]
	for _, result := range results {
		if strings.SplitN(result.Locale, "-", 2)[0] == language {
			matching = append(matching, result)
		}
	}
	return matching
}

func (s *Service) saveResponse(
	ctx context.Context,
	response domain.Response,
	keyHash string,
	requestHash string,
) (bool, error) {
	if response.NeedsHuman {
		if transactional, ok := s.repo.(ports.TransactionalExchangeRepository); ok {
			return true, transactional.SaveWithEvent(
				ctx,
				response,
				keyHash,
				requestHash,
				"assistant.question_unanswered.v1",
				ports.EscalationEventData(response),
			)
		}
	}
	return false, s.repo.Save(ctx, response, keyHash, requestHash)
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
