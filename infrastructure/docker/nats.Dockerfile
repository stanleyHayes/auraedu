# NATS JetStream for AuraEDU (event bus, agent_plan §3). Deployed as a Render
# private service with a persistent disk mounted at /data for JetStream storage.
FROM nats:2.11-alpine@sha256:e4bf19f15fd3218814a4e3c9e0064e1334bd8aa20d5984b9f1a0afd084f8cc00
RUN mkdir -p /data && chown 65532:65532 /data
USER 65532:65532
# -js enables JetStream; -sd sets the storage directory (persisted via Render disk).
CMD ["-js", "-sd", "/data", "-m", "8222"]
