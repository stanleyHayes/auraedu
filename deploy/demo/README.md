# AuraEDU demo — the whole stack for $0/month

This runs every Go backend service in **one container** so a working demo fits a
single free instance. It is not the production topology: `render.yaml` still runs
one service per process, and that is what you deploy for real schools.

## Why one container

Render's free tier grants **750 instance-hours per workspace per month**. One
always-on service uses ~730 of them, so "all services free" is not a configuration
problem — the arithmetic only allows one service. Collapsing the processes into one
container is what makes the demo free; nothing about the service code changes, each
still listens on its own port and talks to its siblings over HTTP as in production.

## The free stack

| Piece | Provider | Free tier |
|---|---|---|
| Backend (18 services + gateway) | Render web service | 750 instance-hours/mo, sleeps after 15 min idle |
| PostgreSQL | Neon | permanent, 0.5 GB, scales to zero |
| Redis + NATS | inside the container | ephemeral by design |
| Web + marketing | Vercel | Hobby |

Measured footprint: **~120 MiB of the 512 MiB** limit with all 18 services running.

Redis and NATS run in-container deliberately. The instance sleeps and restarts
freely, so only PostgreSQL holds anything worth keeping — and that lives in Neon.

## Deploy

1. **Database.** Create a Neon project and copy its connection string. Every service
   gets its own schema inside that one database automatically.
2. **Backend.** On Render, create a *Web Service* from this repo — not a Blueprint,
   which would deploy the full production topology:
   - Dockerfile path: `deploy/demo/Dockerfile`
   - Docker context: the repository root
   - Instance type: Free
   - Environment: `DATABASE_URL` = the Neon string.
     Optionally set `JWT_SIGNING_KEY` and `INTERNAL_SERVICE_TOKEN`; both fall back to
     development values that are fine for a demo and unsafe for anything else.
3. **Frontends.** Deploy `apps/web` and `apps/marketing` to Vercel and point their
   gateway origin at the Render URL.
4. **Seed.** From a machine that can reach the database:

   ```
   B="<neon-url>&options=-csearch_path%3D"
   IDENTITY_DATABASE_URL="${B}identity" TENANT_DATABASE_URL="${B}tenant" \
   BILLING_DATABASE_URL="${B}billing" STUDENT_DATABASE_URL="${B}student" \
   STAFF_DATABASE_URL="${B}staff" go run ./tools/seed
   ```

   Sign-in details are written to `credentials.txt`.

## Run it locally

```
docker build -f deploy/demo/Dockerfile -t auraedu-demo .
docker run --rm -p 8080:8080 -e DATABASE_URL="postgres://...@host:5432/auraedu?sslmode=disable" auraedu-demo
curl localhost:8080/ready
```

## What the demo leaves out

The growth/marketing suite (CRM, campaigns, content AI, the website assistant and
its knowledge base, market intelligence) is not included — every feature it serves is
off for all tenants. See `deploy/undeployed-services.txt`.

The three Python AI services are also omitted; they need a second runtime in the
image and their features are `ai_plus` tier.

## Known limits

- **It sleeps.** After 15 minutes idle the instance spins down and the next request
  waits ~1 minute while 18 processes and their migrations start.
- **No backups.** Neon's free tier and an ephemeral NATS do not satisfy the recovery
  policy in `tools/ci/check-disaster-recovery.sh`. That gate guards production and
  this image is not production.
- **One instance.** No per-service scaling; a process that dies takes the instance
  down so the platform restarts it.
