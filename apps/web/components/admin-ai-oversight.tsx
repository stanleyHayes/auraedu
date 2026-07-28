import Link from "next/link";
import { Sparkles } from "lucide-react";
import { DataTable, EmptyState } from "@auraedu/ui";
import { aiStatusStyle, formatConfidence, type AiOversightItem } from "@/lib/admin-ai";

export interface AiOversightQuery {
  student_id?: string;
  status?: string;
  cursor?: string;
}

export function AdminAiOversightList({
  basePath,
  noun,
  items,
  studentNames,
  statusOptions,
  query,
  nextCursor,
  error,
  emptyTitle,
  emptyDescription,
}: {
  basePath: string;
  noun: string;
  items: AiOversightItem[];
  studentNames: Map<string, string>;
  statusOptions: string[];
  query: AiOversightQuery;
  nextCursor: string | null;
  error: string | null;
  emptyTitle: string;
  emptyDescription: string;
}) {
  const nextParams = new URLSearchParams();
  if (query.student_id?.trim()) nextParams.set("student_id", query.student_id.trim());
  if (query.status?.trim()) nextParams.set("status", query.status.trim());
  if (nextCursor) nextParams.set("cursor", nextCursor);

  return (
    <>
      <form
        className="flex flex-col gap-3 rounded-2xl border border-border bg-surface p-4 sm:flex-row sm:items-end"
        method="get"
      >
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Student ID
          <input
            type="text"
            name="student_id"
            defaultValue={query.student_id}
            placeholder="Student UUID"
            className="mt-2 block h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground sm:w-64"
          />
        </label>
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Status
          <select
            name="status"
            defaultValue={query.status ?? ""}
            className="mt-2 block h-10 rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground"
          >
            <option value="">All statuses</option>
            {statusOptions.map((status) => (
              <option key={status} value={status} className="capitalize">
                {status}
              </option>
            ))}
          </select>
        </label>
        <div className="flex gap-2">
          <button
            type="submit"
            className="h-10 rounded-md bg-primary px-5 text-sm font-bold text-primary-foreground transition hover:-translate-y-0.5 hover:shadow-md"
          >
            Apply filters
          </button>
          {query.student_id || query.status || query.cursor ? (
            <Link
              href={basePath}
              className="grid h-10 place-items-center rounded-md border border-border px-4 text-sm font-semibold text-muted-foreground transition hover:text-foreground"
            >
              Reset
            </Link>
          ) : null}
        </div>
      </form>

      {error ? (
        <EmptyState
          title={`Could not load ${noun}`}
          description={
            query.student_id?.trim()
              ? error
              : `${error} If the service requires a student reference, enter a student ID above to inspect one learner at a time.`
          }
          icon={<Sparkles className="size-8" />}
        />
      ) : (
        <>
          <DataTable
            caption={noun}
            rows={items}
            keyExtractor={(item) => item.id}
            columns={[
              {
                key: "item",
                header: "Item",
                cell: (item) => (
                  <div className="max-w-md">
                    <p className="font-semibold">{item.title}</p>
                    <p className="mt-0.5 text-xs capitalize text-muted-foreground">
                      {item.itemType.replaceAll("_", " ")}
                      {item.detail ? ` · ${item.detail}` : ""}
                    </p>
                  </div>
                ),
              },
              {
                key: "student",
                header: "Student",
                cell: (item) =>
                  studentNames.get(item.student_id) ? (
                    <span className="font-semibold">{studentNames.get(item.student_id)}</span>
                  ) : (
                    <span className="font-mono text-xs">{item.student_id.slice(0, 8)}…</span>
                  ),
              },
              {
                key: "status",
                header: "Status",
                cell: (item) => (
                  <span
                    className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${aiStatusStyle(item.status)}`}
                  >
                    {item.status}
                  </span>
                ),
              },
              {
                key: "confidence",
                header: "Confidence",
                cell: (item) => (
                  <div className="max-w-xs">
                    <span className="font-heading text-lg font-black">
                      {formatConfidence(item.confidence)}
                    </span>
                    {item.explanation ? (
                      <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                        {item.explanation}
                      </p>
                    ) : null}
                  </div>
                ),
              },
              {
                key: "created",
                header: "Created",
                cell: (item) => (
                  <span className="whitespace-nowrap text-sm">
                    {new Intl.DateTimeFormat("en-GH", {
                      dateStyle: "medium",
                      timeStyle: "short",
                    }).format(new Date(item.created_at))}
                  </span>
                ),
              },
            ]}
            empty={
              <EmptyState
                title={emptyTitle}
                description={emptyDescription}
                icon={<Sparkles className="size-8" />}
              />
            }
          />
          {nextCursor ? (
            <div className="flex justify-end">
              <Link
                href={`${basePath}?${nextParams}`}
                className="rounded-md border border-border bg-surface px-4 py-2 text-sm font-semibold text-primary transition hover:-translate-y-0.5 hover:shadow-md"
              >
                Older {noun} →
              </Link>
            </div>
          ) : null}
        </>
      )}
    </>
  );
}
