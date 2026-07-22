// Command transactional-email-capture is a local, in-memory stand-in for the
// notification service. It deliberately exposes only the most recent accepted
// payload so end-to-end smoke tests can retrieve one-time tokens without
// writing secrets to logs or the repository.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const maxPayloadBytes = 32 << 10

type capture struct {
	mu      sync.RWMutex
	payload json.RawMessage
}

func main() {
	token := os.Getenv("INTERNAL_SERVICE_TOKEN")
	port := os.Getenv("PORT")
	if token == "" || port == "" {
		log.Fatal("INTERNAL_SERVICE_TOKEN and PORT are required")
	}

	latest := &capture{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	mux.HandleFunc("POST /internal/v1/transactional-email", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var payload json.RawMessage
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPayloadBytes))
		if err := decoder.Decode(&payload); err != nil || !json.Valid(payload) {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		latest.mu.Lock()
		latest.payload = append(latest.payload[:0], payload...)
		latest.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /__capture/latest", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		latest.mu.RLock()
		defer latest.mu.RUnlock()
		if len(latest.payload) == 0 {
			http.Error(w, "no delivery captured", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(latest.payload)
	})

	server := &http.Server{
		Addr:              "127.0.0.1:" + port,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       15 * time.Second,
	}
	log.Printf("transactional email capture listening on 127.0.0.1:%s", port)
	log.Fatal(server.ListenAndServe())
}

func authorized(r *http.Request, token string) bool {
	expected := []byte("Bearer " + token)
	provided := []byte(r.Header.Get("Authorization"))
	return len(expected) == len(provided) && subtle.ConstantTimeCompare(expected, provided) == 1
}
