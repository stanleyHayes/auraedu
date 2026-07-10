// Command worker is the academic-service background event consumer.
package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// TODO(AURA): subscribe to domain events via platform/eventbus; skip disabled-feature
// tenants; update projections idempotently.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Info("academic-service worker started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("academic-service worker stopped")
}
