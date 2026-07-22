import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, BookOpen, CalendarDays } from "lucide-react";
import { EmptyState } from "@auraedu/ui";
import { fetchTenantBranding } from "@/lib/tenant";
import { fetchPublicProgrammes } from "@/lib/programmes";

interface PageProps {
  params: Promise<{ tenant: string }>;
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { tenant } = await params;
  const school = await fetchTenantBranding(tenant);
  return {
    title: `Programmes — ${school.name}`,
    description: `Explore programmes with applications currently open at ${school.name}.`,
  };
}

export default async function ProgrammesPage({ params }: PageProps) {
  const { tenant } = await params;
  const [school, programmes] = await Promise.all([
    fetchTenantBranding(tenant),
    fetchPublicProgrammes(tenant),
  ]);

  return (
    <div className="mx-auto max-w-6xl px-6 py-16 lg:py-24">
      <div className="max-w-3xl">
        <p className="text-xs font-bold uppercase tracking-[0.18em] text-[var(--primary)]">
          Verified programme catalogue
        </p>
        <h1 className="mt-4 font-heading text-4xl font-extrabold tracking-tight lg:text-6xl">
          Find your next programme at {school.name}.
        </h1>
        <p className="mt-6 text-lg leading-8 text-muted-foreground">
          Every option below comes from the school&apos;s admissions catalogue and shows only an
          intake accepting applications now.
        </p>
      </div>
      {programmes.length === 0 ? (
        <div className="mt-12">
          <EmptyState
            icon={<BookOpen className="size-8" />}
            title="No applications are open right now"
            description="The admissions team has not published an intake that is currently accepting applications."
          />
        </div>
      ) : (
        <div className="mt-12 grid gap-6 lg:grid-cols-2">
          {programmes.map((programme) => {
            const intake = programme.intakes[0];
            return (
              <article
                key={programme.id}
                className="flex flex-col rounded-2xl border border-border bg-surface p-7 shadow-sm"
              >
                <div className="flex items-start justify-between gap-4">
                  <span className="rounded-full bg-[var(--muted)] px-3 py-1 font-mono text-xs font-semibold">
                    {programme.code}
                  </span>
                  <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-700">
                    <span className="size-2 rounded-full bg-emerald-500" /> Applications open
                  </span>
                </div>
                <h2 className="mt-6 font-heading text-2xl font-bold">{programme.name}</h2>
                <p className="mt-3 flex-1 text-sm leading-7 text-muted-foreground">
                  {programme.summary}
                </p>
                {intake ? (
                  <div className="mt-6 rounded-xl bg-muted p-4">
                    <p className="flex items-center gap-2 text-sm font-semibold">
                      <CalendarDays className="size-4 text-primary" /> {intake.name}
                    </p>
                    <p className="mt-2 text-xs text-muted-foreground">
                      Applications close{" "}
                      {new Date(intake.application_closes_at).toLocaleDateString("en-GB", {
                        dateStyle: "long",
                      })}
                    </p>
                  </div>
                ) : null}
                <div className="mt-6 flex flex-wrap gap-3">
                  <Link
                    href={`/${tenant}/programmes/${programme.slug}`}
                    className="inline-flex min-h-11 items-center rounded-md border border-border px-4 text-sm font-semibold hover:bg-muted"
                  >
                    Programme details
                  </Link>
                  {intake ? (
                    <Link
                      href={`/applicant?programme=${programme.id}&intake=${intake.id}`}
                      className="inline-flex min-h-11 items-center gap-2 rounded-md bg-primary px-4 text-sm font-semibold text-primary-foreground"
                    >
                      Apply now <ArrowRight className="size-4" />
                    </Link>
                  ) : null}
                </div>
              </article>
            );
          })}
        </div>
      )}
    </div>
  );
}
