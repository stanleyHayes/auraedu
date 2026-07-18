# analytics-service — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from the repo root: docker build -f apps/analytics-service/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY platform/ ./platform/
COPY apps/analytics-service/ ./apps/analytics-service/
WORKDIR /src/apps/analytics-service
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/analytics-service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
CMD ["server"]
