# academic-service — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from the repo root: docker build -f apps/academic-service/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY platform/ ./platform/
COPY contracts/ ./contracts/
COPY apps/academic-service/ ./apps/academic-service/
WORKDIR /src/apps/academic-service
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local FEATURES_REGISTRY=/src/contracts/features/features.yaml
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
COPY --from=build /src/contracts /contracts
ENV FEATURES_REGISTRY=/contracts/features/features.yaml
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
