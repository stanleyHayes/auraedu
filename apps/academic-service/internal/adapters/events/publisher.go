package events

// TODO(AURA): publish/consume domain events via platform/eventbus (NATS JetStream,
// CloudEvents). Every event MUST carry tenant_id; workers skip disabled-feature tenants.
