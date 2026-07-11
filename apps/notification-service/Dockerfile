# notification-service — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from the repo root: docker build -f apps/notification-service/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY platform/ ./platform/
COPY apps/notification-service/ ./apps/notification-service/
WORKDIR /src/apps/notification-service
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/notification-service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
CMD ["server"]
