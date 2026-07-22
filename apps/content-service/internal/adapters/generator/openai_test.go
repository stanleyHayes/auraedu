package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
)

func TestOpenAIGeneratorUsesResponsesAPIWithoutStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		instructions, ok := body["instructions"].(string)
		if !ok || body["store"] != false || body["model"] != "model-a" || strings.Contains(instructions, "guaranteed admission") {
			t.Fatalf("unsafe or malformed provider request: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"Visit us on 30 August. Terms apply"}]}]}`))
	}))
	defer server.Close()
	generator, err := NewOpenAI(server.URL, "secret", "model-a", &http.Client{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	result, err := generator.Generate(context.Background(), ports.GenerateInput{ContentType: "social_post", Title: "Open day", Brief: "Invite prospective families to the open day.", Audience: "Prospective families", Locale: "en-GH", KeyMessages: []string{"Visit us"}, Facts: []domain.Fact{{Label: "Date", Value: "30 August"}}, Profile: domain.BrandProfile{ToneOfVoice: "Warm", RequiredDisclaimers: []string{"Terms apply"}, ProhibitedClaims: []string{"guaranteed admission"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Visit us on 30 August. Terms apply" || result.Generator != "openai:model-a" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestOpenAIGeneratorDoesNotLeakProviderErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"sensitive prompt echo"}}`))
	}))
	defer server.Close()
	generator, setupErr := NewOpenAI(server.URL, "secret", "model-a", server.Client())
	if setupErr != nil {
		t.Fatal(setupErr)
	}
	_, err := generator.Generate(context.Background(), ports.GenerateInput{})
	if err == nil || strings.Contains(err.Error(), "sensitive prompt echo") {
		t.Fatalf("provider body must not escape adapter: %v", err)
	}
}
