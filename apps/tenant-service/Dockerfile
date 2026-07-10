# tenant-service — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from repo root: docker build -f apps/tenant-service/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY platform/ ./platform/
COPY apps/tenant-service/ ./apps/tenant-service/
WORKDIR /src/apps/tenant-service
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8082
USER nonroot:nonroot
ENTRYPOINT ["/app"]
