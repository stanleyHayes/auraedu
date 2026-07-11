import Link from "next/link";
import { Button, RegisterCard, type Pupil } from "@auraedu/ui";

const roster: Pupil[] = [
  { id: "1", name: "Ama Owusu", present: true },
  { id: "2", name: "Kwame Mensah", present: true },
  { id: "3", name: "Efua Sarpong", present: true },
  { id: "4", name: "Yaw Boateng", present: true },
  { id: "5", name: "Adjoa Nyarko", present: true },
  { id: "6", name: "Kojo Amaning", present: false },
  { id: "7", name: "Abena Darko", present: true },
  { id: "8", name: "Kofi Asante", present: true },
];

const features = [
  { key: "students", name: "Student information", desc: "Profiles, guardians, enrolments and documents in one record." },
  { key: "academics", name: "Academics", desc: "Classes, subjects, timetables and grading schemes per school." },
  { key: "attendance", name: "Attendance", desc: "Daily and per-subject registers with instant presence counts." },
  { key: "assessments", name: "Assessments", desc: "Tests, exams, scores and report cards, term after term." },
  { key: "fees", name: "Fees & communication", desc: "Invoices, receipts, announcements and parent messaging." },
  { key: "analytics", name: "Analytics", desc: "Attendance, academic and financial dashboards by role." },
];

const steps = [
  { n: "01", title: "Create your school", desc: "Enter the school name and admin details; we generate your tenant code." },
  { n: "02", title: "Pick a plan", desc: "Start with a free trial, then choose the plan that matches your modules." },
  { n: "03", title: "Switch on features", desc: "Enable attendance, assessments, fees and more — only what you need." },
  { n: "04", title: "Go live", desc: "Import students and staff, set your brand, and start the term." },
];

const testimonials = [
  { quote: "AuraEDU replaced three separate tools. Our register, results and fees now live in one place.", school: "University Practice SHS", role: "Administrator" },
  { quote: "Parents finally see report cards and invoices on the same day they are published.", school: "Aboom AME Zion C Basic", role: "Head Teacher" },
  { quote: "Setting up a new term took hours, not weeks.", school: "Cape Coast Prep", role: "Academic Officer" },
];

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

export default function HomePage() {
  return (
    <div className="bg-background text-foreground">
      {/* HERO */}
      <section className="mx-auto grid max-w-6xl items-center gap-14 px-6 py-16 lg:grid-cols-[1.04fr_0.96fr] lg:py-20">
        <div>
          <Eyebrow>School operating system · Ghana</Eyebrow>
          <h1 className="mt-5 text-balance font-display text-4xl font-extrabold leading-[1.03] tracking-tight sm:text-5xl lg:text-[4rem]">
            Every student accounted for.{" "}
            <span className="text-primary [box-shadow:inset_0_-0.09em_0_var(--color-brand-tint)]">Every school</span>, one platform.
          </h1>
          <p className="mt-6 max-w-[36ch] text-lg leading-relaxed text-muted-foreground">
            AuraEDU runs the whole school — admissions, registers, results, report cards and fees — for many schools at once. Each keeps its own brand, data, and features.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Button asChild className="h-11 px-5">
              <Link href="/signup">Sign up your school</Link>
            </Button>
            <Button variant="secondary" asChild className="h-11 px-5">
              <Link href="/#features">See features</Link>
            </Button>
          </div>
          <div className="mt-7 flex items-center gap-3 font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
            <span>UPSHS</span>
            <span className="size-1 rounded-full bg-current" aria-hidden="true" />
            <span>Aboom AME Zion C</span>
            <span className="size-1 rounded-full bg-current" aria-hidden="true" />
            <span>+ new schools weekly</span>
          </div>
        </div>

        <RegisterCard
          title="Attendance · Form 2 Science"
          meta="Mon 10 Jul"
          pupils={roster}
          total={36}
          className="shadow-[0_18px_40px_-28px_rgba(22,36,29,0.3)]"
        />
      </section>

      {/* FEATURES */}
      <section id="features" className="border-t border-border">
        <div className="mx-auto max-w-6xl px-6 py-16">
          <div className="flex flex-col gap-3.5">
            <Eyebrow>Core modules</Eyebrow>
            <h2 className="text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl">
              Everything a school runs on, module by module.
            </h2>
            <p className="max-w-[58ch] text-muted-foreground">
              Start with the essentials and switch on advanced features when you need them. Each module keeps its own data and permissions.
            </p>
          </div>
          <div className="mt-9 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((f) => (
              <div key={f.key} className="rounded-lg border border-border bg-surface p-5 transition-colors hover:border-[var(--color-brand)]/40">
                <div className="flex items-center gap-2">
                  <Tick className="w-3.5 text-primary" />
                  <h3 className="text-base font-bold">{f.name}</h3>
                </div>
                <p className="mt-2 text-[13px] leading-relaxed text-muted-foreground">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* HOW IT WORKS */}
      <section id="how-it-works" className="border-t border-border">
        <div className="mx-auto max-w-6xl px-6 py-16">
          <div className="flex flex-col gap-3.5">
            <Eyebrow>How it works</Eyebrow>
            <h2 className="text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl">From sign-up to first register in a day.</h2>
          </div>
          <ol className="mt-9 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {steps.map((s) => (
              <li key={s.n} className="relative rounded-lg border border-border bg-surface p-5">
                <span className="font-mono text-xs uppercase tracking-[0.14em] text-primary">{s.n}</span>
                <h3 className="mt-2 text-base font-bold">{s.title}</h3>
                <p className="mt-1.5 text-[13px] leading-relaxed text-muted-foreground">{s.desc}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* TESTIMONIALS */}
      <section id="testimonials" className="border-t border-border">
        <div className="mx-auto max-w-6xl px-6 py-16">
          <div className="flex flex-col gap-3.5">
            <Eyebrow>Schools using AuraEDU</Eyebrow>
            <h2 className="text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl">Trusted by schools across Ghana.</h2>
          </div>
          <div className="mt-9 grid gap-4 md:grid-cols-3">
            {testimonials.map((t, i) => (
              <blockquote key={i} className="rounded-lg border border-border bg-surface p-5">
                <p className="text-[15px] leading-relaxed text-foreground">“{t.quote}”</p>
                <footer className="mt-4">
                  <p className="text-sm font-semibold">{t.school}</p>
                  <p className="text-xs text-muted-foreground">{t.role}</p>
                </footer>
              </blockquote>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section id="join" className="px-6 pb-4 pt-16">
        <div className="mx-auto max-w-6xl rounded-xl bg-ink-950 p-10 text-paper-50 sm:p-14">
          <span className="inline-flex items-center gap-2.5 font-mono text-xs uppercase tracking-[0.16em] text-ink-200">
            <Tick className="w-3.5 text-primary" />
            Onboard in a day
          </span>
          <h2 className="mt-3.5 max-w-[18ch] text-balance font-display text-3xl font-extrabold tracking-tight sm:text-[2.75rem]">
            Bring your school onto AuraEDU.
          </h2>
          <p className="mt-4 max-w-[46ch] text-ink-200">
            Create the school, choose a plan, switch on the features you need, upload your logo and colours, import students and staff — then go live.
          </p>
          <div className="mt-7 flex flex-wrap gap-3">
            <Button asChild className="h-11 px-5">
              <Link href="/signup">Sign up your school</Link>
            </Button>
            <Button variant="secondary" asChild className="h-11 border-ink-800 bg-transparent px-5 text-paper-50 hover:bg-ink-900">
              <Link href="/pricing">See pricing</Link>
            </Button>
          </div>
        </div>
      </section>
    </div>
  );
}
