import Image from "next/image";
import Link from "next/link";
import {
  ArrowRight,
  BarChart3,
  BookOpen,
  Building2,
  CalendarDays,
  Check,
  CircleUserRound,
  CloudUpload,
  Database,
  GraduationCap,
  LockKeyhole,
  MessageSquareText,
  Network,
  ShieldCheck,
  Sparkles,
  SlidersHorizontal,
  UsersRound,
} from "lucide-react";
import { Eyebrow } from "@/components/brand-primitives";
import {
  AnimatedCounter,
  Reveal3D,
  ScrollReveal,
  StaggerChildren,
  StaggerItem,
} from "@/components/motion-primitives";

const dailyRhythm = [
  {
    time: "7:30",
    title: "School opens",
    copy: "People, gates and the day begin.",
    icon: Building2,
  },
  {
    time: "8:00",
    title: "Teaching time",
    copy: "Attendance, lessons and classwork flow.",
    icon: BookOpen,
  },
  {
    time: "12:00",
    title: "Operations",
    copy: "Requests and records stay current.",
    icon: SlidersHorizontal,
  },
  {
    time: "2:00",
    title: "Publish & record",
    copy: "Progress and notices become clear.",
    icon: Check,
  },
  {
    time: "4:00",
    title: "Home connected",
    copy: "Families receive the right update.",
    icon: MessageSquareText,
  },
];

const platformLayers = [
  {
    number: "01",
    title: "Foundation",
    copy: "Secure identity, tenancy, permissions and school records.",
    detail: "A reliable core for every school.",
    icon: Network,
    tone: "blue",
  },
  {
    number: "02",
    title: "Daily operations",
    copy: "Teaching, attendance, finance, communication and resources.",
    detail: "The school day in one rhythm.",
    icon: CalendarDays,
    tone: "teal",
  },
  {
    number: "03",
    title: "Growth",
    copy: "Recruitment, admissions, knowledge and campaign control.",
    detail: "Build relationships responsibly.",
    icon: BarChart3,
    tone: "orange",
  },
  {
    number: "04",
    title: "Intelligence",
    copy: "Trusted analytics and explainable AI with human review.",
    detail: "Turn information into action.",
    icon: Sparkles,
    tone: "ink",
  },
];

const roleStories = [
  {
    role: "Teachers",
    title: "Teach. Notice. Respond.",
    copy: "Keep the daily actions direct—take attendance, record learning and act on useful signals without another reporting burden.",
    href: "/features#teaching-learning",
    image: "/images/auraedu/role-teacher-source.png",
    alt: "A teacher reviewing an exercise book with a student",
  },
  {
    role: "Families",
    title: "Stay close to progress.",
    copy: "Receive the information the school has approved: results, balances, notices, guidance and next steps in one trusted place.",
    href: "/features#portals-mobile",
    image: "/images/auraedu/role-family-source.png",
    alt: "A parent and student reviewing a school update together",
  },
  {
    role: "School leaders",
    title: "Lead with clear context.",
    copy: "See operations, teaching, finance, admissions and engagement without stitching together separate tools or exports.",
    href: "/features#platform-control",
    image: "/images/auraedu/role-leader-source.png",
    alt: "A school leader reviewing information with a colleague",
  },
];

const assurances = [
  {
    title: "Tenant isolation",
    copy: "Every school keeps a separate data boundary.",
    icon: ShieldCheck,
  },
  {
    title: "Permissions that fit",
    copy: "Every role sees only the work it is allowed to do.",
    icon: LockKeyhole,
  },
  {
    title: "Secure data movement",
    copy: "Records move through explicit, auditable contracts.",
    icon: Database,
  },
  {
    title: "Accountable AI",
    copy: "AI explains its evidence and people remain responsible.",
    icon: Sparkles,
  },
];

const operatingModules = [
  { label: "Student records", meta: "Profiles · enrolment", icon: GraduationCap },
  { label: "Attendance", meta: "Daily · subject", icon: Check },
  { label: "Learning", meta: "Lessons · progress", icon: BookOpen, active: true },
  { label: "Communication", meta: "Notices · messages", icon: MessageSquareText },
  { label: "Admissions", meta: "Leads · applications", icon: UsersRound },
  { label: "Analytics", meta: "Signals · review", icon: BarChart3 },
];

export default function HomePage() {
  return (
    <div className="marketing-home overflow-hidden bg-background text-foreground">
      <section className="hero-stage text-white">
        <div className="mx-auto grid min-h-[760px] max-w-[1440px] lg:grid-cols-[0.82fr_1.18fr]">
          <ScrollReveal className="relative z-10 flex flex-col justify-center px-6 py-20 sm:px-10 lg:px-16 lg:py-24">
            <Eyebrow inverse>Education operating system</Eyebrow>
            <h1 className="mt-6 max-w-[15ch] text-balance font-heading text-[clamp(3.25rem,4.8vw,5rem)] font-bold leading-[0.94] tracking-[-0.05em]">
              Run your school clearly.{" "}
              <span className="text-teal-bright">Help every learner move forward.</span>
            </h1>
            <p className="mt-7 max-w-[56ch] text-base leading-7 text-slate-300 sm:text-lg">
              AuraEDU connects school operations, learning, families, growth, trusted data and
              accountable AI—so every role can act with the right context.
            </p>
            <div className="mt-9 flex flex-col gap-3 sm:flex-row">
              <Link href="/signup" className="cta-primary group">
                Start your school
                <ArrowRight
                  className="size-4 transition-transform group-hover:translate-x-1"
                  aria-hidden="true"
                />
              </Link>
              <Link href="/features" className="cta-secondary group">
                Explore the platform
                <ArrowRight
                  className="size-4 transition-transform group-hover:translate-x-1"
                  aria-hidden="true"
                />
              </Link>
            </div>
            <div className="mt-11 grid max-w-xl grid-cols-3 gap-5 border-t border-white/15 pt-6">
              <div>
                <strong className="block text-2xl text-white">
                  <AnimatedCounter value={1} />
                </strong>
                <span className="text-xs text-slate-400">shared identity</span>
              </div>
              <div>
                <strong className="block text-2xl text-white">
                  <AnimatedCounter value={4} />
                </strong>
                <span className="text-xs text-slate-400">operating layers</span>
              </div>
              <div>
                <strong className="block text-2xl text-white">Every</strong>
                <span className="text-xs text-slate-400">school role</span>
              </div>
            </div>
          </ScrollReveal>

          <Reveal3D className="relative min-h-[620px] lg:min-h-full">
            <Image
              src="/images/auraedu/hero-classroom-source.png"
              alt="Students learning in a Ghanaian classroom"
              fill
              priority
              sizes="(max-width: 1024px) 100vw, 58vw"
              className="object-cover object-center"
            />
            <div className="hero-photo-shade absolute inset-0" aria-hidden="true" />
            <div className="absolute inset-x-5 bottom-6 z-10 sm:inset-x-8 lg:bottom-10 lg:left-auto lg:right-10 lg:w-[430px]">
              <div className="operating-canvas" aria-label="Connected AuraEDU modules">
                <div className="flex items-center justify-between border-b border-white/10 px-5 py-4">
                  <div>
                    <p className="text-sm font-semibold text-white">Your operating set</p>
                    <p className="mt-0.5 text-[11px] text-slate-400">
                      Connected around one trusted core
                    </p>
                  </div>
                  <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-lime-signal">
                    <span className="size-1.5 rounded-full bg-lime-signal" aria-hidden="true" />{" "}
                    Live
                  </span>
                </div>
                <StaggerChildren className="grid grid-cols-2 gap-2 p-3 sm:p-4">
                  {operatingModules.map((module) => {
                    const Icon = module.icon;
                    return (
                      <StaggerItem key={module.label}>
                        <div className={`module-chip ${module.active ? "module-chip-active" : ""}`}>
                          <span className="module-icon">
                            <Icon className="size-4" aria-hidden="true" />
                          </span>
                          <span>
                            <strong>{module.label}</strong>
                            <small>{module.meta}</small>
                          </span>
                        </div>
                      </StaggerItem>
                    );
                  })}
                </StaggerChildren>
              </div>
            </div>
          </Reveal3D>
        </div>
      </section>

      <section className="border-b border-slate-200 bg-white" aria-labelledby="connected-day-title">
        <div className="mx-auto grid max-w-7xl gap-12 px-6 py-20 lg:grid-cols-[0.72fr_2.28fr] lg:items-center">
          <ScrollReveal>
            <Eyebrow>A day on the living campus</Eyebrow>
            <h2
              id="connected-day-title"
              className="mt-4 max-w-[13ch] text-balance font-heading text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-navy-deep sm:text-5xl"
            >
              One connected day. <span className="text-teal-strong">Every learner supported.</span>
            </h2>
            <p className="mt-5 max-w-md leading-7 text-slate-600">
              AuraEDU follows the rhythm of school life, carrying the right information to the right
              person at the right moment.
            </p>
          </ScrollReveal>
          <StaggerChildren className="daily-rhythm">
            {dailyRhythm.map((moment) => {
              const Icon = moment.icon;
              return (
                <StaggerItem key={moment.time} className="daily-moment">
                  <span className="daily-time">{moment.time}</span>
                  <span className="daily-icon">
                    <Icon className="size-5" aria-hidden="true" />
                  </span>
                  <h3>{moment.title}</h3>
                  <p>{moment.copy}</p>
                </StaggerItem>
              );
            })}
          </StaggerChildren>
        </div>
      </section>

      <section id="platform" className="bg-cool-mist" aria-labelledby="platform-title">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="flex flex-col justify-between gap-6 md:flex-row md:items-end">
            <div>
              <Eyebrow>Start with what matters</Eyebrow>
              <h2
                id="platform-title"
                className="mt-4 max-w-[16ch] text-balance font-heading text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-navy-deep sm:text-6xl"
              >
                A platform that grows with your school.
              </h2>
            </div>
            <p className="max-w-lg leading-7 text-slate-600">
              Build on one dependable foundation. Enable the daily workflows you need, then add
              growth and intelligence without replacing the platform underneath.
            </p>
          </ScrollReveal>
          <StaggerChildren className="platform-flow mt-14">
            {platformLayers.map((layer, index) => {
              const Icon = layer.icon;
              return (
                <StaggerItem key={layer.number} className="platform-step-wrap">
                  <article className={`platform-step platform-step-${layer.tone}`}>
                    <div className="flex items-start justify-between gap-5">
                      <span className="font-mono text-xs tracking-[0.16em]">{layer.number}</span>
                      <Icon className="size-7" strokeWidth={1.8} aria-hidden="true" />
                    </div>
                    <h3>{layer.title}</h3>
                    <p>{layer.copy}</p>
                    <span className="platform-detail">{layer.detail}</span>
                  </article>
                  {index < platformLayers.length - 1 ? (
                    <ArrowRight className="platform-arrow" aria-hidden="true" />
                  ) : null}
                </StaggerItem>
              );
            })}
          </StaggerChildren>
          <ScrollReveal className="mt-10">
            <Link href="/features" className="text-link group">
              View the complete platform map
              <ArrowRight
                className="size-4 transition-transform group-hover:translate-x-1"
                aria-hidden="true"
              />
            </Link>
          </ScrollReveal>
        </div>
      </section>

      <section className="bg-white" aria-labelledby="roles-title">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="grid gap-7 lg:grid-cols-[0.75fr_1.25fr] lg:items-end">
            <div>
              <Eyebrow>Built around people</Eyebrow>
              <h2
                id="roles-title"
                className="mt-4 max-w-[13ch] text-balance font-heading text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-navy-deep sm:text-6xl"
              >
                Different roles. <span className="text-teal-strong">One shared mission.</span>
              </h2>
            </div>
            <p className="max-w-xl leading-7 text-slate-600 lg:justify-self-end">
              Give people focused experiences shaped to the work they actually do. The platform is
              shared; the interface, permissions and decisions remain role-aware.
            </p>
          </ScrollReveal>
          <StaggerChildren className="mt-12 grid gap-5 lg:grid-cols-3">
            {roleStories.map((story) => (
              <StaggerItem key={story.role} className="h-full">
                <article className="role-story group">
                  <div className="relative aspect-[4/3] overflow-hidden">
                    <Image
                      src={story.image}
                      alt={story.alt}
                      fill
                      sizes="(max-width: 1024px) 100vw, 33vw"
                      className="object-cover transition-transform duration-700 group-hover:scale-[1.025]"
                    />
                  </div>
                  <div className="p-6">
                    <p className="role-label">{story.role}</p>
                    <h3>{story.title}</h3>
                    <p>{story.copy}</p>
                    <Link href={story.href} className="text-link mt-5">
                      Explore for {story.role.toLowerCase()}{" "}
                      <ArrowRight className="size-4" aria-hidden="true" />
                    </Link>
                  </div>
                </article>
              </StaggerItem>
            ))}
          </StaggerChildren>
        </div>
      </section>

      <section className="trust-stage text-white" aria-labelledby="trust-title">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="grid gap-8 lg:grid-cols-[0.72fr_1.28fr] lg:items-end">
            <div>
              <Eyebrow inverse>Trust is infrastructure</Eyebrow>
              <h2
                id="trust-title"
                className="mt-4 max-w-[12ch] text-balance font-heading text-4xl font-bold leading-[1.02] tracking-[-0.04em] sm:text-6xl"
              >
                Secure by design. <span className="text-teal-bright">Accountable by default.</span>
              </h2>
            </div>
            <p className="max-w-xl leading-7 text-slate-300 lg:justify-self-end">
              Shared technology should never blur school boundaries. Identity, access, data movement
              and AI review are designed as part of the product—not added after it.
            </p>
          </ScrollReveal>
          <StaggerChildren className="mt-12 grid gap-px overflow-hidden rounded-2xl border border-white/10 bg-white/10 md:grid-cols-2 xl:grid-cols-4">
            {assurances.map((item) => {
              const Icon = item.icon;
              return (
                <StaggerItem key={item.title} className="trust-item">
                  <Icon className="size-7 text-lime-signal" strokeWidth={1.7} aria-hidden="true" />
                  <h3>{item.title}</h3>
                  <p>{item.copy}</p>
                </StaggerItem>
              );
            })}
          </StaggerChildren>
        </div>
      </section>

      <section id="how-it-works" className="bg-white" aria-labelledby="onboarding-title">
        <div className="mx-auto grid max-w-7xl gap-12 px-6 py-24 lg:grid-cols-[0.7fr_1.3fr] lg:items-center">
          <ScrollReveal>
            <Eyebrow>Get started deliberately</Eyebrow>
            <h2
              id="onboarding-title"
              className="mt-4 max-w-[11ch] text-balance font-heading text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-navy-deep sm:text-6xl"
            >
              Start simple. <span className="text-teal-strong">Grow with confidence.</span>
            </h2>
            <p className="mt-5 max-w-md leading-7 text-slate-600">
              We map AuraEDU to your school’s current work, bring records across carefully and open
              each role with support.
            </p>
          </ScrollReveal>
          <StaggerChildren className="onboarding-flow">
            {[
              {
                title: "Create your school",
                copy: "Set the identity, administrator and operating context.",
                icon: Building2,
              },
              {
                title: "Configure the system",
                copy: "Choose the modules and settings that fit.",
                icon: SlidersHorizontal,
              },
              {
                title: "Bring your records",
                copy: "Import carefully with clear validation.",
                icon: CloudUpload,
              },
              {
                title: "Open each role",
                copy: "Invite people and begin with supported workflows.",
                icon: CircleUserRound,
              },
            ].map((step, index) => {
              const Icon = step.icon;
              return (
                <StaggerItem key={step.title} className="onboarding-step">
                  <span className="onboarding-icon">
                    <Icon className="size-6" aria-hidden="true" />
                  </span>
                  <span>
                    <small>0{index + 1}</small>
                    <strong>{step.title}</strong>
                    <p>{step.copy}</p>
                  </span>
                </StaggerItem>
              );
            })}
          </StaggerChildren>
        </div>
      </section>

      <section className="px-6 pb-8">
        <ScrollReveal className="final-cta mx-auto max-w-7xl">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-teal-bright">
              Your next school day can run differently
            </p>
            <h2 className="mt-4 max-w-[16ch] text-balance font-heading text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-white sm:text-6xl">
              Build one reliable operating rhythm for your school.
            </h2>
          </div>
          <div className="lg:text-right">
            <p className="max-w-lg leading-7 text-slate-300 lg:ml-auto">
              Tell us how your school works today. We will help define the right starting set and a
              path that does not disrupt teaching.
            </p>
            <div className="mt-7 flex flex-col gap-3 sm:flex-row lg:justify-end">
              <Link href="/signup" className="cta-primary">
                Start your school <ArrowRight className="size-4" aria-hidden="true" />
              </Link>
              <Link href="/contact" className="cta-secondary">
                Talk through your needs
              </Link>
            </div>
          </div>
        </ScrollReveal>
      </section>
    </div>
  );
}
