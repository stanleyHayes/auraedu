# student-service — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from the repo root: docker build -f apps/student-service/Dockerfile .
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
COPY platform/ ./platform/
COPY contracts/ ./contracts/
COPY apps/student-service/ ./apps/student-service/
WORKDIR /src/apps/student-service
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local FEATURES_REGISTRY=/src/contracts/features/features.yaml
ARG GIT_SHA=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/student-service

FROM gcr.io/distroless/static-debian12:nonroot@sha256:aef9602f8710ec12bde19d593fed1f76c708531bb7aba205110f1029786ead7b
COPY --from=build /out/app /app
COPY --from=build /src/apps/student-service/migrations /migrations
COPY --from=build /src/contracts /contracts
ENV FEATURES_REGISTRY=/contracts/features/features.yaml
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
CMD ["server"]
