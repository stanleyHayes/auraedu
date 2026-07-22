// Package servercmd provides the api-gateway server command.
// Sprint 1 (EP-03): service registry + reverse proxy, JWT verification, tenant
// resolution, rate limiting, request-id, CORS, structured access logging, and
// feature-flag edge pre-checks.
package servercmd

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
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
)

const service = "api-gateway"

// version is injected at build time via -ldflags "-X main.version=<sha>" (see Dockerfile);
// falls back to GIT_SHA env, then "dev".
var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)

	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}
	shutdownTracing, err := observ.InitTracing(service, version)
	if err != nil {
		return err
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			log.Error("failed to flush tracing", "err", err)
		}
	}()

	cfg, err := gateway.LoadConfig()
	if err != nil {
		return err
	}

	health := gateway.NewHealth(service, version)

	proxy, err := gateway.NewReverseProxy(cfg.Registry, log)
	if err != nil {
		return err
	}

	// Development keeps two deterministic school fixtures. Production resolves
	// every tenant through Tenant Service and rejects unverified headers.
	developmentTenantHosts := map[string]string(nil)
	developmentTenantSubdomains := map[string]string(nil)
	if cfg.Environment != "production" {
		developmentTenantSubdomains = map[string]string{"upshs": "upshs", "aboom": "aboom"}
		developmentTenantHosts = map[string]string{"upshs.auraedu.test": "upshs", "aboom.auraedu.test": "aboom"}
	}
	subdomainBaseDomain := "auraedu.com"
	if cfg.Environment != "production" {
		subdomainBaseDomain = "auraedu.test"
	}
	tenantResolver := &stubs.TenantResolver{
		BySubdomain:           developmentTenantSubdomains,
		ByHost:                developmentTenantHosts,
		TenantServiceURL:      config.ServiceURL(config.Getenv("SERVICE_TENANT_URL", "http://localhost:8082")),
		SubdomainBaseDomain:   subdomainBaseDomain,
		Client:                &http.Client{Timeout: 4 * time.Second},
		AllowUnverifiedHeader: cfg.Environment != "production",
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
	if cfg.Environment != "production" {
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
	}
	flagClient := flags.NewTenantServiceClient(config.Getenv("SERVICE_TENANT_URL", ""), fallback)

	limiter, redisClient, err := newRateLimiter(cfg, health)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			log.Error("failed to close redis client", "err", closeErr)
		}
	}()

	builder := &gateway.Builder{
		Log:         log,
		Config:      cfg,
		Registry:    cfg.Registry,
		Proxy:       proxy,
		Tenant:      tenantResolver,
		Flags:       flagClient,
		RateLimiter: limiter,
		Health:      health,
		Dependencies: gateway.NewDependencyHealthHandler(
			gateway.DefaultDependencies(),
			nil,
			3*time.Second,
		),
		Service: service,
		Version: version,
	}

	addr := ":" + itoa(cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           observ.HTTPHandler(service, httpx.RequestBoundaryMiddleware(builder.Build())),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("gateway listening", "addr", addr, "version", version)
		errCh <- srv.ListenAndServe()
	}()

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-stop:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	log.Info("gateway stopped")
	return nil
}

func newRateLimiter(cfg *gateway.Config, health *gateway.HealthState) (gateway.RateLimiter, *gateway.RedisClient, error) {
	if cfg.RedisURL == "" {
		return nil, nil, errors.New("REDIS_URL is required")
	}

	redisClient, err := gateway.NewRedisClient(cfg.RedisURL)
	if err != nil {
		return nil, nil, err
	}

	health.AddReadinessCheck("redis", func() error {
		return pingRedis(redisClient, 2*time.Second)
	})

	if err := pingRedis(redisClient, 3*time.Second); err != nil {
		if closeErr := redisClient.Close(); closeErr != nil {
			return nil, nil, errors.Join(err, closeErr)
		}
		return nil, nil, err
	}

	return &gateway.TokenBucket{
		Store:  redisClient,
		RPS:    cfg.RateLimitRPS,
		Burst:  cfg.RateLimitBurst,
		Window: cfg.RateLimitWindow,
	}, redisClient, nil
}

func pingRedis(client *gateway.RedisClient, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return client.Ping(ctx)
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

// Run starts the api-gateway HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	return run()
}
