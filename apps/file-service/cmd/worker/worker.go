// Package workercmd provides the file-service worker command.
package workercmd

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
	log.Info("file-service worker started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("file-service worker stopped")
}

// Run starts the file-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
