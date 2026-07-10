# NATS JetStream for AuraEDU (event bus, agent_plan §3). Deployed as a Render
# private service with a persistent disk mounted at /data for JetStream storage.
FROM nats:2.11-alpine
# -js enables JetStream; -sd sets the storage directory (persisted via Render disk).
CMD ["-js", "-sd", "/data", "-m", "8222"]
