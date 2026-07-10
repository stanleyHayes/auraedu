import { Button, RegisterCard, type Pupil } from "@auraedu/ui";
import { SiteHeader } from "@/components/site-header";

/* ---- content (real, not lorem) ---- */
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

const schools = [
  { crest: "UP", name: "University Practice SHS", code: "upshs · Senior High", color: "#7B1113", chips: ["attendance", "assessments", "fees", "ai_recommendations"], off: "library", note: "AI recommendations enabled" },
  { crest: "AZ", name: "Aboom AME Zion C Basic", code: "aboom-ame-zion-c · Basic", color: "#1E7D52", chips: ["attendance", "report_cards", "announcements"], off: "cbt_exams", note: "Report cards enabled" },
  { crest: "CC", name: "Cape Coast Prep", code: "cape-coast-prep · Basic", color: "#2456A6", chips: ["admissions", "fees", "online_payments"], off: "hostel", note: "Online payments enabled" },
];

const modules = [
  { key: "admissions", name: "Admissions", desc: "Applications, offers and enrolment", on: true },
  { key: "attendance", name: "Attendance", desc: "Daily and per-subject registers", on: true },
  { key: "assessments", name: "Assessments", desc: "Tests, exams and recorded scores", on: true },
  { key: "report_cards", name: "Report cards", desc: "Termly reports and transcripts, as PDF", on: true },
  { key: "fees", name: "Fees & payments", desc: "Invoices, balances and receipts", on: true },
  { key: "ai_recommendations", name: "AI recommendations", desc: "Learning suggestions a teacher approves first", on: true },
  { key: "library", name: "Library", desc: "Catalogue and lending", on: false },
  { key: "hostel", name: "Hostel", desc: "Boarding house and room allocation", on: false },
];

const roles = [
  { name: "School admin", desc: "Onboarding, staff, academic structure, fees and settings.", plat: "Web" },
  { name: "Teacher", desc: "Registers, scores, assignments and approving AI suggestions.", plat: "Web + Mobile" },
  { name: "Parent", desc: "Attendance, results, report cards, invoices and payments.", plat: "Web + Mobile" },
  { name: "Student", desc: "Timetable, assignments, results and recommendations.", plat: "Web + Mobile" },
];

function Tick({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 12" className={className} aria-hidden="true">
      <path d="M1 6.5 5.2 10.5 15 1" fill="none" stroke="currentColor" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" />
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

export default function Page() {
  return (
    <div id="top" className="min-h-screen bg-background text-foreground">
      <SiteHeader />

      <main>
        {/* HERO */}
        <section className="mx-auto grid max-w-6xl items-center gap-14 px-6 py-16 lg:grid-cols-[1.04fr_0.96fr] lg:py-20">
          <div>
            <Eyebrow>School operating system · Ghana</Eyebrow>
            <h1 className="mt-5 text-balance font-display text-4xl font-extrabold leading-[1.03] tracking-tight sm:text-5xl lg:text-[4rem]">
              Every student accounted for.{" "}
              <span className="text-primary [box-shadow:inset_0_-0.09em_0_var(--color-brand-tint)]">Every school</span>, one platform.
            </h1>
            <p className="mt-6 max-w-[34ch] text-lg leading-relaxed text-muted-foreground">
              AuraEDU runs the whole school — admissions, registers, results, report cards and fees — for many schools at once. Each keeps its own brand, its own data, and only the features it needs.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <Button onClick={undefined} className="h-11 px-5">Sign up your school</Button>
              <Button variant="secondary" className="h-11 px-5">See a live school</Button>
            </div>
            <div className="mt-7 flex items-center gap-3 font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
              <span>UPSHS</span><span className="size-1 rounded-full bg-current" aria-hidden="true" /><span>Aboom AME Zion C</span><span className="size-1 rounded-full bg-current" aria-hidden="true" /><span>+ new schools weekly</span>
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

        {/* MULTI-TENANT */}
        <section id="schools" className="border-t border-border">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <div className="flex flex-col gap-3.5">
              <Eyebrow>One codebase · many schools</Eyebrow>
              <h2 className="text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl">The same system, wearing each school&apos;s colours.</h2>
              <p className="max-w-[56ch] text-muted-foreground">No school gets a separate app. Branding, academic structure, grading and enabled features are configuration — so a new school is onboarded, not rebuilt. Data stays isolated, tenant by tenant.</p>
            </div>
            <div className="mt-9 grid gap-4 md:grid-cols-3">
              {schools.map((s) => (
                <div key={s.crest} className="rounded-lg border border-border bg-surface p-5">
                  <div className="flex items-center gap-3 border-b border-border pb-3.5">
                    <span className="grid size-9 flex-none place-items-center rounded-lg font-display text-sm font-extrabold text-white" style={{ backgroundColor: s.color }}>{s.crest}</span>
                    <div>
                      <div className="text-sm font-bold leading-tight">{s.name}</div>
                      <div className="mt-0.5 font-mono text-[11px] text-muted-foreground">{s.code}</div>
                    </div>
                  </div>
                  <div className="mt-3.5 flex flex-wrap gap-1.5">
                    {s.chips.map((c) => (
                      <span key={c} className="rounded-full border px-2.5 py-1 font-mono text-[11px]" style={{ color: s.color, borderColor: `${s.color}55`, backgroundColor: `${s.color}14` }}>{c}</span>
                    ))}
                    <span className="rounded-full border border-border px-2.5 py-1 font-mono text-[11px] text-muted-foreground">{s.off}</span>
                  </div>
                  <div className="mt-4 flex items-center gap-2 text-[13px] text-muted-foreground">
                    <Tick className="w-3.5" />{s.note}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* MODULES as a register */}
        <section id="modules" className="border-t border-border">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <div className="flex flex-col gap-3.5">
              <Eyebrow>Features toggle per school</Eyebrow>
              <h2 className="text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl">A register of everything a school can switch on.</h2>
              <p className="max-w-[56ch] text-muted-foreground">Each module is an independent feature flag. Turn it on and it appears in the portal, the API and the reports; leave it off and it stays out of the way. Shown below: UPSHS.</p>
            </div>
            <div className="mt-8 overflow-hidden rounded-lg border border-border bg-surface">
              {modules.map((m) => (
                <div key={m.key} className="grid grid-cols-[1fr_auto] items-center gap-4 border-b border-border px-5 py-4 last:border-b-0 hover:bg-muted sm:grid-cols-[200px_1fr_88px]">
                  <div className="font-mono text-[13px]">{m.key}</div>
                  <div className="hidden sm:block">
                    <div className="text-sm font-semibold">{m.name}</div>
                    <div className="mt-0.5 text-sm text-muted-foreground">{m.desc}</div>
                  </div>
                  <span className={`inline-flex items-center justify-end gap-1.5 font-mono text-[11px] uppercase tracking-[0.1em] ${m.on ? "text-primary" : "text-muted-foreground"}`}>
                    {m.on ? <Tick className="w-3.5" /> : <span className="h-0.5 w-2.5 rounded bg-current" />}
                    {m.on ? "On" : "Off"}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* ROLES */}
        <section id="roles" className="border-t border-border">
          <div className="mx-auto max-w-6xl px-6 py-16">
            <div className="flex flex-col gap-3.5">
              <Eyebrow>Who it&apos;s for</Eyebrow>
              <h2 className="text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl">Everyone at the school, on the right screen.</h2>
              <p className="max-w-[56ch] text-muted-foreground">Teachers, parents and students get both web and a mobile app. Administrators run the school from the web console.</p>
            </div>
            <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              {roles.map((r) => (
                <div key={r.name} className="rounded-lg border border-border bg-surface p-5">
                  <h3 className="text-base font-bold">{r.name}</h3>
                  <p className="mt-1.5 text-[13px] text-muted-foreground">{r.desc}</p>
                  <div className="mt-3.5 font-mono text-[10.5px] uppercase tracking-[0.08em] text-muted-foreground">{r.plat}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* CTA */}
        <section id="join" className="px-6 pb-4 pt-16">
          <div className="mx-auto max-w-6xl rounded-xl bg-ink-950 p-10 text-paper-50 sm:p-14">
            <span className="inline-flex items-center gap-2.5 font-mono text-xs uppercase tracking-[0.16em] text-ink-200"><Tick className="w-3.5 text-primary" />Onboard in a day</span>
            <h2 className="mt-3.5 max-w-[18ch] text-balance font-display text-3xl font-extrabold tracking-tight sm:text-[2.75rem]">Bring your school onto AuraEDU.</h2>
            <p className="mt-4 max-w-[46ch] text-ink-200">Create the school, choose a plan, switch on the features you need, upload your logo and colours, import students and staff — then go live.</p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Button className="h-11 px-5">Sign up your school</Button>
              <Button variant="secondary" className="h-11 border-ink-800 bg-transparent px-5 text-paper-50 hover:bg-ink-900">Book a walkthrough</Button>
            </div>
          </div>
        </section>
      </main>

      <footer className="mx-auto max-w-6xl px-6 py-10">
        <div className="flex flex-wrap items-center justify-between gap-5 border-t border-border pt-6 text-[13px] text-muted-foreground">
          <span className="flex items-center gap-2 font-display text-base font-extrabold text-foreground">
            <span className="grid size-5 place-items-center rounded bg-foreground" aria-hidden="true"><Tick className="w-3 text-primary" /></span>AuraEDU
          </span>
          <span className="font-mono">© 2026 · One platform, every school</span>
        </div>
      </footer>
    </div>
  );
}
