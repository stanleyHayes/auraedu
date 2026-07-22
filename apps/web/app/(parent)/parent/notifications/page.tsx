import { Megaphone, BellOff } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

// created_at exists in the live notification-service DTO but not in the contract.
type Message = OpenAPI.notification_v1.components["schemas"]["Message"] & {
  created_at?: string;
};

export default async function ParentNotificationsPage() {
  const client = await createServerClient();

  const [announcementsResult, messagesResult] = await Promise.allSettled([
    client.get<OpenAPI.notification_v1.components["schemas"]["AnnouncementList"]>(
      "/api/v1/announcements",
    ),
    client.get<{ data?: Message[]; next_cursor?: string | null }>("/api/v1/messages"),
  ]);

  const announcements =
    announcementsResult.status === "fulfilled" ? (announcementsResult.value.data ?? []) : [];
  const messages = messagesResult.status === "fulfilled" ? (messagesResult.value.data ?? []) : [];
  const failed = announcementsResult.status === "rejected" && messagesResult.status === "rejected";

  if (failed) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<Megaphone className="size-6" />}
          title="Notifications"
          description="School announcements and notices for parents."
        />
        <EmptyState
          icon={<BellOff className="size-8" />}
          title="Notifications unavailable"
          description="Could not load notifications right now."
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Megaphone className="size-6" />}
        title="Notifications"
        description="School announcements and notices for parents."
      />

      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-sans font-semibold tracking-tight">Announcements</h3>
        {announcements.length === 0 ? (
          <div className="mt-4">
            <EmptyState
              icon={<Megaphone className="size-8" />}
              title="No announcements"
              description="School announcements will appear here."
            />
          </div>
        ) : (
          <ul className="mt-4 space-y-3">
            {announcements.map((announcement) => (
              <li
                key={announcement.id}
                className="rounded-[var(--radius-md)] border border-[var(--border)] p-4"
              >
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h4 className="font-medium text-[var(--foreground)]">{announcement.title}</h4>
                    <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                      {announcement.body}
                    </p>
                  </div>
                  {announcement.created_at ? (
                    <span className="shrink-0 text-xs text-[var(--muted-foreground)]">
                      {new Date(announcement.created_at).toLocaleDateString()}
                    </span>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-sans font-semibold tracking-tight">Messages</h3>
        {messages.length === 0 ? (
          <div className="mt-4">
            <EmptyState
              icon={<BellOff className="size-8" />}
              title="No messages"
              description="Direct messages from the school will appear here."
            />
          </div>
        ) : (
          <ul className="mt-4 space-y-3">
            {messages.map((message) => (
              <li
                key={message.id}
                className="rounded-[var(--radius-md)] border border-[var(--border)] p-4"
              >
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h4 className="font-medium text-[var(--foreground)]">
                      {message.subject ?? "Message"}
                    </h4>
                    {message.body ? (
                      <p className="mt-1 text-sm text-[var(--muted-foreground)]">{message.body}</p>
                    ) : null}
                    <p className="mt-1 text-xs capitalize text-[var(--muted-foreground)]">
                      {message.channel} · {message.status}
                    </p>
                  </div>
                  {message.created_at ? (
                    <span className="shrink-0 text-xs text-[var(--muted-foreground)]">
                      {new Date(message.created_at).toLocaleDateString()}
                    </span>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
