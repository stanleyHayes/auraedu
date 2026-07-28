import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, CalendarCheck2, Megaphone, ReceiptText, School, Wallet } from "lucide-react";
import { Eyebrow } from "@/components/brand-primitives";
import { ScrollReveal, StaggerChildren, StaggerItem } from "@/components/motion-primitives";
import { TermFlow } from "@/components/term-flow";

export const metadata: Metadata = {
  title: "For parents",
  description:
    "Got a link from your school? See how AuraEDU shows your child's fees, receipts, report cards, attendance and announcements—and how sign-in works.",
};

const parentViews = [
  {
    title: "Fees and balances",
    copy: "See exactly what is owed, what it is for and when it is due—no end-of-term surprises.",
    icon: Wallet,
  },
  {
    title: "Receipts",
    copy: "Every payment you make returns a receipt you can keep, trace and show if ever asked.",
    icon: ReceiptText,
  },
  {
    title: "Report cards",
    copy: "Results arrive the moment the school publishes them, in the format you already know.",
    icon: School,
  },
  {
    title: "Attendance",
    copy: "Know that your child arrived, and hear quickly on the days they did not.",
    icon: CalendarCheck2,
  },
  {
    title: "Announcements",
    copy: "School notices reach you directly—no more relying on a crumpled letter in a bag.",
    icon: Megaphone,
  },
];

const signinSteps = [
  {
    title: "Your school issues access",
    copy: "The school links you to your child and sends you an invitation or a link. You never register yourself into someone else's family.",
  },
  {
    title: "You sign in with your own details",
    copy: "Use the phone number or email the school already knows. One account follows your child across classes and terms.",
  },
  {
    title: "You see only your child",
    copy: "Your view is permission-scoped to your own family—fees, results and notices for your children, nothing else.",
  },
];

const faqTeaser = [
  {
    question: "Is my child's information private?",
    answer:
      "Yes. Records stay inside your school's isolated tenant, and you see only your own family.",
  },
  {
    question: "Who pays the mobile money charges?",
    answer: "The charge is shown before you confirm; who absorbs it is the school's policy choice.",
  },
  {
    question: "What if a payment fails?",
    answer: "Nothing is marked paid. You can retry safely—duplicates are rejected automatically.",
  },
];

export default function ForParentsPage() {
  return (
    <div className="overflow-hidden">
      <section className="trust-stage text-white">
        <ScrollReveal className="mx-auto max-w-7xl px-6 py-24 sm:py-28">
          <Eyebrow inverse>For parents & guardians</Eyebrow>
          <h1 className="mt-5 max-w-[13ch] text-balance text-[clamp(3.2rem,6vw,6.5rem)] font-bold leading-[0.9] tracking-[-0.055em]">
            Got a link from your school?{" "}
            <span className="text-teal-bright">Here is what it opens.</span>
          </h1>
          <p className="mt-7 max-w-3xl text-lg leading-8 text-slate-300">
            When your school runs on AuraEDU, that link is your window into your child’s school
            life—fees, receipts, report cards, attendance and announcements, all in one trusted
            place.
          </p>
        </ScrollReveal>
      </section>

      <section className="bg-white" aria-labelledby="parent-views-title">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal>
            <Eyebrow>What you will see</Eyebrow>
            <h2
              id="parent-views-title"
              className="mt-4 max-w-[16ch] text-balance text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-navy-deep sm:text-5xl"
            >
              The answers you used to call the school for.
            </h2>
          </ScrollReveal>
          <StaggerChildren className="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {parentViews.map((view, index) => {
              const Icon = view.icon;
              return (
                <StaggerItem key={view.title}>
                  <article className="relative h-full min-h-[210px] overflow-hidden rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                    <span
                      aria-hidden="true"
                      className="pointer-events-none absolute -top-2 right-4 font-heading text-[76px] font-bold leading-none text-cobalt/[0.07]"
                    >
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <span className="relative grid size-11 place-items-center rounded-xl bg-accent text-cobalt">
                      <Icon className="size-5" aria-hidden="true" />
                    </span>
                    <h3 className="relative mt-5 text-xl font-bold tracking-[-0.03em] text-navy-deep">
                      {view.title}
                    </h3>
                    <p className="relative mt-2.5 text-sm leading-6 text-slate-600">{view.copy}</p>
                  </article>
                </StaggerItem>
              );
            })}
            <StaggerItem>
              <article className="flex h-full min-h-[210px] flex-col justify-between rounded-2xl bg-navy-deep p-6 text-white">
                <p className="max-w-[24ch] text-xl font-bold leading-snug tracking-[-0.02em]">
                  One sign-in. Every term. Every child in your care.
                </p>
                <Link
                  href="#sign-in"
                  className="mt-6 inline-flex w-fit items-center gap-2 text-sm font-bold text-lime-signal"
                >
                  How sign-in works <ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </article>
            </StaggerItem>
          </StaggerChildren>
        </div>
      </section>

      <section id="sign-in" className="scroll-mt-24 bg-cool-mist" aria-labelledby="signin-title">
        <div className="mx-auto grid max-w-7xl gap-12 px-6 py-24 lg:grid-cols-2 lg:items-center">
          <ScrollReveal>
            <Eyebrow>Getting in</Eyebrow>
            <h2
              id="signin-title"
              className="mt-4 max-w-[14ch] text-balance text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-navy-deep sm:text-5xl"
            >
              Sign-in works because the school vouches for you.
            </h2>
            <StaggerChildren className="mt-10 grid gap-0">
              {signinSteps.map((step, index) => (
                <StaggerItem
                  key={step.title}
                  className="grid grid-cols-[auto_1fr] items-start gap-4 border-t border-slate-200 py-5 last:border-b"
                >
                  <span className="grid size-10 place-items-center rounded-lg bg-white font-mono text-xs font-bold text-teal-strong shadow-sm">
                    {String(index + 1).padStart(2, "0")}
                  </span>
                  <div>
                    <h3 className="font-bold text-navy-deep">{step.title}</h3>
                    <p className="mt-1.5 text-sm leading-6 text-slate-600">{step.copy}</p>
                  </div>
                </StaggerItem>
              ))}
            </StaggerChildren>
          </ScrollReveal>
          <ScrollReveal delay={0.1}>
            <TermFlow />
          </ScrollReveal>
        </div>
      </section>

      <section className="bg-white" aria-labelledby="parent-faq-title">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="flex flex-col justify-between gap-6 md:flex-row md:items-end">
            <div>
              <Eyebrow>Quick answers</Eyebrow>
              <h2
                id="parent-faq-title"
                className="mt-4 max-w-[16ch] text-balance text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-navy-deep sm:text-5xl"
              >
                The three questions every parent asks first.
              </h2>
            </div>
            <Link href="/faq" className="text-link group w-fit">
              Read the full FAQ
              <ArrowRight
                className="size-4 transition-transform group-hover:translate-x-1"
                aria-hidden="true"
              />
            </Link>
          </ScrollReveal>
          <StaggerChildren className="mt-10 grid gap-px overflow-hidden rounded-2xl border border-slate-200 bg-slate-200 md:grid-cols-3">
            {faqTeaser.map((item) => (
              <StaggerItem key={item.question} className="bg-white p-7">
                <h3 className="font-bold leading-snug text-navy-deep">{item.question}</h3>
                <p className="mt-3 text-sm leading-6 text-slate-600">{item.answer}</p>
              </StaggerItem>
            ))}
          </StaggerChildren>
        </div>
      </section>

      <section className="px-6 pb-8">
        <ScrollReveal className="final-cta mx-auto max-w-7xl">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-teal-bright">
              Run a school?
            </p>
            <h2 className="mt-4 max-w-[16ch] text-balance font-heading text-4xl font-bold leading-[1.02] tracking-[-0.04em] text-white sm:text-6xl">
              Give every parent this window into school life.
            </h2>
          </div>
          <div className="lg:text-right">
            <p className="max-w-lg leading-7 text-slate-300 lg:ml-auto">
              Parents adopt what schools provide. Start the review that brings fees, results and
              notices into one place families trust.
            </p>
            <div className="mt-7 flex flex-col gap-3 sm:flex-row lg:justify-end">
              <Link href="/signup" className="cta-primary">
                Start your school <ArrowRight className="size-4" aria-hidden="true" />
              </Link>
              <Link href="/features" className="cta-secondary">
                See the parent portal
              </Link>
            </div>
          </div>
        </ScrollReveal>
      </section>
    </div>
  );
}
