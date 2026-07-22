import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  BookOpenText,
  CheckCircle2,
  MessageCircleMore,
  ShieldCheck,
  Sparkles,
  type LucideIcon,
} from "lucide-react";
import { fetchPageBySlug } from "@/lib/website";
import { fetchTenantBranding } from "@/lib/tenant";
import { WebsiteSection } from "@/components/website-section";
import { Button, Reveal } from "@auraedu/ui";

interface PublicHomePageProps {
  params: Promise<{ tenant: string }>;
}

export async function generateMetadata({ params }: PublicHomePageProps): Promise<Metadata> {
  const { tenant: tenantCode } = await params;
  const [tenant, page] = await Promise.all([
    fetchTenantBranding(tenantCode),
    fetchPageBySlug(tenantCode, "home"),
  ]);

  return {
    title: page?.title ? `${page.title} — ${tenant.name}` : tenant.name,
    description: page?.meta_description ?? `Welcome to ${tenant.name}.`,
    robots: { index: true, follow: true },
  };
}

export default async function PublicHomePage({ params }: PublicHomePageProps) {
  const { tenant: tenantCode } = await params;
  const [tenant, page] = await Promise.all([
    fetchTenantBranding(tenantCode),
    fetchPageBySlug(tenantCode, "home"),
  ]);

  if (!page) {
    return (
      <WelcomePlaceholder
        tenantCode={tenantCode}
        schoolName={tenant.name}
        schoolShort={tenant.short}
      />
    );
  }

  return (
    <article>
      {page.sections && page.sections.length > 0 ? (
        page.sections.map((section) => <WebsiteSection key={section.id} section={section} />)
      ) : (
        <WelcomePlaceholder
          title={page.title}
          tenantCode={tenantCode}
          schoolName={tenant.name}
          schoolShort={tenant.short}
        />
      )}
    </article>
  );
}

const previewPaths: {
  icon: LucideIcon;
  title: string;
  description: string;
}[] = [
  {
    icon: BookOpenText,
    title: "Explore programmes",
    description: "See the programmes and intakes the admissions team has published.",
  },
  {
    icon: CheckCircle2,
    title: "Prepare with clarity",
    description: "Use verified entry guidance to understand what comes next.",
  },
  {
    icon: MessageCircleMore,
    title: "Ask with confidence",
    description: "The admissions assistant answers only from approved school sources.",
  },
];

function WelcomePlaceholder({
  title,
  tenantCode,
  schoolName,
  schoolShort,
}: {
  title?: string;
  tenantCode: string;
  schoolName: string;
  schoolShort: string;
}) {
  const programmesHref = `/${tenantCode}/programmes`;

  return (
    <article>
      <section className="school-site-hero relative isolate overflow-hidden text-white">
        <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[var(--color-signal)] via-[var(--color-teal-bright)] to-[var(--color-brand)]" />
        <div className="mx-auto grid min-h-[38rem] max-w-7xl gap-12 px-6 py-20 lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-center lg:py-24">
          <Reveal>
            <div>
              <p className="inline-flex items-center gap-2 rounded-full border border-white/14 bg-white/[0.07] px-3 py-2 font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--color-teal-bright)]">
                <Sparkles className="size-3.5" aria-hidden="true" /> {schoolShort} digital campus
              </p>
              <h1 className="mt-7 max-w-4xl text-balance font-heading text-5xl font-black leading-[0.98] tracking-[-0.04em] sm:text-6xl lg:text-7xl">
                {title ?? "Your next chapter starts here."}
              </h1>
              <p className="mt-6 max-w-2xl text-lg leading-8 text-white/72">
                {schoolName} is creating one clear place to discover programmes, prepare an
                application and get trustworthy admissions guidance.
              </p>
              <div className="mt-9 flex flex-wrap gap-3">
                <Button asChild variant="gold" pill className="h-12 px-6">
                  <Link href={programmesHref}>
                    Explore programmes <ArrowRight className="size-4" aria-hidden="true" />
                  </Link>
                </Button>
                <Button
                  asChild
                  variant="secondary"
                  pill
                  className="h-12 border-white/24 bg-white/[0.06] px-6 text-white hover:bg-white/12 hover:text-white"
                >
                  <Link href="/login">Open school portal</Link>
                </Button>
              </div>
            </div>
          </Reveal>

          <Reveal delay={120} variant="right">
            <aside className="overflow-hidden rounded-3xl border border-white/14 bg-white/[0.075] shadow-2xl backdrop-blur-sm">
              <div className="border-b border-white/12 px-6 py-5">
                <p className="font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--color-signal)]">
                  Start here
                </p>
                <p className="mt-2 text-xl font-extrabold">
                  A simple path from interest to action.
                </p>
              </div>
              <ol className="divide-y divide-white/10 px-6">
                {previewPaths.map((path, index) => (
                  <li key={path.title} className="flex gap-4 py-5">
                    <span className="font-mono text-xs font-black text-[var(--color-teal-bright)]">
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <div>
                      <p className="font-bold">{path.title}</p>
                      <p className="mt-1 text-sm leading-6 text-white/60">{path.description}</p>
                    </div>
                  </li>
                ))}
              </ol>
            </aside>
          </Reveal>
        </div>
      </section>

      <section className="py-20 lg:py-24">
        <div className="mx-auto max-w-7xl px-6">
          <Reveal>
            <div className="grid gap-8 border-t border-[var(--border)] pt-8 lg:grid-cols-[17rem_minmax(0,1fr)] lg:gap-16">
              <p className="font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--primary)]">
                The admissions journey
              </p>
              <div>
                <h2 className="max-w-3xl text-balance font-heading text-3xl font-black tracking-tight text-[var(--foreground)] lg:text-5xl">
                  Move forward without the guesswork.
                </h2>
                <p className="mt-5 max-w-2xl text-base leading-8 text-[var(--muted-foreground)]">
                  Details appear here only after the school publishes them. Until then, every
                  pathway remains honest about what is known and when a person should confirm it.
                </p>
              </div>
            </div>
          </Reveal>

          <div className="mt-12 grid gap-5 md:grid-cols-3">
            {previewPaths.map((path, index) => {
              const Icon = path.icon;
              return (
                <Reveal key={path.title} delay={index * 70}>
                  <article className="school-site-card h-full rounded-3xl border border-[var(--border)] bg-[var(--surface)] p-6">
                    <div className="flex items-center justify-between">
                      <span className="grid size-11 place-items-center rounded-2xl bg-[var(--accent)] text-[var(--primary)]">
                        <Icon className="size-5" aria-hidden="true" />
                      </span>
                      <span className="font-mono text-[10px] font-black tracking-[0.18em] text-[var(--muted-foreground)]/50">
                        {String(index + 1).padStart(2, "0")}
                      </span>
                    </div>
                    <h3 className="mt-8 text-xl font-extrabold text-[var(--foreground)]">
                      {path.title}
                    </h3>
                    <p className="mt-3 text-sm leading-7 text-[var(--muted-foreground)]">
                      {path.description}
                    </p>
                  </article>
                </Reveal>
              );
            })}
          </div>
        </div>
      </section>

      <section className="pb-20 lg:pb-24">
        <div className="mx-auto max-w-7xl px-6">
          <Reveal variant="scale">
            <div className="grid gap-8 rounded-3xl border border-[var(--border)] bg-[var(--surface)] p-7 shadow-xl lg:grid-cols-[auto_minmax(0,1fr)_auto] lg:items-center lg:p-9">
              <span className="grid size-12 place-items-center rounded-2xl bg-[var(--accent)] text-[var(--primary)]">
                <ShieldCheck className="size-6" aria-hidden="true" />
              </span>
              <div>
                <h2 className="text-xl font-extrabold text-[var(--foreground)]">
                  School-approved information, clearly labelled.
                </h2>
                <p className="mt-2 max-w-3xl text-sm leading-7 text-[var(--muted-foreground)]">
                  The admissions assistant cites approved sources and hands uncertain questions to
                  the school team instead of inventing an answer.
                </p>
              </div>
              <Link
                href={programmesHref}
                className="inline-flex h-11 items-center justify-center gap-2 rounded-full bg-[var(--primary)] px-5 text-sm font-extrabold text-[var(--primary-foreground)] transition-transform hover:-translate-y-0.5 motion-reduce:transition-none"
              >
                Browse programmes <ArrowRight className="size-4" aria-hidden="true" />
              </Link>
            </div>
          </Reveal>
        </div>
      </section>
    </article>
  );
}
