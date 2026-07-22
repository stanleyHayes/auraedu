import Link from "next/link";
import { ArrowRight, CheckCircle2 } from "lucide-react";
import { Eyebrow } from "./brand-primitives";
import { ScrollReveal } from "./motion-primitives";

export interface TrustSection {
  title: string;
  copy: string;
  points?: readonly string[];
}

export function TrustPage({
  eyebrow,
  title,
  introduction,
  updated,
  sections,
}: {
  eyebrow: string;
  title: string;
  introduction: string;
  updated: string;
  sections: readonly TrustSection[];
}) {
  return (
    <div className="overflow-hidden bg-white">
      <section className="trust-stage text-white">
        <ScrollReveal className="mx-auto max-w-7xl px-6 py-24 sm:py-28">
          <Eyebrow inverse>{eyebrow}</Eyebrow>
          <h1 className="mt-5 max-w-[14ch] text-balance text-[clamp(3.2rem,6vw,6.5rem)] font-bold leading-[0.9] tracking-[-0.055em]">
            {title}
          </h1>
          <p className="mt-7 max-w-3xl text-lg leading-8 text-slate-300">{introduction}</p>
          <p className="mt-8 font-mono text-xs uppercase tracking-[0.14em] text-teal-bright">
            Last reviewed {updated}
          </p>
        </ScrollReveal>
      </section>

      <section className="bg-cool-mist">
        <div className="mx-auto grid max-w-7xl gap-5 px-6 py-20 lg:grid-cols-2">
          {sections.map((section, index) => (
            <ScrollReveal
              key={section.title}
              className="rounded-2xl border border-slate-200 bg-white p-7 shadow-sm sm:p-9"
            >
              <p className="font-mono text-xs font-bold tracking-[0.16em] text-cobalt">
                {String(index + 1).padStart(2, "0")}
              </p>
              <h2 className="mt-5 text-2xl font-bold tracking-[-0.035em] text-navy-deep sm:text-3xl">
                {section.title}
              </h2>
              <p className="mt-4 leading-7 text-slate-600">{section.copy}</p>
              {section.points ? (
                <ul className="mt-6 grid gap-3">
                  {section.points.map((point) => (
                    <li key={point} className="flex gap-3 text-sm leading-6 text-slate-700">
                      <CheckCircle2
                        className="mt-0.5 size-4 shrink-0 text-teal-strong"
                        aria-hidden="true"
                      />
                      {point}
                    </li>
                  ))}
                </ul>
              ) : null}
            </ScrollReveal>
          ))}
        </div>
      </section>

      <section className="bg-white">
        <div className="mx-auto flex max-w-7xl flex-col gap-6 px-6 py-20 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <Eyebrow>Ask a direct question</Eyebrow>
            <h2 className="mt-3 max-w-[22ch] text-4xl font-bold tracking-[-0.04em] text-navy-deep">
              Trust should be inspectable, not implied.
            </h2>
          </div>
          <Link href="/contact" className="cta-solid">
            Contact the AuraEDU team <ArrowRight className="size-4" aria-hidden="true" />
          </Link>
        </div>
      </section>
    </div>
  );
}
