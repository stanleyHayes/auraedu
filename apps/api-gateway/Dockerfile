# Multi-stage build for the AuraEDU API Gateway (Go). Built from the repo root so
# the local `platform` module is available: `docker build -f apps/api-gateway/Dockerfile .`
FROM golang:1.25-alpine AS build
WORKDIR /src

# Copy the workspace + both modules (gateway depends on platform).
COPY go.work go.work.sum* ./
COPY platform/ ./platform/
COPY apps/api-gateway/ ./apps/api-gateway/

ENV GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" \
    -o /out/gateway ./apps/api-gateway/cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gateway /gateway
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/gateway"]
