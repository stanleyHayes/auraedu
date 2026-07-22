import { notFound } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, ArrowRight, CalendarDays, CheckCircle2 } from "lucide-react";
import { fetchPublicProgrammes } from "@/lib/programmes";

export default async function ProgrammePage({
  params,
}: {
  params: Promise<{ tenant: string; slug: string }>;
}) {
  const { tenant, slug } = await params;
  const programme = (await fetchPublicProgrammes(tenant)).find((item) => item.slug === slug);
  if (!programme) notFound();
  const intake = programme.intakes[0];
  return (
    <article className="mx-auto max-w-5xl px-6 py-16 lg:py-24">
      <Link
        href={`/${tenant}/programmes`}
        className="inline-flex items-center gap-2 text-sm font-semibold text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" /> All programmes
      </Link>
      <div className="mt-8 grid gap-10 lg:grid-cols-[1fr_20rem]">
        <div>
          <p className="font-mono text-xs font-bold uppercase tracking-[0.16em] text-primary">
            {programme.code}
          </p>
          <h1 className="mt-4 font-heading text-4xl font-extrabold tracking-tight lg:text-6xl">
            {programme.name}
          </h1>
          <p className="mt-6 text-xl leading-8 text-muted-foreground">{programme.summary}</p>
          <div className="mt-10 whitespace-pre-wrap text-base leading-8 text-foreground/85">
            {programme.description}
          </div>
        </div>
        <aside className="h-fit rounded-2xl border border-border bg-surface p-6 shadow-sm">
          <p className="flex items-center gap-2 text-sm font-bold text-emerald-700">
            <CheckCircle2 className="size-4" /> Applications open
          </p>
          {intake ? (
            <>
              <h2 className="mt-5 font-heading text-xl font-bold">{intake.name}</h2>
              <dl className="mt-4 space-y-4 text-sm">
                <div>
                  <dt className="text-muted-foreground">Programme begins</dt>
                  <dd className="mt-1 font-semibold">
                    {new Date(intake.starts_at).toLocaleDateString("en-GB", { dateStyle: "long" })}
                  </dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Applications close</dt>
                  <dd className="mt-1 font-semibold">
                    {new Date(intake.application_closes_at).toLocaleDateString("en-GB", {
                      dateStyle: "long",
                    })}
                  </dd>
                </div>
              </dl>
              <Link
                href={`/applicant?programme=${programme.id}&intake=${intake.id}`}
                className="mt-6 inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-md bg-primary px-4 text-sm font-semibold text-primary-foreground"
              >
                Start application <ArrowRight className="size-4" />
              </Link>
            </>
          ) : (
            <p className="mt-4 text-sm text-muted-foreground">
              <CalendarDays className="mr-2 inline size-4" />
              No intake is currently open.
            </p>
          )}
        </aside>
      </div>
    </article>
  );
}
