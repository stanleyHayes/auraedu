# AuraEDU web portal (Next.js 16 standalone) for Render. Build from repo root:
#   docker build -f apps/web/Dockerfile .
FROM node:24-alpine AS base
RUN corepack enable
WORKDIR /repo

FROM base AS build
COPY . .
RUN pnpm install --frozen-lockfile
# Build shared-types so generated .d.ts are available for subpath imports.
RUN pnpm --filter @auraedu/shared-types run build
RUN pnpm --filter web build

FROM base AS runner
ENV NODE_ENV=production
ENV PORT=3000
WORKDIR /repo
COPY --from=build /repo/apps/web/.next/standalone ./
COPY --from=build /repo/apps/web/.next/static ./apps/web/.next/static
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
