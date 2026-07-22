import Image from "next/image";
import Link from "next/link";
import {
  ArrowRight,
  BarChart3,
  BookOpenCheck,
  Building2,
  Check,
  CreditCard,
  Database,
  GraduationCap,
  LockKeyhole,
  Network,
  ShieldCheck,
  Smartphone,
  Sparkles,
  UsersRound,
} from "lucide-react";
import { Eyebrow } from "@/components/brand-primitives";
import { ScrollReveal, StaggerChildren, StaggerItem } from "@/components/motion-primitives";

export const metadata = {
  title: "Platform",
  description:
    "Explore AuraEDU's connected school operations, teaching, family engagement, finance, analytics and accountable AI.",
};

const operatingAreas = [
  {
    id: "people-records",
    number: "01",
    label: "Foundation",
    title: "Know your school. Keep every record dependable.",
    description:
      "A permission-aware source of truth for the people, places and structures that make a school work.",
    icon: Database,
    modules: [
      "Admissions",
      "Students & guardians",
      "Staff",
      "Academic years",
      "Classes & subjects",
    ],
  },
  {
    id: "teaching-learning",
    number: "02",
    label: "Learning",
    title: "Turn daily classroom activity into useful learning context.",
    description:
      "Fast teacher workflows for attendance, assessment and progress—without a second reporting job.",
    icon: BookOpenCheck,
    modules: ["Attendance", "Assignments", "Assessments", "Gradebook", "Report cards", "CBT exams"],
  },
  {
    id: "finance-communication",
    number: "03",
    label: "Operations",
    title: "Keep money, messages and decisions connected.",
    description:
      "Give leaders and families one reliable view of balances, payments, announcements and follow-up.",
    icon: CreditCard,
    modules: [
      "Fee structures",
      "Invoices & balances",
      "Online payments",
      "Announcements",
      "Email, SMS & WhatsApp",
    ],
  },
  {
    id: "portals-mobile",
    number: "04",
    label: "Experience",
    title: "Give every role a focused place to do its work.",
    description:
      "One shared platform with clear, role-aware experiences across web, mobile and the public school website.",
    icon: Smartphone,
    modules: [
      "School admin web",
      "Teacher web + mobile",
      "Parent web + mobile",
      "Student web + mobile",
      "Public school website",
    ],
  },
  {
    id: "insight-ai",
    number: "05",
    label: "Intelligence",
    title: "See the signal. Keep the decision human.",
    description:
      "Explainable recommendations and risk signals stay inside the school's data boundary and include a review path.",
    icon: Sparkles,
    modules: [
      "Operational analytics",
      "Learning recommendations",
      "Risk predictions",
      "Career guidance",
      "Teacher review",
    ],
  },
];

const foundations = [
  {
    icon: Network,
    title: "One connected core",
    copy: "Modules share identity and explicit contracts—not hidden database shortcuts.",
  },
  {
    icon: LockKeyhole,
    title: "Permission-aware",
    copy: "Every role sees only the learners, actions and records it is allowed to access.",
  },
  {
    icon: ShieldCheck,
    title: "School data stays separate",
    copy: "Tenant isolation is designed into every request, event and stored record.",
  },
  {
    icon: Building2,
    title: "Distinct by default",
    copy: "Schools keep their own brand, configuration, feature set and operating model.",
  },
];

export default function FeaturesPage() {
  return (
    <div className="overflow-hidden">
      <section className="feature-hero text-white">
        <div className="mx-auto grid min-h-[650px] max-w-[1440px] lg:grid-cols-[0.92fr_1.08fr]">
          <ScrollReveal className="flex flex-col justify-center px-6 py-20 sm:px-10 lg:px-16">
            <Eyebrow inverse>Inside the operating system</Eyebrow>
            <h1 className="mt-6 max-w-[12ch] text-balance font-heading text-[clamp(3.25rem,5.5vw,6.4rem)] font-bold leading-[0.9] tracking-[-0.055em]">
              Not more tools. <span className="text-teal-bright">One school rhythm.</span>
            </h1>
            <p className="mt-7 max-w-[58ch] text-lg leading-8 text-slate-300">
              AuraEDU turns disconnected records, tasks and conversations into one dependable
              flow—from the first admission enquiry to the next learning decision.
            </p>
            <div className="mt-9 flex flex-wrap gap-3">
              <Link href="/signup" className="cta-primary group">
                Start with your priority{" "}
                <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" />
              </Link>
              <Link href="#platform-map" className="cta-secondary">
                See the complete map
              </Link>
            </div>
          </ScrollReveal>
          <div className="relative min-h-[500px] overflow-hidden lg:min-h-full">
            <Image
              src="/images/auraedu/role-teacher-source.png"
              alt="A teacher supporting a learner"
              fill
              priority
              sizes="(max-width: 1024px) 100vw, 54vw"
              className="object-cover object-left"
            />
            <div className="feature-photo-shade absolute inset-0" aria-hidden="true" />
            <div className="absolute inset-x-6 bottom-7 grid gap-2 sm:grid-cols-3 lg:left-auto lg:right-10 lg:w-[560px]">
              {[
                { icon: GraduationCap, title: "Learner context", copy: "One clear history" },
                { icon: UsersRound, title: "People connected", copy: "Roles stay focused" },
                { icon: BarChart3, title: "Signals explained", copy: "Humans decide" },
              ].map(({ icon: Icon, title, copy }) => (
                <div key={title} className="feature-hero-stat">
                  <Icon className="size-5 text-lime-signal" aria-hidden="true" />
                  <strong>{title}</strong>
                  <span>{copy}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section id="platform-map" className="bg-white">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="grid gap-8 lg:grid-cols-[0.72fr_1.28fr] lg:items-end">
            <div>
              <Eyebrow>Platform map</Eyebrow>
              <h2 className="mt-4 max-w-[12ch] text-balance text-4xl font-bold leading-[0.98] tracking-[-0.045em] text-navy-deep sm:text-6xl">
                Follow the work, not the org chart.
              </h2>
            </div>
            <p className="max-w-xl text-lg leading-8 text-slate-600 lg:justify-self-end">
              Each operating area can launch independently. Together, they build a continuous view
              of school life without turning every screen into a control panel.
            </p>
          </ScrollReveal>

          <div className="mt-16 border-t border-slate-200">
            {operatingAreas.map((area) => {
              const Icon = area.icon;
              return (
                <ScrollReveal key={area.id}>
                  <article id={area.id} className="platform-row scroll-mt-24">
                    <div className="platform-row-index">
                      <span>{area.number}</span>
                      <Icon className="size-7" strokeWidth={1.8} />
                    </div>
                    <div>
                      <p className="platform-row-label">{area.label}</p>
                      <h3>{area.title}</h3>
                    </div>
                    <div>
                      <p className="platform-row-copy">{area.description}</p>
                      <ul className="platform-module-list">
                        {area.modules.map((module) => (
                          <li key={module}>
                            <Check className="size-3.5" />
                            {module}
                          </li>
                        ))}
                      </ul>
                    </div>
                  </article>
                </ScrollReveal>
              );
            })}
          </div>
        </div>
      </section>

      <section id="platform-control" className="trust-stage text-white">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="grid gap-8 lg:grid-cols-2 lg:items-end">
            <div>
              <Eyebrow inverse>What everything stands on</Eyebrow>
              <h2 className="mt-4 max-w-[12ch] text-balance text-4xl font-bold leading-[1] tracking-[-0.045em] sm:text-6xl">
                Shared technology. <span className="text-teal-bright">Clear boundaries.</span>
              </h2>
            </div>
            <p className="max-w-xl text-lg leading-8 text-slate-300 lg:justify-self-end">
              The platform can grow because its foundations do not change from module to module or
              school to school.
            </p>
          </ScrollReveal>
          <StaggerChildren className="mt-12 grid overflow-hidden rounded-2xl border border-white/10 md:grid-cols-2 lg:grid-cols-4">
            {foundations.map((item) => {
              const Icon = item.icon;
              return (
                <StaggerItem key={item.title} className="h-full">
                  <article className="platform-foundation">
                    <Icon className="size-6 text-lime-signal" />
                    <h3>{item.title}</h3>
                    <p>{item.copy}</p>
                  </article>
                </StaggerItem>
              );
            })}
          </StaggerChildren>
        </div>
      </section>

      <section className="bg-cool-mist">
        <ScrollReveal className="mx-auto grid max-w-7xl gap-8 px-6 py-20 lg:grid-cols-[1fr_auto] lg:items-center">
          <div>
            <Eyebrow>Start where the pressure is</Eyebrow>
            <h2 className="mt-4 max-w-[20ch] text-4xl font-bold leading-tight tracking-[-0.04em] text-navy-deep">
              You do not need to launch the whole platform to feel the difference.
            </h2>
            <p className="mt-4 max-w-2xl leading-7 text-slate-600">
              Choose the first operating problem. We will map the smallest dependable starting set
              and the path from there.
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <Link href="/signup" className="cta-solid">
              Start school setup <ArrowRight className="size-4" />
            </Link>
            <Link href="/contact" className="cta-outline">
              Plan an onboarding
            </Link>
          </div>
        </ScrollReveal>
      </section>
    </div>
  );
}
