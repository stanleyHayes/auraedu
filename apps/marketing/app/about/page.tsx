import Image from "next/image";
import Link from "next/link";
import {
  ArrowRight,
  BookOpen,
  HeartHandshake,
  Lightbulb,
  ShieldCheck,
  UsersRound,
} from "lucide-react";
import { Eyebrow } from "@/components/brand-primitives";
import { ScrollReveal, StaggerChildren, StaggerItem } from "@/components/motion-primitives";

export const metadata = {
  title: "Why AuraEDU",
  description: "Why AuraEDU exists and the principles behind the education operating system.",
};

const principles = [
  {
    number: "01",
    title: "Learning before software",
    copy: "Every capability should improve learning, school reliability or the time educators have for students.",
    icon: BookOpen,
  },
  {
    number: "02",
    title: "Less work for teachers",
    copy: "A new feature must not create another reporting burden or duplicate a workflow teachers already complete.",
    icon: HeartHandshake,
  },
  {
    number: "03",
    title: "AI assists people",
    copy: "AI can surface evidence, patterns and options. Teachers and school leaders remain responsible for decisions.",
    icon: Lightbulb,
  },
  {
    number: "04",
    title: "Distinct schools stay distinct",
    copy: "Every school keeps its identity, configuration and isolated data without receiving a separate codebase.",
    icon: ShieldCheck,
  },
];

export default function AboutPage() {
  return (
    <div className="overflow-hidden">
      <section className="about-hero bg-white">
        <div className="mx-auto grid max-w-[1440px] lg:grid-cols-[1.05fr_0.95fr]">
          <ScrollReveal className="flex min-h-[650px] flex-col justify-center px-6 py-20 sm:px-10 lg:px-16">
            <Eyebrow>Why AuraEDU exists</Eyebrow>
            <h1 className="mt-6 max-w-[13ch] text-balance text-[clamp(3.5rem,6vw,7rem)] font-bold leading-[0.89] tracking-[-0.06em] text-navy-deep">
              School is a human system.{" "}
              <span className="text-teal-strong">Its software should be too.</span>
            </h1>
            <p className="mt-7 max-w-[60ch] text-lg leading-8 text-slate-600">
              Every school day asks people to coordinate learning, care, money, time, communication
              and public trust. AuraEDU exists to make that work coherent—without flattening the
              character of the school.
            </p>
          </ScrollReveal>
          <div className="relative min-h-[520px] lg:min-h-full">
            <Image
              src="/images/auraedu/role-family-source.png"
              alt="A family reviewing a school update together"
              fill
              priority
              sizes="(max-width: 1024px) 100vw, 48vw"
              className="object-cover"
            />
            <div className="about-photo-note">
              <UsersRound className="size-5 text-lime-signal" />
              <p>
                <strong>Technology belongs in the background.</strong>
                <span>People, relationships and learning stay in front.</span>
              </p>
            </div>
          </div>
        </div>
      </section>

      <section className="trust-stage text-white">
        <div className="mx-auto grid max-w-7xl gap-12 px-6 py-24 lg:grid-cols-[0.72fr_1.28fr]">
          <ScrollReveal>
            <Eyebrow inverse>The problem we refuse to normalize</Eyebrow>
            <h2 className="mt-4 max-w-[11ch] text-balance text-4xl font-bold leading-[1] tracking-[-0.045em] sm:text-6xl">
              Education should not run on{" "}
              <span className="text-teal-bright">institutional memory.</span>
            </h2>
          </ScrollReveal>
          <ScrollReveal className="grid gap-8 text-lg leading-8 text-slate-300" delay={0.1}>
            <p>
              When records live in paper files, decisions live in private spreadsheets and family
              communication lives in scattered chats, the school becomes harder to understand—and
              easier to fail quietly.
            </p>
            <p>
              AuraEDU gives the institution a dependable operating memory. It connects the daily
              facts without turning teachers into data-entry staff or replacing professional
              judgement with a score.
            </p>
            <div className="border-l-2 border-lime-signal pl-6 text-2xl font-semibold leading-9 text-white">
              The point is not to digitize every task. The point is to help the right person act at
              the right moment.
            </div>
          </ScrollReveal>
        </div>
      </section>

      <section className="bg-cool-mist">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="grid gap-8 lg:grid-cols-2 lg:items-end">
            <div>
              <Eyebrow>Product constitution</Eyebrow>
              <h2 className="mt-4 max-w-[14ch] text-4xl font-bold tracking-[-0.045em] text-navy-deep sm:text-6xl">
                Rules strong enough to say no.
              </h2>
            </div>
            <p className="max-w-xl text-lg leading-8 text-slate-600 lg:justify-self-end">
              These principles are not campaign language. They are constraints for product, design,
              engineering and AI behaviour.
            </p>
          </ScrollReveal>
          <StaggerChildren className="mt-14 grid gap-px overflow-hidden rounded-2xl border border-slate-200 bg-slate-200 md:grid-cols-2">
            {principles.map((item) => {
              const Icon = item.icon;
              return (
                <StaggerItem key={item.number} className="h-full">
                  <article className="principle-card">
                    <div className="flex items-center justify-between">
                      <span>{item.number}</span>
                      <Icon className="size-6 text-cobalt" />
                    </div>
                    <h3>{item.title}</h3>
                    <p>{item.copy}</p>
                  </article>
                </StaggerItem>
              );
            })}
          </StaggerChildren>
        </div>
      </section>

      <section className="bg-white">
        <ScrollReveal className="mx-auto grid max-w-7xl gap-10 px-6 py-24 lg:grid-cols-[1.1fr_0.9fr] lg:items-center">
          <div className="relative aspect-[5/4] overflow-hidden rounded-2xl">
            <Image
              src="/images/auraedu/role-leader-source.png"
              alt="A school leader discussing information with a colleague"
              fill
              sizes="(max-width: 1024px) 100vw, 56vw"
              className="object-cover"
            />
          </div>
          <div>
            <Eyebrow>Built with schools, not at them</Eyebrow>
            <h2 className="mt-4 text-4xl font-bold leading-tight tracking-[-0.04em] text-navy-deep sm:text-5xl">
              Bring the real workflow. Keep the institution's character.
            </h2>
            <p className="mt-5 text-lg leading-8 text-slate-600">
              We start with how the school already works, where information breaks down and which
              decisions carry the most consequence. Then we map the smallest useful change.
            </p>
            <Link href="/contact" className="text-link mt-8">
              Talk through your school <ArrowRight className="size-4" />
            </Link>
          </div>
        </ScrollReveal>
      </section>

      <section className="bg-cool-mist">
        <ScrollReveal className="mx-auto flex max-w-7xl flex-col gap-8 px-6 py-20 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <Eyebrow>Build deliberately</Eyebrow>
            <h2 className="mt-3 max-w-[21ch] text-4xl font-bold tracking-[-0.04em] text-navy-deep">
              A better school system begins with one honest operating problem.
            </h2>
          </div>
          <Link href="/signup" className="cta-solid">
            Start your school setup <ArrowRight className="size-4" />
          </Link>
        </ScrollReveal>
      </section>
    </div>
  );
}
