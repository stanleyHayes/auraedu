package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	port := getenv("PORT", "18193")
	token := getenv("INTERNAL_SERVICE_TOKEN", "admissions-smoke-token")
	fileID := os.Getenv("SMOKE_FILE_ID")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /internal/v1/files/{file_id}/ownership", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-Tenant-ID") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if fileID == "" || r.PathValue("file_id") != fileID {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"file_id": fileID, "owner_id": "applicant-1", "status": "active", "content_type": "application/pdf",
		})
	})
	log.Fatal(http.ListenAndServe(":"+strings.TrimPrefix(port, ":"), mux))
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
