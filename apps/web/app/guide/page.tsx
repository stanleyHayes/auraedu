import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft, BookOpenText, CheckCircle2, Headphones } from "lucide-react";
import { AuraEduLogo } from "@/components/auraedu-logo";
import { PAGE_GUIDES } from "@/lib/page-guides";

export const metadata: Metadata = {
  title: "User guide — AuraEDU",
  description: "Contextual walkthroughs for the AuraEDU education operating system.",
};

export default function GuidePage() {
  const sections = PAGE_GUIDES.reduce((grouped, guide) => {
    const current = grouped.get(guide.section) ?? [];
    current.push(guide);
    grouped.set(guide.section, current);
    return grouped;
  }, new Map<string, typeof PAGE_GUIDES>());

  return (
    <main id="guide-main" className="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
      <div className="relative overflow-hidden border-b border-[var(--border)] bg-[var(--color-navy)] text-white">
        <span
          aria-hidden
          className="absolute -left-28 -top-36 size-96 rounded-full bg-[var(--color-brand)]/35 blur-3xl"
        />
        <span
          aria-hidden
          className="absolute -right-20 bottom-0 size-72 rounded-full bg-[var(--color-teal-bright)]/15 blur-3xl"
        />
        <div className="relative mx-auto max-w-6xl px-5 py-8 sm:px-8 sm:py-12">
          <div className="flex items-center justify-between gap-4">
            <AuraEduLogo tone="light" className="h-7" />
            <Link
              href="/"
              className="inline-flex h-10 items-center gap-2 rounded-full border border-white/15 bg-white/[0.07] px-4 text-sm font-bold text-white transition hover:bg-white/12"
            >
              <ArrowLeft className="size-4" aria-hidden="true" /> Return to workspace
            </Link>
          </div>
          <div className="mt-14 max-w-3xl">
            <p className="font-mono text-[11px] font-black uppercase tracking-[0.2em] text-[var(--color-teal-bright)]">
              AuraEDU user guide
            </p>
            <h1 className="mt-3 text-balance font-heading text-4xl font-black tracking-[-0.035em] sm:text-6xl">
              A calm guide to every workspace.
            </h1>
            <p className="mt-5 max-w-2xl text-base leading-7 text-white/70 sm:text-lg">
              The same verified steps power each page’s help panel, accessible transcript and
              British-English narration. Use this library when you want the complete picture.
            </p>
          </div>
          <div className="mt-9 grid max-w-3xl gap-3 sm:grid-cols-2">
            <div className="flex gap-3 rounded-2xl border border-white/10 bg-white/[0.06] p-4">
              <BookOpenText
                className="mt-0.5 size-5 text-[var(--color-signal)]"
                aria-hidden="true"
              />
              <div>
                <b className="text-sm">Visible walkthroughs</b>
                <p className="mt-1 text-xs leading-5 text-white/60">
                  Numbered instructions remain readable without JavaScript or audio.
                </p>
              </div>
            </div>
            <div className="flex gap-3 rounded-2xl border border-white/10 bg-white/[0.06] p-4">
              <Headphones className="mt-0.5 size-5 text-[var(--color-signal)]" aria-hidden="true" />
              <div>
                <b className="text-sm">Optional narration</b>
                <p className="mt-1 text-xs leading-5 text-white/60">
                  Use Listen in a page header; Stop or leave the page to end speech.
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-6xl space-y-12 px-5 py-12 sm:px-8 sm:py-16">
        {[...sections.entries()].map(([section, guides]) => (
          <section key={section} aria-labelledby={`guide-${slug(section)}`}>
            <div className="mb-5 flex items-center gap-3">
              <span className="h-px w-8 bg-[var(--primary)]" />
              <h2 id={`guide-${slug(section)}`} className="font-heading text-2xl font-extrabold">
                {section}
              </h2>
            </div>
            <div className="grid gap-4 lg:grid-cols-2">
              {guides.map((guide) => (
                <article
                  key={guide.key}
                  className="group rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-sm transition duration-200 hover:-translate-y-0.5 hover:border-[var(--primary)]/30 hover:shadow-lg motion-reduce:transform-none"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="font-mono text-[10px] font-black uppercase tracking-[0.16em] text-[var(--primary)]">
                        {guide.href}
                      </p>
                      <h3 className="mt-1 font-heading text-xl font-extrabold">{guide.title}</h3>
                    </div>
                    <span className="grid size-9 shrink-0 place-items-center rounded-xl bg-[var(--accent)] text-[var(--primary)]">
                      <CheckCircle2 className="size-[18px]" aria-hidden="true" />
                    </span>
                  </div>
                  <p className="mt-3 text-sm leading-6 text-[var(--muted-foreground)]">
                    {guide.description}
                  </p>
                  <ol className="mt-5 space-y-3 border-t border-[var(--border)] pt-5">
                    {guide.steps.map((step, index) => (
                      <li
                        key={step}
                        className="grid grid-cols-[1.5rem_1fr] gap-3 text-sm leading-6"
                      >
                        <span className="font-mono text-xs font-black text-[var(--primary)]">
                          {String(index + 1).padStart(2, "0")}
                        </span>
                        <span>{step}</span>
                      </li>
                    ))}
                  </ol>
                </article>
              ))}
            </div>
          </section>
        ))}
      </div>
    </main>
  );
}

function slug(value: string) {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}
