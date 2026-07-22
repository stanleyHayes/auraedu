# `@auraedu/config`

Validated frontend configuration for the AuraEDU web and marketing applications. Zod schemas separate browser-safe values from server-only values and normalize URLs, log levels, and the tenant header name.

Use `parseWebClientEnv`, `parseWebServerEnv`, or `parseMarketingClientEnv` in tests and startup checks. Application code can consume `webEnv`, `getMarketingEnv`, `publicApiUrl`, `publicAppUrl`, `tenantHeaderName`, and `gatewayInternalUrl`.

Never expose a server credential through a `NEXT_PUBLIC_` variable. Invalid required values must fail startup instead of silently selecting a production fallback. Verify with the package `typecheck`, `lint`, and `test` scripts.
