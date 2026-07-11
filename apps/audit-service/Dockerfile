# audit-service — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from the repo root: docker build -f apps/audit-service/Dockerfile .
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY platform/ ./platform/
COPY apps/audit-service/ ./apps/audit-service/
WORKDIR /src/apps/audit-service
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/server && \
    go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
COPY --from=build /out/worker /worker
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
