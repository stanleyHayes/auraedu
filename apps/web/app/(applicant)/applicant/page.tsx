import { revalidatePath } from "next/cache";
import { ArrowRight, CheckCircle2, ClipboardList, FileText, Send } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { ApplicationDocumentUploader } from "@/components/application-document-uploader";
import { createServerClient, getCurrentTenantCode } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { fetchPublicProgrammes, findCatalogueSelection } from "@/lib/programmes";
type Application = OpenAPI.admissions_v1.components["schemas"]["Application"];
function text(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value.trim() : "";
}
async function start(data: FormData) {
  "use server";
  const programme = text(data, "programme_id"),
    intake = text(data, "intake_id"),
    lead = text(data, "lead_id");
  if (!programme || !intake) return;
  const client = await createServerClient();
  await client.post("/api/v1/applications", {
    programme_id: programme,
    intake_id: intake,
    lead_id: lead || null,
  });
  revalidatePath("/applicant");
}
async function update(data: FormData) {
  "use server";
  const id = text(data, "id");
  if (!id) return;
  const client = await createServerClient();
  await client.patch(`/api/v1/applications/${id}`, {
    legal_name: text(data, "legal_name"),
    email: text(data, "email"),
    phone: text(data, "phone"),
  });
  revalidatePath("/applicant");
}
async function submit(data: FormData) {
  "use server";
  const id = text(data, "id");
  if (!id) return;
  const client = await createServerClient();
  await client.post(`/api/v1/applications/${id}/submit`, {});
  revalidatePath("/applicant");
}
async function accept(data: FormData) {
  "use server";
  const id = text(data, "id");
  if (!id) return;
  const client = await createServerClient();
  await client.post(`/api/v1/applications/${id}/offer/accept`, {});
  revalidatePath("/applicant");
}
export default async function ApplicantPage({
  searchParams,
}: {
  searchParams: Promise<{ programme?: string; intake?: string; lead?: string }>;
}) {
  await requireAuth();
  const query = await searchParams;
  const tenantCode = await getCurrentTenantCode();
  const programmes = await fetchPublicProgrammes(tenantCode);
  let items: Application[] = [];
  let error: string | null = null;
  try {
    const client = await createServerClient();
    items = (await client.get<{ data: Application[] }>("/api/v1/applications?limit=25")).data;
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load your application";
  }
  const active = items.find((item) => !["rejected", "withdrawn"].includes(item.status)) ?? items[0];
  const selected = findCatalogueSelection(programmes, query.programme, query.intake);
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ClipboardList className="size-7" />}
        title="My application"
        description="Save your progress, attach required evidence, submit when complete, and track every decision."
      />
      {!active && selected ? (
        <section className="rounded-xl border border-border bg-surface p-6">
          <p className="font-mono text-xs font-semibold uppercase tracking-[0.14em] text-primary">
            {selected.programme.code}
          </p>
          <h2 className="mt-2 font-heading text-xl font-bold">Start {selected.programme.name}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {selected.intake.name} is accepting applications until{" "}
            {new Date(selected.intake.application_closes_at).toLocaleDateString("en-GB", {
              dateStyle: "long",
            })}
            . You can save and return before submitting.
          </p>
          <form action={start} className="mt-4">
            <input type="hidden" name="programme_id" value={selected.programme.id} />
            <input type="hidden" name="intake_id" value={selected.intake.id} />
            <input type="hidden" name="lead_id" value={query.lead ?? ""} />
            <Button type="submit">Start application</Button>
          </form>
        </section>
      ) : !active && query.programme && query.intake ? (
        <EmptyState
          title="This intake is not available"
          description="It may have closed, been unpublished, or belong to another institution. Choose a currently open intake below."
          icon={<ClipboardList className="size-8" />}
        />
      ) : null}
      {error ? (
        <EmptyState
          title="Could not load your application"
          description={error}
          icon={<ClipboardList className="size-8" />}
        />
      ) : !active ? (
        programmes.length === 0 ? (
          <EmptyState
            title="No applications are open"
            description="Your institution has no published intake accepting applications right now."
            icon={<ClipboardList className="size-8" />}
          />
        ) : (
          <section>
            <h2 className="font-heading text-xl font-bold">Choose an open programme</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              These options come directly from your institution&apos;s verified admissions
              catalogue.
            </p>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              {programmes.map((programme) =>
                programme.intakes.map((intake) => (
                  <article
                    key={intake.id}
                    className="rounded-xl border border-border bg-surface p-5"
                  >
                    <p className="font-mono text-xs font-semibold text-primary">{programme.code}</p>
                    <h3 className="mt-2 font-heading text-lg font-bold">{programme.name}</h3>
                    <p className="mt-2 text-sm text-muted-foreground">
                      {intake.name} · closes{" "}
                      {new Date(intake.application_closes_at).toLocaleDateString("en-GB")}
                    </p>
                    <form action={start} className="mt-4">
                      <input type="hidden" name="programme_id" value={programme.id} />
                      <input type="hidden" name="intake_id" value={intake.id} />
                      <Button type="submit" variant="secondary">
                        Start application <ArrowRight className="size-4" />
                      </Button>
                    </form>
                  </article>
                )),
              )}
            </div>
          </section>
        )
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-3">
            <StatCard label="Completion" value={`${active.completion_percentage}%`} />
            <StatCard label="Application status" value={active.status.replaceAll("_", " ")} />
            <StatCard label="Offer" value={active.offer_status} />
          </div>
          <section className="rounded-xl border border-border bg-surface p-6">
            <div className="mb-5 flex items-start gap-3">
              <FileText className="mt-0.5 size-5 text-primary" />
              <div>
                <h2 className="font-heading text-xl font-bold">Application details</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {active.programme_name} · {active.intake_name}
                </p>
              </div>
            </div>
            {active.status === "draft" ? (
              <>
                <form action={update} className="grid gap-4 md:grid-cols-2">
                  <input type="hidden" name="id" value={active.id} />
                  <label className="text-sm font-semibold md:col-span-2">
                    Legal name
                    <input
                      required
                      minLength={2}
                      maxLength={200}
                      name="legal_name"
                      defaultValue={active.legal_name}
                      className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
                    />
                  </label>
                  <label className="text-sm font-semibold">
                    Email
                    <input
                      required
                      type="email"
                      name="email"
                      defaultValue={active.email}
                      className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
                    />
                  </label>
                  <label className="text-sm font-semibold">
                    Phone
                    <input
                      required
                      minLength={7}
                      maxLength={30}
                      name="phone"
                      defaultValue={active.phone}
                      className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
                    />
                  </label>
                  <div className="md:col-span-2">
                    <Button type="submit" variant="secondary">
                      Save progress
                    </Button>
                  </div>
                </form>
                <div className="mt-5">
                  <ApplicationDocumentUploader applicationId={active.id} />
                </div>
                <div className="mt-5 rounded-lg bg-muted p-4">
                  <h3 className="text-sm font-semibold">Checklist</h3>
                  {active.missing_requirements.length ? (
                    <ul className="mt-2 list-inside list-disc text-sm text-muted-foreground">
                      {active.missing_requirements.map((requirement) => (
                        <li key={requirement}>{requirement.replaceAll("_", " ")}</li>
                      ))}
                    </ul>
                  ) : (
                    <p className="mt-2 flex items-center gap-2 text-sm text-emerald-700">
                      <CheckCircle2 className="size-4" />
                      All required items are present.
                    </p>
                  )}
                </div>
                <form action={submit} className="mt-5">
                  <input type="hidden" name="id" value={active.id} />
                  <Button type="submit" disabled={active.missing_requirements.length > 0}>
                    <Send className="mr-2 size-4" />
                    Submit final application
                  </Button>
                </form>
              </>
            ) : (
              <div className="space-y-3">
                <p className="text-sm leading-6 text-muted-foreground">
                  Your application was submitted on{" "}
                  {active.submitted_at
                    ? new Date(active.submitted_at).toLocaleString("en-GB")
                    : "the recorded date"}
                  . Changes are locked to preserve the reviewed record.
                </p>
                {active.review_note ? (
                  <p className="rounded-lg bg-muted p-4 text-sm">
                    <strong>Decision note:</strong> {active.review_note}
                  </p>
                ) : null}
                {active.offer_status === "issued" ? (
                  <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-5 text-emerald-950">
                    <h3 className="font-heading text-lg font-bold">Admission offer</h3>
                    <p className="mt-2 text-sm leading-6">{active.offer_conditions}</p>
                    <p className="mt-2 text-xs">
                      Accept by{" "}
                      {active.offer_expires_at
                        ? new Date(active.offer_expires_at).toLocaleString("en-GB")
                        : "the stated deadline"}
                    </p>
                    <form action={accept} className="mt-4">
                      <input type="hidden" name="id" value={active.id} />
                      <Button type="submit">Accept offer</Button>
                    </form>
                  </div>
                ) : active.offer_status === "accepted" ? (
                  <p className="flex items-center gap-2 rounded-lg bg-emerald-50 p-4 text-sm font-semibold text-emerald-800">
                    <CheckCircle2 className="size-5" />
                    Offer accepted. Your institution will provide enrolment next steps.
                  </p>
                ) : null}
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}
