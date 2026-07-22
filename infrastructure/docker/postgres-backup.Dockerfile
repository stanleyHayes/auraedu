# Hourly ownership-neutral PostgreSQL 18 logical exports to immutable off-site storage.
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0 GOTOOLCHAIN=local
COPY tools/dr/postgres-backup/go.mod ./
COPY tools/dr/postgres-backup/ ./
RUN --mount=type=cache,target=/root/.cache/go-build go build -trimpath -ldflags="-s -w" -o /out/postgres-backup .

FROM postgres:18-alpine@sha256:9a8afca54e7861fd90fab5fdf4c42477a6b1cb7d293595148e674e0a3181de15
COPY --from=build /out/postgres-backup /usr/local/bin/auraedu-postgres-backup
USER 70:70
ENTRYPOINT ["/usr/local/bin/auraedu-postgres-backup"]
