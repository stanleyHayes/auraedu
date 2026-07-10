// Command worker is the Tenant Service background consumer.
package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// TODO(AURA-5.x): consume billing.subscription_changed to sync plan-gated flags;
// publish tenant.created / tenant.feature_enabled|disabled via platform/eventbus.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Info("tenant-service worker started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("tenant-service worker stopped")
}
