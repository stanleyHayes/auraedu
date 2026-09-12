# AuraEDU demo — every backend process in ONE container so the whole stack fits a
# single free instance. Production runs one Render service per process (render.yaml);
# this image exists only so a working demo costs nothing. It is NOT a production
# topology: there is no per-service scaling, and NATS/Redis state is ephemeral.
#
# Build from the repo root:  docker build -f deploy/demo/Dockerfile -t auraedu-demo .
FROM golang:1.26.5-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
COPY platform/ ./platform/
COPY contracts/ ./contracts/
COPY apps/api-gateway/ ./apps/api-gateway/
COPY apps/identity-service/ ./apps/identity-service/
COPY apps/tenant-service/ ./apps/tenant-service/
COPY apps/student-service/ ./apps/student-service/
COPY apps/staff-service/ ./apps/staff-service/
COPY apps/academic-service/ ./apps/academic-service/
COPY apps/file-service/ ./apps/file-service/
COPY apps/attendance-service/ ./apps/attendance-service/
COPY apps/assessment-service/ ./apps/assessment-service/
COPY apps/report-service/ ./apps/report-service/
COPY apps/fees-service/ ./apps/fees-service/
COPY apps/payment-service/ ./apps/payment-service/
COPY apps/notification-service/ ./apps/notification-service/
COPY apps/billing-service/ ./apps/billing-service/
COPY apps/website-service/ ./apps/website-service/
COPY apps/analytics-service/ ./apps/analytics-service/
COPY apps/cbt-service/ ./apps/cbt-service/
COPY apps/audit-service/ ./apps/audit-service/
COPY apps/admissions-service/ ./apps/admissions-service/
ENV GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=0 GOTOOLCHAIN=local
ARG GIT_SHA=demo
# Every service's entrypoint is cmd/<service-name>; cmd/server and cmd/worker are
# packages it dispatches to, not main packages.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    set -eux; \
    for svc in api-gateway identity-service tenant-service student-service staff-service academic-service file-service attendance-service assessment-service report-service fees-service payment-service notification-service billing-service website-service analytics-service cbt-service audit-service admissions-service; do \
      cd "/src/apps/$svc"; \
      go build -ldflags "-s -w -X main.version=${GIT_SHA}" -o "/out/$svc" "./cmd/$svc"; \
    done

FROM alpine:3.22
RUN apk add --no-cache nats-server redis ca-certificates tini
WORKDIR /srv
COPY --from=build /out/ /usr/local/bin/
COPY --from=build /src/contracts /contracts
COPY apps/identity-service/migrations /srv/identity-service/migrations
COPY apps/tenant-service/migrations /srv/tenant-service/migrations
COPY apps/student-service/migrations /srv/student-service/migrations
COPY apps/staff-service/migrations /srv/staff-service/migrations
COPY apps/academic-service/migrations /srv/academic-service/migrations
COPY apps/file-service/migrations /srv/file-service/migrations
COPY apps/attendance-service/migrations /srv/attendance-service/migrations
COPY apps/assessment-service/migrations /srv/assessment-service/migrations
COPY apps/report-service/migrations /srv/report-service/migrations
COPY apps/fees-service/migrations /srv/fees-service/migrations
COPY apps/payment-service/migrations /srv/payment-service/migrations
COPY apps/notification-service/migrations /srv/notification-service/migrations
COPY apps/billing-service/migrations /srv/billing-service/migrations
COPY apps/website-service/migrations /srv/website-service/migrations
COPY apps/analytics-service/migrations /srv/analytics-service/migrations
COPY apps/cbt-service/migrations /srv/cbt-service/migrations
COPY apps/audit-service/migrations /srv/audit-service/migrations
COPY apps/admissions-service/migrations /srv/admissions-service/migrations
COPY deploy/demo/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY deploy/demo/services.txt /srv/services.txt
RUN chmod +x /usr/local/bin/entrypoint.sh
ENV FEATURES_REGISTRY=/contracts/features/features.yaml
EXPOSE 8080
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/entrypoint.sh"]
