# Multi-stage build for the AuraEDU API Gateway (Go). Built from the repo root so
# the local `platform` module is available: `docker build -f apps/api-gateway/Dockerfile .`
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src

# Hermetic single-module build: the gateway's go.mod `replace`s platform → ../../platform,
# so we only need those two dirs. GOWORK=off avoids depending on go.work listing every service.
COPY platform/ ./platform/
COPY apps/api-gateway/ ./apps/api-gateway/

WORKDIR /src/apps/api-gateway
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/gateway ./cmd/api-gateway

FROM gcr.io/distroless/static-debian12:nonroot@sha256:aef9602f8710ec12bde19d593fed1f76c708531bb7aba205110f1029786ead7b
COPY --from=build /out/gateway /gateway
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/gateway"]
CMD ["server"]
