// Command worker is the Identity Service background consumer.
package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// TODO(AURA-4.x): consume events to sync roles/permissions; emit user.role_changed.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Info("identity-service worker started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("identity-service worker stopped")
}
