# Scheduled JetStream account backup. The binary fails closed unless immutable,
# encrypted off-site storage plus success-heartbeat and failure-alert endpoints
# are configured. Build from the repository root.
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0 GOTOOLCHAIN=local
COPY tools/dr/nats-backup/go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod GOBIN=/out go install github.com/nats-io/natscli/nats@v0.3.1
COPY tools/dr/nats-backup/ ./
RUN --mount=type=cache,target=/root/.cache/go-build go build -trimpath -ldflags="-s -w" -o /out/backup .

FROM gcr.io/distroless/static-debian12:nonroot@sha256:aef9602f8710ec12bde19d593fed1f76c708531bb7aba205110f1029786ead7b
ENV HOME=/tmp XDG_CONFIG_HOME=/tmp/.config
COPY --from=build /out/backup /backup
COPY --from=build /out/nats /nats
USER nonroot:nonroot
ENTRYPOINT ["/backup"]
