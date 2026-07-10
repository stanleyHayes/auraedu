# Multi-stage build for the AuraEDU API Gateway (Go). Built from the repo root so
# the local `platform` module is available: `docker build -f apps/api-gateway/Dockerfile .`
FROM golang:1.25-alpine AS build
WORKDIR /src

# Hermetic single-module build: the gateway's go.mod `replace`s platform → ../../platform,
# so we only need those two dirs. GOWORK=off avoids depending on go.work listing every service.
COPY platform/ ./platform/
COPY apps/api-gateway/ ./apps/api-gateway/

WORKDIR /src/apps/api-gateway
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/gateway ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gateway /gateway
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/gateway"]
