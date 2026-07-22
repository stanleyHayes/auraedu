import { revalidatePath, revalidateTag } from "next/cache";
import { BookOpen, CalendarPlus, CheckCircle2, CircleOff } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { createServerClient, getCurrentTenantCode } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Programme = OpenAPI.admissions_v1.components["schemas"]["Programme"];
type ProgrammeStatus = OpenAPI.admissions_v1.components["schemas"]["ProgrammeStatus"];
type IntakeStatus = OpenAPI.admissions_v1.components["schemas"]["IntakeStatus"];

function text(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value.trim() : "";
}

async function refreshCatalogue() {
  const tenant = await getCurrentTenantCode();
  revalidatePath("/admin/programmes");
  revalidatePath(`/${tenant}/programmes`);
  revalidateTag(`programmes:${tenant}`, "max");
}

async function createProgramme(data: FormData) {
  "use server";
  const client = await createServerClient();
  await client.post("/api/v1/programmes", {
    code: text(data, "code").toUpperCase(),
    name: text(data, "name"),
    slug: text(data, "slug").toLowerCase(),
    summary: text(data, "summary"),
    description: text(data, "description"),
  });
  await refreshCatalogue();
}

async function setProgrammeStatus(data: FormData) {
  "use server";
  const id = text(data, "id");
  const status = text(data, "status") as ProgrammeStatus;
  if (!id) return;
  const client = await createServerClient();
  await client.patch(`/api/v1/programmes/${id}`, { status });
  await refreshCatalogue();
}

async function createIntake(data: FormData) {
  "use server";
  const programmeID = text(data, "programme_id");
  const starts = new Date(text(data, "starts_at"));
  const opens = new Date(text(data, "application_opens_at"));
  const closes = new Date(text(data, "application_closes_at"));
  const capacityValue = text(data, "capacity");
  if (!programmeID || ![starts, opens, closes].every((value) => Number.isFinite(value.getTime())))
    return;
  const client = await createServerClient();
  await client.post(`/api/v1/programmes/${programmeID}/intakes`, {
    name: text(data, "name"),
    starts_at: starts.toISOString(),
    application_opens_at: opens.toISOString(),
    application_closes_at: closes.toISOString(),
    capacity: capacityValue ? Number(capacityValue) : null,
  });
  await refreshCatalogue();
}

async function setIntakeStatus(data: FormData) {
  "use server";
  const id = text(data, "id");
  const status = text(data, "status") as IntakeStatus;
  if (!id) return;
  const client = await createServerClient();
  await client.patch(`/api/v1/intakes/${id}`, { status });
  await refreshCatalogue();
}

export default async function ProgrammeCataloguePage() {
  await requireAuth();
  let programmes: Programme[] = [];
  let error: string | null = null;
  try {
    const client = await createServerClient();
    programmes = (await client.get<{ data: Programme[] }>("/api/v1/programmes?limit=100")).data;
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load the programme catalogue";
  }
  const published = programmes.filter((item) => item.status === "published").length;
  const openIntakes = programmes
    .flatMap((item) => item.intakes)
    .filter((item) => item.status === "open").length;

  return (
    <div className="space-y-7">
      <PageHeader
        icon={<BookOpen className="size-7" />}
        title="Programme catalogue"
        description="Publish verified programme information and control exactly when each intake accepts applications."
      />
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Programmes" value={programmes.length} />
        <StatCard label="Published" value={published} />
        <StatCard label="Open intakes" value={openIntakes} />
      </div>
      <section className="rounded-xl border border-border bg-surface p-6">
        <h2 className="font-heading text-xl font-bold">Add a programme</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          New programmes begin as drafts and never appear publicly until a staff member publishes
          them.
        </p>
        <form action={createProgramme} className="mt-5 grid gap-4 md:grid-cols-2">
          <label className="text-sm font-semibold">
            Programme code
            <input
              required
              minLength={2}
              maxLength={32}
              pattern="[A-Z0-9][A-Z0-9_-]+"
              name="code"
              placeholder="SCI"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal uppercase"
            />
          </label>
          <label className="text-sm font-semibold">
            Name
            <input
              required
              minLength={2}
              maxLength={160}
              name="name"
              placeholder="General Science"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            />
          </label>
          <label className="text-sm font-semibold md:col-span-2">
            URL slug
            <input
              required
              minLength={2}
              maxLength={100}
              pattern="[a-z0-9]+(-[a-z0-9]+)*"
              name="slug"
              placeholder="general-science"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            />
          </label>
          <label className="text-sm font-semibold md:col-span-2">
            Short summary
            <textarea
              required
              minLength={2}
              maxLength={500}
              name="summary"
              rows={2}
              className="mt-2 w-full rounded-md border border-border bg-background p-3 font-normal"
            />
          </label>
          <label className="text-sm font-semibold md:col-span-2">
            Full description
            <textarea
              required
              minLength={2}
              maxLength={10000}
              name="description"
              rows={5}
              className="mt-2 w-full rounded-md border border-border bg-background p-3 font-normal"
            />
          </label>
          <div className="md:col-span-2">
            <Button type="submit">Create draft programme</Button>
          </div>
        </form>
      </section>
      {error ? (
        <EmptyState
          icon={<CircleOff className="size-8" />}
          title="Could not load the catalogue"
          description={error}
        />
      ) : programmes.length === 0 ? (
        <EmptyState
          icon={<BookOpen className="size-8" />}
          title="No programmes yet"
          description="Create the first verified programme, then add and open an application intake."
        />
      ) : (
        <section className="space-y-5">
          {programmes.map((programme) => (
            <article key={programme.id} className="rounded-xl border border-border bg-surface p-6">
              <div className="flex flex-col justify-between gap-5 lg:flex-row lg:items-start">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded-full bg-muted px-2.5 py-1 font-mono text-xs font-semibold">
                      {programme.code}
                    </span>
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-semibold ${programme.status === "published" ? "bg-emerald-50 text-emerald-800" : "bg-amber-50 text-amber-800"}`}
                    >
                      {programme.status}
                    </span>
                  </div>
                  <h2 className="mt-3 font-heading text-2xl font-bold">{programme.name}</h2>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">
                    {programme.summary}
                  </p>
                </div>
                <form action={setProgrammeStatus}>
                  <input type="hidden" name="id" value={programme.id} />
                  <input
                    type="hidden"
                    name="status"
                    value={programme.status === "published" ? "draft" : "published"}
                  />
                  <Button
                    type="submit"
                    variant={programme.status === "published" ? "secondary" : "primary"}
                  >
                    {programme.status === "published" ? "Unpublish" : "Publish programme"}
                  </Button>
                </form>
              </div>
              <div className="mt-6 grid gap-4 lg:grid-cols-2">
                {programme.intakes.map((intake) => (
                  <div
                    key={intake.id}
                    className="rounded-lg border border-border bg-background p-4"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <h3 className="font-semibold">{intake.name}</h3>
                        <p className="mt-1 text-xs text-muted-foreground">
                          Starts{" "}
                          {new Date(intake.starts_at).toLocaleDateString("en-GB", {
                            dateStyle: "medium",
                          })}
                        </p>
                      </div>
                      <span
                        className={`rounded-full px-2 py-1 text-xs font-semibold ${intake.status === "open" ? "bg-emerald-50 text-emerald-800" : "bg-muted text-muted-foreground"}`}
                      >
                        {intake.status}
                      </span>
                    </div>
                    <p className="mt-3 text-xs text-muted-foreground">
                      Application window:{" "}
                      {new Date(intake.application_opens_at).toLocaleDateString("en-GB")} –{" "}
                      {new Date(intake.application_closes_at).toLocaleDateString("en-GB")}
                    </p>
                    <form action={setIntakeStatus} className="mt-4">
                      <input type="hidden" name="id" value={intake.id} />
                      <input
                        type="hidden"
                        name="status"
                        value={intake.status === "open" ? "closed" : "open"}
                      />
                      <Button
                        type="submit"
                        className="h-9 px-3 text-xs"
                        variant="secondary"
                        disabled={programme.status !== "published" && intake.status !== "open"}
                      >
                        {intake.status === "open" ? "Close applications" : "Open applications"}
                      </Button>
                    </form>
                  </div>
                ))}
                <form
                  action={createIntake}
                  className="rounded-lg border border-dashed border-border bg-muted/40 p-4"
                >
                  <input type="hidden" name="programme_id" value={programme.id} />
                  <h3 className="flex items-center gap-2 font-semibold">
                    <CalendarPlus className="size-4" /> Add an intake
                  </h3>
                  <div className="mt-4 grid gap-3 sm:grid-cols-2">
                    <label className="text-xs font-semibold sm:col-span-2">
                      Intake name
                      <input
                        required
                        minLength={2}
                        maxLength={120}
                        name="name"
                        placeholder="September 2026"
                        className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 font-normal"
                      />
                    </label>
                    <label className="text-xs font-semibold">
                      Applications open
                      <input
                        required
                        type="datetime-local"
                        name="application_opens_at"
                        className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 font-normal"
                      />
                    </label>
                    <label className="text-xs font-semibold">
                      Applications close
                      <input
                        required
                        type="datetime-local"
                        name="application_closes_at"
                        className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 font-normal"
                      />
                    </label>
                    <label className="text-xs font-semibold">
                      Programme starts
                      <input
                        required
                        type="datetime-local"
                        name="starts_at"
                        className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 font-normal"
                      />
                    </label>
                    <label className="text-xs font-semibold">
                      Capacity (optional)
                      <input
                        type="number"
                        min={1}
                        max={1000000}
                        name="capacity"
                        className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 font-normal"
                      />
                    </label>
                  </div>
                  <Button type="submit" className="mt-4 h-9 px-3 text-xs">
                    <CheckCircle2 className="mr-2 size-4" /> Save draft intake
                  </Button>
                </form>
              </div>
            </article>
          ))}
        </section>
      )}
    </div>
  );
}
