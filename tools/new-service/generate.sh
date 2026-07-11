#!/usr/bin/env bash
# AuraEDU service generator (AURA-0.2). Scaffolds an identical hexagonal Go service
# (agent_plan §5) so every domain service is structurally the same.
#
#   make new-service NAME=student        (creates apps/student-service)
#
# Emits: cmd/{server,worker}, internal/{domain,application,ports,adapters}, migrations,
# tests, Dockerfile, README, go.mod; wires go.work; prints the render.yaml block to add.
set -euo pipefail

NAME="${1:-}"
if [ -z "$NAME" ]; then echo "usage: $0 <name>   (e.g. student)"; exit 1; fi
if ! printf '%s' "$NAME" | grep -Eq '^[a-z][a-z0-9-]*$'; then
  echo "error: name must be lowercase letters/digits/hyphens, e.g. 'student' or 'ai-tutor'"; exit 1
fi

# Resolve repo root (two levels up from tools/new-service).
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

SVCNAME="$NAME"                       # student
SVCDIR="${NAME}-service"              # student-service
SVCMODULE="github.com/auraedu/${SVCDIR}"
DIR="apps/${SVCDIR}"

# PascalCase the (possibly hyphenated) name → Go type identifier. Portable (no bash4).
pascal() {
  out=""; oldIFS="$IFS"; IFS='-'
  for w in $1; do
    f="$(printf '%s' "$w" | cut -c1 | tr '[:lower:]' '[:upper:]')"
    r="$(printf '%s' "$w" | cut -c2-)"
    out="${out}${f}${r}"
  done
  IFS="$oldIFS"; printf '%s' "$out"
}
SVCTITLE="$(pascal "$NAME")"          # Student

# Refuse to clobber an existing service (a lone README.md placeholder is fine to replace).
if [ -d "$DIR" ] && [ -n "$(find "$DIR" -type f ! -name 'README.md' 2>/dev/null)" ]; then
  echo "error: $DIR already exists with source files — aborting."; exit 1
fi

echo "==> scaffolding $DIR (module $SVCMODULE, type $SVCTITLE)"
mkdir -p "$DIR"/cmd/server "$DIR"/cmd/worker \
  "$DIR"/internal/domain "$DIR"/internal/application "$DIR"/internal/ports \
  "$DIR"/internal/adapters/postgres "$DIR"/internal/adapters/http "$DIR"/internal/adapters/events \
  "$DIR"/migrations "$DIR"/tests/unit "$DIR"/tests/integration "$DIR"/tests/contract "$DIR"/tests/tenant_isolation

# --- templates (quoted heredocs → backticks/$ are literal; %%TOKENS%% substituted below) ---

cat > "$DIR/go.mod" <<'EOF'
module %%MODULE%%

go 1.26.5

toolchain go1.26.5

require github.com/auraedu/platform v0.0.0

replace github.com/auraedu/platform => ../../platform
EOF

cat > "$DIR/internal/domain/%%NAME%%.go.tmpl" <<'EOF'
package domain

import "time"

// %%TITLE%% is the aggregate root of the %%NAME%% service. Every record is tenant-scoped.
type %%TITLE%% struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// New%%TITLE%% constructs a %%TITLE%%, enforcing invariants (tenant + name required).
func New%%TITLE%%(id, tenantID, name string) (*%%TITLE%%, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if name == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	return &%%TITLE%%{ID: id, TenantID: tenantID, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
EOF
mv "$DIR/internal/domain/%%NAME%%.go.tmpl" "$DIR/internal/domain/${SVCNAME}.go"

cat > "$DIR/internal/domain/errors.go" <<'EOF'
package domain

import "errors"

var (
	ErrNotFound      = errors.New("%%NAME%%: not found")
	ErrValidation    = errors.New("%%NAME%%: validation failed")
	ErrMissingTenant = errors.New("%%NAME%%: tenant context required")
)
EOF

cat > "$DIR/internal/ports/repository.go" <<'EOF'
package ports

import (
	"context"

	"%%MODULE%%/internal/domain"
)

// Repository persists %%TITLE%% aggregates. Implementations MUST scope every query by
// tenantID (defense-in-depth with Postgres RLS, agent_plan §7).
type Repository interface {
	Create(ctx context.Context, tenantID string, e *domain.%%TITLE%%) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.%%TITLE%%, error)
	List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.%%TITLE%%, string, error)
}
EOF

cat > "$DIR/internal/application/service.go" <<'EOF'
package application

import (
	"context"

	"%%MODULE%%/internal/domain"
	"%%MODULE%%/internal/ports"
)

// Service holds the %%NAME%% use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
type Service struct {
	repo ports.Repository
}

func NewService(repo ports.Repository) *Service { return &Service{repo: repo} }

// Create validates and persists a new %%TITLE%% for the given tenant.
func (s *Service) Create(ctx context.Context, tenantID, id, name string) (*domain.%%TITLE%%, error) {
	e, err := domain.New%%TITLE%%(id, tenantID, name)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, e); err != nil {
		return nil, err
	}
	return e, nil
}
EOF

cat > "$DIR/internal/adapters/postgres/repository.go" <<'EOF'
package postgres

import (
	"context"

	"%%MODULE%%/internal/domain"
	"%%MODULE%%/internal/ports"
)

// Repository is the Postgres implementation of ports.Repository.
// TODO(AURA): wire the pgx pool from platform/db; every query must SET app.tenant_id
// (RLS) and filter by tenant_id.
type Repository struct{}

var _ ports.Repository = (*Repository)(nil)

func NewRepository() *Repository { return &Repository{} }

func (r *Repository) Create(ctx context.Context, tenantID string, e *domain.%%TITLE%%) error {
	_ = ctx
	_ = tenantID
	_ = e
	return nil // TODO: INSERT
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.%%TITLE%%, error) {
	_ = ctx
	_ = tenantID
	_ = id
	return nil, domain.ErrNotFound // TODO: SELECT
}

func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.%%TITLE%%, string, error) {
	_ = ctx
	_ = tenantID
	_ = limit
	_ = cursor
	return nil, "", nil // TODO: SELECT ... cursor pagination
}
EOF

cat > "$DIR/internal/adapters/http/handler.go" <<'EOF'
package http

import (
	"net/http"

	"%%MODULE%%/internal/application"
)

// Handler adapts HTTP to the %%NAME%% use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
// TODO(AURA): implement per contracts/openapi/%%NAME%%.v1.yaml; enforce
// authenticated → tenant → RBAC → feature-flag → ownership before each action.
func (h *Handler) Register(mux *http.ServeMux) {
	_ = h
	_ = mux
	// mux.HandleFunc("GET /api/v1/%%NAME%%s", h.list)
}
EOF

cat > "$DIR/internal/adapters/events/publisher.go" <<'EOF'
package events

// TODO(AURA): publish/consume domain events via platform/eventbus (NATS JetStream,
// CloudEvents). Every event MUST carry tenant_id; workers skip disabled-feature tenants.
EOF

cat > "$DIR/cmd/server/main.go" <<'EOF'
// Command server is the %%DIR%% HTTP entrypoint. Sprint scaffold: health + wiring.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	svchttp "%%MODULE%%/internal/adapters/http"
	"%%MODULE%%/internal/adapters/postgres"
	"%%MODULE%%/internal/application"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
)

const service = "%%DIR%%"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	repo := postgres.NewRepository()
	svc := application.NewService(repo)
	handler := svchttp.NewHandler(svc)

	mux := http.NewServeMux()
	httpx.NewHealth(service, version).WithLogger(log).Register(mux)
	handler.Register(mux)

	addr := ":" + strconv.Itoa(config.Port(8080))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Info(service+" listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info(service + " stopped")
}
EOF

cat > "$DIR/cmd/worker/main.go" <<'EOF'
// Command worker is the %%DIR%% background event consumer.
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
	log.Info("%%DIR%% worker started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("%%DIR%% worker stopped")
}
EOF

cat > "$DIR/tests/unit/%%NAME%%_test.go.tmpl" <<'EOF'
package unit

import (
	"testing"

	"%%MODULE%%/internal/domain"
)

func TestNew%%TITLE%%_RequiresTenant(t *testing.T) {
	if _, err := domain.New%%TITLE%%("id-1", "", "Acme"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNew%%TITLE%%_Valid(t *testing.T) {
	e, err := domain.New%%TITLE%%("id-1", "upshs", "Acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.TenantID != "upshs" {
		t.Fatalf("tenant not set: got %q", e.TenantID)
	}
}
EOF
mv "$DIR/tests/unit/%%NAME%%_test.go.tmpl" "$DIR/tests/unit/${SVCNAME}_test.go"

for d in migrations tests/integration tests/contract tests/tenant_isolation; do : > "$DIR/$d/.gitkeep"; done

cat > "$DIR/Dockerfile" <<'EOF'
# %%DIR%% — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from the repo root: docker build -f apps/%%DIR%%/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY platform/ ./platform/
COPY apps/%%DIR%%/ ./apps/%%DIR%%/
WORKDIR /src/apps/%%DIR%%
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
EOF

cat > "$DIR/README.md" <<'EOF'
# %%DIR%%

Hexagonal Go service (agent_plan §5). Scaffolded by `make new-service NAME=%%NAME%%`.

**Status:** skeleton — health + wiring compile. Implement the 8-story spine (agent_plan §16):
domain+migrations, repository, CRUD+HTTP, events published/consumed, feature-flag gating,
tenant-isolation tests, observability+audit.

## Run
```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/%%DIR%%
curl localhost:8080/health
```

## Contract
REST: `contracts/openapi/%%NAME%%.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
EOF

# --- substitute placeholders across all generated files ---
find "$DIR" -type f \( -name '*.go' -o -name 'go.mod' -o -name 'Dockerfile' -o -name '*.md' \) -print0 |
while IFS= read -r -d '' f; do
  sed -i.bak \
    -e "s|%%MODULE%%|${SVCMODULE}|g" \
    -e "s|%%TITLE%%|${SVCTITLE}|g" \
    -e "s|%%DIR%%|${SVCDIR}|g" \
    -e "s|%%NAME%%|${SVCNAME}|g" \
    "$f"
  rm -f "$f.bak"
done

# --- wire into go.work (idempotent) ---
if ! grep -q "\./apps/${SVCDIR}\b" go.work 2>/dev/null; then
  if ! GOFLAGS=-mod=readonly go work use "./apps/${SVCDIR}" 2>/dev/null; then
    awk -v dir="./apps/${SVCDIR}" '
      /^use \(/ { print; print "\t" dir; next } { print }
    ' go.work > go.work.tmp && mv go.work.tmp go.work
  fi
fi

# --- format + self-verify build/test ---
gofmt -w "$DIR"
# Use GOTOOLCHAIN=auto for the self-verify so a locally installed older 1.25 patch
# can still compile the scaffold against platform/go.mod's go directive.
( cd "$DIR" && GOWORK=off GOFLAGS=-mod=readonly GOTOOLCHAIN=auto go build ./... \
  && GOWORK=off GOFLAGS=-mod=readonly GOTOOLCHAIN=auto go vet ./... \
  && GOWORK=off GOFLAGS=-mod=readonly GOTOOLCHAIN=auto go test ./... )

echo ""
echo "✅ $DIR scaffolded, builds, vets, and tests pass."
echo ""
echo "Next: add this block under services: in render.yaml (and a database under databases:):"
cat <<EOF
  - name: ${SVCDIR}
    type: pserv
    runtime: docker
    region: frankfurt
    plan: starter
    dockerfilePath: ./apps/${SVCDIR}/Dockerfile
    dockerContext: .
    autoDeployTrigger: checksPass
    buildFilter:
      paths: ["apps/${SVCDIR}/**", "platform/**"]
    envVars:
      - key: DATABASE_URL
        fromDatabase: { name: ${SVCNAME}-db, property: connectionString }
      - key: NATS_HOST
        fromService: { name: nats, property: host }
      - { fromGroup: auraedu-shared-config }
EOF