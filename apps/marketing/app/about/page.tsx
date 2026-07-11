import Link from "next/link";
import { Button } from "@auraedu/ui";

function Tick({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 12" className={className} aria-hidden="true">
      <path
        d="M1 6.5 5.2 10.5 15 1"
        fill="none"
        stroke="currentColor"
        strokeWidth={2.4}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function Eyebrow({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-2.5 font-mono text-xs uppercase tracking-[0.16em] text-muted-foreground">
      <Tick className="w-3.5 text-primary" />
      {children}
    </span>
  );
}

export const metadata = {
  title: "About AuraEDU",
  description: "AuraEDU is a multi-tenant school operating system built for Ghanaian schools.",
};

export default function AboutPage() {
  return (
    <div className="mx-auto max-w-3xl px-6 py-16">
      <Eyebrow>About us</Eyebrow>
      <h1 className="mt-4 text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl lg:text-5xl">
        Built for every school in Ghana.
      </h1>

      <div className="mt-8 space-y-4 text-[15px] leading-relaxed text-muted-foreground">
        <p>
          AuraEDU is the system of record for schools. We started with a simple idea: a single platform that
          admissions offices, teachers, bursars, parents and students can trust — while each school keeps its own
          brand, data and enabled features.
        </p>
        <p>
          Our identity is drawn from the Ghanaian classroom: the chalkboard, the attendance register and the red
          marking pen. That ritual — ticking a student present — is the signature of the whole product.
        </p>
        <p>
          Every school is a tenant. Data is isolated. Features are configurable. A new school is onboarded, not
          rebuilt.
        </p>
      </div>

      <div className="mt-10 grid gap-4 sm:grid-cols-3">
        {[
          { label: "Schools", value: "50+" },
          { label: "Students tracked", value: "20,000+" },
          { label: "Modules", value: "12" },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-border bg-surface p-5 text-center">
            <p className="font-display text-3xl font-extrabold text-primary">{s.value}</p>
            <p className="mt-1 text-sm text-muted-foreground">{s.label}</p>
          </div>
        ))}
      </div>

      <div className="mt-10">
        <Button asChild className="h-11 px-5">
          <Link href="/contact">Get in touch</Link>
        </Button>
      </div>
    </div>
  );
}
