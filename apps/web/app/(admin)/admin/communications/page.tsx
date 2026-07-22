import { Megaphone, Radio, Send, Users } from "lucide-react";
import { DataTable, EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import {
  AnnouncementFormSheet,
  DeleteAnnouncementButton,
} from "@/components/announcement-workflow";
import { createServerClient } from "@/lib/api";

type Announcement = OpenAPI.notification_v1.components["schemas"]["Announcement"];

export default async function AdminCommunicationsPage() {
  let rows: Announcement[] = [];
  let error: string | null = null;
  try {
    const client = await createServerClient();
    const list = await client.get<
      OpenAPI.notification_v1.components["schemas"]["AnnouncementList"]
    >("/api/v1/announcements?limit=100");
    rows = list.data ?? [];
  } catch (caught) {
    error =
      caught instanceof Error ? caught.message : "The announcement service could not be reached.";
  }
  const targeted = rows.filter((item) => item.audience !== "all").length;
  return (
    <div className="space-y-7">
      <PageHeader
        icon={<Megaphone className="size-6" />}
        title="Communications"
        description="Publish trusted school updates to the people who need to act on them."
        action={<AnnouncementFormSheet />}
      />
      <section className="grid gap-4 sm:grid-cols-3">
        <Reveal>
          <StatCard label="Published updates" value={rows.length} unit="in view" />
        </Reveal>
        <Reveal delay={70}>
          <StatCard
            label="Targeted messages"
            value={targeted}
            unit="role scoped"
            tone={targeted ? "ok" : "default"}
          />
        </Reveal>
        <Reveal delay={140}>
          <StatCard
            label="Whole school"
            value={rows.filter((item) => item.audience === "all").length}
            unit="announcements"
          />
        </Reveal>
      </section>
      <Reveal delay={100}>
        <section className="relative overflow-hidden rounded-3xl border border-[var(--border)] bg-[var(--foreground)] px-6 py-5 text-[var(--background)]">
          <span
            aria-hidden
            className="absolute -right-20 -top-24 size-64 rounded-full bg-[var(--portal-accent,var(--color-brand))]/35 blur-3xl"
          />
          <div className="relative flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
            <div className="flex items-start gap-3">
              <Radio className="mt-1 size-5 text-[var(--portal-signal,var(--color-signal))]" />
              <div>
                <h2 className="font-heading text-xl font-bold">
                  One record. Every enabled surface.
                </h2>
                <p className="mt-1 text-sm leading-6 text-[var(--background)]/65">
                  Audience policy is enforced by Notification Service; students, families, and staff
                  only retrieve announcements intended for their role.
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--background)]/60">
              <Users className="size-4" />
              Role aware
              <Send className="size-4" />
            </div>
          </div>
        </section>
      </Reveal>
      {error ? (
        <EmptyState
          icon={<Megaphone className="size-8" />}
          title="Communications unavailable"
          description={error}
        />
      ) : (
        <Reveal delay={160}>
          <section className="overflow-hidden rounded-3xl border border-[var(--border)] bg-[var(--surface)] p-2 shadow-[0_14px_42px_rgba(6,22,49,0.06)] sm:p-4">
            <DataTable
              caption="Announcements"
              rows={rows}
              keyExtractor={(announcement) => announcement.id}
              columns={[
                {
                  key: "title",
                  header: "Announcement",
                  cell: (announcement) => (
                    <div>
                      <strong className="block">{announcement.title}</strong>
                      <small className="line-clamp-2 max-w-xl text-[var(--muted-foreground)]">
                        {announcement.body}
                      </small>
                    </div>
                  ),
                },
                {
                  key: "audience",
                  header: "Audience",
                  cell: (announcement) => (
                    <span className="rounded-full bg-[var(--muted)] px-2.5 py-1 text-xs font-bold capitalize">
                      {announcement.audience === "guardians"
                        ? "Parents & guardians"
                        : announcement.audience}
                    </span>
                  ),
                },
                {
                  key: "created",
                  header: "Published",
                  cell: (announcement) =>
                    announcement.created_at
                      ? new Date(announcement.created_at).toLocaleString("en-GB")
                      : "Not recorded",
                },
                {
                  key: "actions",
                  header: "",
                  className: "w-12",
                  cell: (announcement) => (
                    <DeleteAnnouncementButton id={announcement.id} title={announcement.title} />
                  ),
                },
              ]}
              empty={
                <EmptyState
                  icon={<Megaphone className="size-8" />}
                  title="Share the first school update"
                  description="Publish an announcement to make communications visible in web and mobile role inboxes."
                />
              }
            />
          </section>
        </Reveal>
      )}
    </div>
  );
}
