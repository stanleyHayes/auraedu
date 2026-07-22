# AuraEDU web portal (Next.js 16 standalone) for Render. Build from repo root:
#   docker build -f apps/web/Dockerfile .
FROM node:26.5.0-alpine@sha256:e88a35be04478413b7c71c455cd9865de9b9360e1f43456be5951032d7ac1a66 AS build
RUN npm install --global pnpm@11.11.0
WORKDIR /repo
ENV NEXT_TELEMETRY_DISABLED=1
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/package.json
COPY apps/marketing/package.json apps/marketing/package.json
COPY apps/mobile/package.json apps/mobile/package.json
COPY packages/api-client/package.json packages/api-client/package.json
COPY packages/config/package.json packages/config/package.json
COPY packages/eslint-config/package.json packages/eslint-config/package.json
COPY packages/feature-flags/package.json packages/feature-flags/package.json
COPY packages/logger/package.json packages/logger/package.json
COPY packages/shared-types/package.json packages/shared-types/package.json
COPY packages/tokens/package.json packages/tokens/package.json
COPY packages/ui/package.json packages/ui/package.json
COPY tools/codegen/package.json tools/codegen/package.json
RUN --mount=type=cache,id=auraedu-pnpm,target=/root/.local/share/pnpm/store,sharing=locked \
    pnpm install --frozen-lockfile --filter web...
COPY . .
# Build shared-types so generated .d.ts are available for subpath imports.
RUN ./packages/shared-types/node_modules/.bin/tsc -p packages/shared-types/tsconfig.json
ARG ENVIRONMENT=development
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ARG NEXT_PUBLIC_APP_URL=http://localhost:3000
ENV ENVIRONMENT=$ENVIRONMENT
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL
RUN cd apps/web && ./node_modules/.bin/next build

FROM node:26.5.0-alpine@sha256:e88a35be04478413b7c71c455cd9865de9b9360e1f43456be5951032d7ac1a66 AS runner
ENV NODE_ENV=production
ENV PORT=3000
ENV NEXT_TELEMETRY_DISABLED=1
WORKDIR /repo
COPY --from=build --chown=node:node /repo/apps/web/.next/standalone ./
COPY --from=build --chown=node:node /repo/apps/web/.next/static ./apps/web/.next/static
# `next build` does not bundle public/ into standalone output. Without this the
# brand SVGs 404 at the static layer and fall through to the (public)/[tenant]
# route, which then 500s trying to resolve a tenant named "brand".
COPY --from=build --chown=node:node /repo/apps/web/public ./apps/web/public
USER node
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
