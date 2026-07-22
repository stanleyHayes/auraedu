import { relayProviderWebhook } from "@/lib/provider-webhook-relay";

export function POST(request: Request) {
  return relayProviderWebhook(request, {
    callbackPath: "/api/v1/webhooks/resend",
    contentType: "application/json",
    forwardedHeaders: ["svix-id", "svix-timestamp", "svix-signature"],
  });
}
