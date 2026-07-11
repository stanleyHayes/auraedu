// Package workercmd provides the tenant-service worker command.
package workercmd

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

// Run starts the tenant-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
