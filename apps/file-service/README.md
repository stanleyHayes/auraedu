# file-service

Hexagonal Go service (agent_plan §5). Manages file uploads, downloads, and metadata for a tenant.

**Status:** implemented — CRUD, multipart upload, download, RBAC, feature-flag gating, tenant isolation, events, and pluggable storage.

## Storage backends

The service supports two storage adapters selected at runtime:

| Backend     | Trigger                                            | Notes                                                    |
|-------------|----------------------------------------------------|----------------------------------------------------------|
| Local FS    | Default (no `CLOUDINARY_URL`)                      | Stores files under `FILE_STORAGE_DIR` scoped by tenant.  |
| Cloudinary  | `CLOUDINARY_URL=cloudinary://key:secret@cloud_name`| Uploads use tenant-scoped `public_id` under `CLOUDINARY_RESOURCE_TYPE` (default `raw`). |

## Environment variables

| Variable                   | Required | Default                         | Description                                |
|----------------------------|----------|---------------------------------|--------------------------------------------|
| `DATABASE_URL`             | yes      | —                               | Postgres connection string.                |
| `NATS_URL` / `NATS_HOST`   | no       | —                               | NATS URL for event publishing.             |
| `JWT_SIGNING_KEY`          | yes*     | —                               | Used by upstream/gateway; service reads actor headers. |
| `FEATURES_REGISTRY`        | no       | `../../contracts/features/features.yaml` | Feature-flag registry path (static snapshot, also the fallback). |
| `SERVICE_TENANT_URL`       | no       | —                               | Tenant-service base URL (e.g. `http://localhost:8082`); enables live per-tenant flag overrides with the static registry as fallback. Unset = static snapshot only. |
| `FILE_STORAGE_DIR`         | no       | `/tmp/auraedu-files`            | Local storage directory.                   |
| `CLOUDINARY_URL`           | no       | —                               | Enables Cloudinary storage.                |
| `CLOUDINARY_RESOURCE_TYPE` | no       | `raw`                           | Cloudinary resource type (`raw`, `image`, `auto`, ...). |

## Run locally

```bash
# local filesystem storage
GOFLAGS=-mod=readonly go run ./cmd/server

# cloudinary storage
CLOUDINARY_URL=cloudinary://key:secret@cloud_name go run ./cmd/server

curl localhost:8080/health
```

## API

REST contract: `contracts/openapi/file.v1.yaml`

- `GET    /api/v1/files`
- `POST   /api/v1/files` — multipart/form-data with `file` part
- `GET    /api/v1/files/{file_id}`
- `PATCH  /api/v1/files/{file_id}`
- `DELETE /api/v1/files/{file_id}`
- `GET    /api/v1/files/{file_id}/download`

Every action enforces: authenticated → tenant → RBAC (`files.read`/`create`/`update`/`delete`) → feature-flag (`file_management`) → ownership.

## Events

Publishes CloudEvents via NATS JetStream:

- `file.uploaded.v1`
- `file.updated.v1`
- `file.deleted.v1`
