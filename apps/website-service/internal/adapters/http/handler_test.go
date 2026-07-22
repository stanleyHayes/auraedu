package http

import (
	"context"
	"net/http"
	"testing"
)

func TestRegisterUsesNonOverlappingPageRoutes(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(nil).Register(mux)

	cases := map[string]string{
		"GET /api/v1/website/page-slugs/home":                                     "GET /api/v1/website/page-slugs/{slug}",
		"GET /api/v1/website/pages/11111111-1111-1111-1111-111111111111/sections": "GET /api/v1/website/pages/{page_id}/sections",
	}
	for target, expected := range cases {
		request, err := http.NewRequestWithContext(context.Background(), target[:3], target[4:], nil)
		if err != nil {
			t.Fatal(err)
		}
		_, pattern := mux.Handler(request)
		if pattern != expected {
			t.Fatalf("expected %s to resolve as %q, got %q", target, expected, pattern)
		}
	}
}
