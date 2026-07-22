# AuraEDU marketing (Next.js 16 standalone) for Render. Build from repo root:
#   docker build -f apps/marketing/Dockerfile .
# Standalone output is traced from the monorepo root (next.config.ts outputFileTracingRoot).
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
    pnpm install --frozen-lockfile --filter marketing...
COPY . .
# Build shared-types so generated .d.ts are available for subpath imports.
RUN ./packages/shared-types/node_modules/.bin/tsc -p packages/shared-types/tsconfig.json
ARG ENVIRONMENT=development
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ARG NEXT_PUBLIC_APP_URL=https://app.auraedu.com
ENV ENVIRONMENT=$ENVIRONMENT
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL
RUN cd apps/marketing && ./node_modules/.bin/next build

FROM node:26.5.0-alpine@sha256:e88a35be04478413b7c71c455cd9865de9b9360e1f43456be5951032d7ac1a66 AS runner
ENV NODE_ENV=production
ENV PORT=3001
ENV NEXT_TELEMETRY_DISABLED=1
WORKDIR /repo
COPY --from=build --chown=node:node /repo/apps/marketing/.next/standalone ./
COPY --from=build --chown=node:node /repo/apps/marketing/.next/static ./apps/marketing/.next/static
# `next build` does not bundle public/ into standalone output.
COPY --from=build --chown=node:node /repo/apps/marketing/public ./apps/marketing/public
USER node
EXPOSE 3001
CMD ["node", "apps/marketing/server.js"]
