# Render Blueprint fragments

Render requires a single [`render.yaml`](../../render.yaml) at the repository root.
This directory holds reusable fragments that make up that Blueprint, so L5 can review
and discuss sections in isolation before assembling them.

| Fragment | Contents |
|---|---|
| `00-env-groups.yaml` | Shared env groups (`auraedu-shared-config`, `auraedu-secrets`) |
| `10-databases.yaml` | Render Postgres databases (one per service DB) |
| `20-shared-services.yaml` | Shared infrastructure: Redis/Valkey keyvalue + NATS JetStream pserv |
| `30-service-pattern.yaml` | Reusable templates for `web`, `pserv`, `worker`, and `cron` services |

When adding a new service, update the relevant fragment, then copy the assembled result
into `render.yaml`. Keep the fragments and the root Blueprint in sync.
