// Command server is the AuraEDU API Gateway — the single public entry point.
// Sprint 1 (EP-03): service registry + reverse proxy, JWT verification, tenant
// resolution, rate limiting, request-id, CORS, structured access logging, and
// feature-flag edge pre-checks.
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

	"github.com/auraedu/api-gateway/internal/gateway"
	"github.com/auraedu/api-gateway/internal/stubs"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/flags"
)

const service = "api-gateway"

// version is injected at build time via -ldflags "-X main.version=<sha>" (see Dockerfile);
// falls back to GIT_SHA env, then "dev".
var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	cfg, err := gateway.LoadConfig()
	if err != nil {
		log.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	health := gateway.NewHealth(service, version)

	proxy, err := gateway.NewReverseProxy(cfg.Registry, log)
	if err != nil {
		log.Error("failed to build reverse proxy", "err", err)
		os.Exit(1)
	}

	// Local stubs for platform/tenancy and platform/flags until those packages land.
	// In production these will be replaced by calls to the Tenant Service and a
	// flag snapshot client (agent_plan §15 EP-03).
	tenantResolver := &stubs.TenantResolver{
		BySubdomain: map[string]string{
			"upshs": "upshs",
			"aboom": "aboom",
		},
		ByHost: map[string]string{
			"upshs.auraedu.test": "upshs",
			"aboom.auraedu.test": "aboom",
		},
		TenantServiceURL: config.Getenv("SERVICE_TENANT_URL", "http://localhost:8082"),
	}

	defaults := map[string]bool{
		"student_management": true,
		"staff_management":   true,
		"attendance":         true,
		"assessments":        true,
		"billing":            true,
		"identity":           true,
	}
	tenantOverrides := map[string]map[string]bool{
		"upshs": {
			"student_management": true,
			"online_payments":    true,
			"cbt_exams":          true,
		},
		"aboom": {
			"student_management": true,
			"online_payments":    false,
			"cbt_exams":          false,
		},
	}

	fallback := flags.NewStaticSnapshot()
	for tenant := range tenantOverrides {
		for feature, enabled := range defaults {
			fallback.Set(tenant, feature, enabled)
		}
	}
	for tenant, overrides := range tenantOverrides {
		for feature, enabled := range overrides {
			fallback.Set(tenant, feature, enabled)
		}
	}
	flagClient := flags.NewTenantServiceClient(config.Getenv("SERVICE_TENANT_URL", ""), fallback)

	limiter := newRateLimiter(cfg, health, log)

	builder := &gateway.Builder{
		Log:         log,
		Config:      cfg,
		Registry:    cfg.Registry,
		Proxy:       proxy,
		Tenant:      tenantResolver,
		Flags:       flagClient,
		RateLimiter: limiter,
		Service:     service,
		Version:     version,
	}

	addr := ":" + itoa(cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           builder.Build(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("gateway listening", "addr", addr, "version", version)
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
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("failed to shutdown server", "err", err)
	}
	log.Info("gateway stopped")
}

func newRateLimiter(cfg *gateway.Config, health *gateway.HealthState, log *slog.Logger) gateway.RateLimiter {
	if cfg.RedisURL == "" {
		return nil
	}

	redisClient, err := gateway.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Error("failed to create redis client; rate limiting disabled", "err", err)
		return nil
	}

	if err := redisClient.Ping(context.Background()); err != nil {
		log.Error("redis ping failed; rate limiting disabled", "err", err)
		if closeErr := redisClient.Close(); closeErr != nil {
			log.Error("failed to close redis client", "err", closeErr)
		}
		return nil
	}

	health.AddReadinessCheck("redis", func() error {
		return redisClient.Ping(context.Background())
	})

	return &gateway.TokenBucket{
		Store:  redisClient,
		RPS:    cfg.RateLimitRPS,
		Burst:  cfg.RateLimitBurst,
		Window: cfg.RateLimitWindow,
	}
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
