// Package workercmd provides the attendance-service worker command.
package workercmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Info("attendance-service worker started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("attendance-service worker stopped")
}

// Run starts the attendance-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
