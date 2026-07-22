# `@auraedu/logger`

Structured JSON logging for AuraEDU browser and Node.js TypeScript surfaces. `createLogger` adds service context, respects log levels, serializes errors, and recursively redacts sensitive keys before writing a record.

```ts
import { createLogger } from "@auraedu/logger";

const log = createLogger({ service: "web", level: "info" });
log.info("session refreshed", { tenantId, requestId });
```

Do not log access tokens, cookies, credentials, raw authorization headers, or student data. Redaction is a safety net, not permission to pass secrets into the logger. Verify with the package `typecheck`, `lint`, and `test` scripts.
