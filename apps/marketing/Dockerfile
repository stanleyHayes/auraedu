# AuraEDU marketing (Next.js 16 standalone) for Render. Build from repo root:
#   docker build -f apps/marketing/Dockerfile .
# Standalone output is traced from the monorepo root (next.config.ts outputFileTracingRoot).
FROM node:24-alpine AS base
RUN corepack enable
WORKDIR /repo

FROM base AS build
COPY . .
RUN pnpm install --frozen-lockfile
RUN pnpm --filter marketing build

FROM base AS runner
ENV NODE_ENV=production
ENV PORT=3001
WORKDIR /repo
COPY --from=build /repo/apps/marketing/.next/standalone ./
COPY --from=build /repo/apps/marketing/.next/static ./apps/marketing/.next/static
EXPOSE 3001
CMD ["node", "apps/marketing/server.js"]
