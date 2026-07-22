# `@auraedu/api-client`

A small typed transport shared by the web and mobile clients for calls through the AuraEDU API Gateway.

`createGatewayClient` centralizes the base URL, bearer token, tenant and request headers, JSON decoding, and normalized errors. It exposes `request`, `getFeatureSnapshot`, and `getTenantBranding`; `toFeatureSnapshot` converts the gateway response into the client feature-flag shape.

## Usage

```ts
import { createGatewayClient } from "@auraedu/api-client";

const gateway = createGatewayClient({
  baseUrl: process.env.NEXT_PUBLIC_API_URL!,
  getAccessToken: () => session.accessToken,
  getTenantId: () => session.tenantId,
});

const profile = await gateway.request<UserProfile>("/api/v1/profile");
```

Use `ApiError`, `UnauthorizedError`, and `FeatureDisabledError` for explicit UI states. Never call private services or their databases from a frontend. Verify with the package `typecheck`, `lint`, and `test` scripts.
