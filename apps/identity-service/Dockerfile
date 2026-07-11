# identity-service — hermetic single-module build (go.mod replaces platform → ../../platform).
# Build from repo root: docker build -f apps/identity-service/Dockerfile .
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY platform/ ./platform/
COPY apps/identity-service/ ./apps/identity-service/
WORKDIR /src/apps/identity-service
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=dev
RUN go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o /out/app ./cmd/identity-service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/app"]
CMD ["server"]
