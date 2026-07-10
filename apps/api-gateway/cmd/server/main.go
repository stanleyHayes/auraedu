// Command server is the AuraEDU API Gateway — the single public entry point.
// Sprint 0: liveness/readiness only. Auth verification, tenant resolution, rate
// limiting and routing land in Sprint 1 (EP-03: AURA-3.1..3.5).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
)

const service = "api-gateway"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mux := http.NewServeMux()
	health := httpx.NewHealth(service, config.Getenv("GIT_SHA", "dev"))
	health.Register(mux)

	addr := ":" + itoa(config.Port(8080))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("gateway listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("gateway stopped")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
