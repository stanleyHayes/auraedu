import type { OpenAPI } from "@auraedu/shared-types";

export type Message = OpenAPI.notification_v1.components["schemas"]["Message"];

/** Effective delivery state: provider outcome wins once reported, otherwise the message status. */
export function effectiveDeliveryState(message: Message): string {
  return message.delivery_status ?? message.status;
}

export function deliveryStateStyle(state: string) {
  switch (state) {
    case "delivered":
    case "sent":
    case "accepted":
      return "border-emerald-300 bg-emerald-50 text-emerald-900";
    case "failed":
    case "bounced":
    case "complained":
    case "suppressed":
      return "border-red-300 bg-red-50 text-red-900";
    case "pending":
    case "delayed":
      return "border-amber-300 bg-amber-50 text-amber-900";
    default:
      return "border-border bg-muted text-muted-foreground";
  }
}

export function deliveryDotStyle(state: string) {
  switch (state) {
    case "delivered":
    case "sent":
    case "accepted":
      return "bg-emerald-500";
    case "failed":
    case "bounced":
    case "complained":
    case "suppressed":
      return "bg-red-500";
    case "pending":
    case "delayed":
      return "bg-amber-500";
    default:
      return "bg-muted-foreground";
  }
}

export function channelLabel(channel: Message["channel"]) {
  return channel.replaceAll("_", " ");
}
