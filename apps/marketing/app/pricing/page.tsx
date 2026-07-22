"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  Check,
  CircleDollarSign,
  Clock3,
  Layers3,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { Skeleton } from "@auraedu/ui";
import type { components } from "@auraedu/shared-types/openapi/billing.v1";
import { Eyebrow } from "@/components/brand-primitives";
import { ScrollReveal } from "@/components/motion-primitives";

type Plan = components["schemas"]["Plan"];
type PlanList = components["schemas"]["PlanList"];

const fallbackPlans: Plan[] = [
  {
    id: "00000000-0000-0000-0000-000000000001",
    code: "starter",
    name: "Starter",
    price_cents: 0,
    currency: "GHS",
    billing_interval: "monthly",
    status: "active",
    features: ["Student records", "Attendance", "Report cards", "Parent & teacher portals"],
  },
  {
    id: "00000000-0000-0000-0000-000000000002",
    code: "growth",
    name: "Growth",
    price_cents: 0,
    currency: "GHS",
    billing_interval: "monthly",
    status: "active",
    features: ["All Starter modules", "Fees & payments", "Assessments", "Messaging"],
  },
  {
    id: "00000000-0000-0000-0000-000000000003",
    code: "professional",
    name: "Professional",
    price_cents: 0,
    currency: "GHS",
    billing_interval: "monthly",
    status: "active",
    features: ["All Growth modules", "CBT exams", "Analytics", "Custom domain"],
  },
  {
    id: "00000000-0000-0000-0000-000000000004",
    code: "ai_plus",
    name: "AI Plus",
    price_cents: 0,
    currency: "GHS",
    billing_interval: "monthly",
    status: "active",
    features: [
      "All Professional modules",
      "Learning recommendations",
      "Risk insights",
      "Career guidance",
    ],
  },
  {
    id: "00000000-0000-0000-0000-000000000005",
    code: "enterprise",
    name: "Enterprise",
    price_cents: 0,
    currency: "GHS",
    billing_interval: "monthly",
    status: "active",
    features: [
      "All AI Plus modules",
      "Custom integrations",
      "Service-level agreement",
      "Guided rollout",
    ],
  },
];

const planContext: Record<string, { for: string; outcome: string }> = {
  starter: {
    for: "Schools replacing fragmented core records",
    outcome: "A dependable first operating layer",
  },
  growth: {
    for: "Schools connecting learning, finance and families",
    outcome: "One daily school rhythm",
  },
  professional: {
    for: "Schools standardising a wider digital campus",
    outcome: "Deeper control and insight",
  },
  ai_plus: {
    for: "Schools ready for reviewed, explainable signals",
    outcome: "Accountable intelligence",
  },
  enterprise: {
    for: "Groups, districts and complex institutions",
    outcome: "A guided operating model",
  },
};

function formatPrice(plan: Plan) {
  if (plan.price_cents <= 0) return "Built around your school";
  return new Intl.NumberFormat("en-GH", {
    style: "currency",
    currency: plan.currency,
    maximumFractionDigits: 0,
  }).format(plan.price_cents / 100);
}

export default function PricingPage() {
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/plans")
      .then(async (response) => {
        if (!response.ok) throw new Error("Plan catalogue unavailable");
        return (await response.json()) as PlanList;
      })
      .then((list) => {
        if (!cancelled) setPlans(list.data?.length ? list.data : fallbackPlans);
      })
      .catch(() => {
        if (!cancelled) setPlans(fallbackPlans);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="overflow-hidden">
      <section className="pricing-hero text-white">
        <ScrollReveal className="mx-auto grid max-w-7xl gap-12 px-6 py-24 lg:grid-cols-[1.15fr_0.85fr] lg:items-end">
          <div>
            <Eyebrow inverse>Plans that fit the journey</Eyebrow>
            <h1 className="mt-5 max-w-[12ch] text-balance text-[clamp(3.4rem,6.5vw,7rem)] font-bold leading-[0.88] tracking-[-0.06em]">
              Price the change. <span className="text-teal-bright">Not just the software.</span>
            </h1>
          </div>
          <div className="pb-2">
            <p className="max-w-xl text-lg leading-8 text-slate-300">
              Your quote reflects the operating set, school size, rollout support and service level
              you actually need. No artificial low price followed by a hidden implementation bill.
            </p>
            <div className="mt-7 flex flex-wrap gap-3 text-xs font-semibold text-slate-300">
              <span className="pricing-proof">
                <ShieldCheck className="size-4 text-lime-signal" />
                No charge before approval
              </span>
              <span className="pricing-proof">
                <Clock3 className="size-4 text-lime-signal" />
                Rollout mapped first
              </span>
            </div>
          </div>
        </ScrollReveal>
      </section>

      <section className="bg-cool-mist">
        <div className="mx-auto max-w-7xl px-6 py-20">
          <ScrollReveal className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <Eyebrow>Choose a starting conversation</Eyebrow>
              <h2 className="mt-4 max-w-[18ch] text-4xl font-bold tracking-[-0.04em] text-navy-deep sm:text-5xl">
                Five operating sets. One path that can grow.
              </h2>
            </div>
            <p className="max-w-lg leading-7 text-slate-600">
              Every plan shares the same secure core. The difference is how much of the school
              rhythm you connect first.
            </p>
          </ScrollReveal>

          <div className="mt-12 grid gap-4 lg:grid-cols-6">
            {loading
              ? Array.from({ length: 5 }).map((_, index) => (
                  <div key={index} className="plan-card lg:col-span-2">
                    <Skeleton className="h-7 w-28" />
                    <Skeleton className="mt-5 h-14 w-full" />
                    <Skeleton className="mt-8 h-40 w-full" />
                  </div>
                ))
              : plans.map((plan, index) => {
                  const context = planContext[plan.code] ?? {
                    for: "A configurable school rollout",
                    outcome: "A dependable operating set",
                  };
                  const featured = plan.code === "growth";
                  const span =
                    index < 3 ? "lg:col-span-2" : index === 3 ? "lg:col-span-3" : "lg:col-span-3";
                  return (
                    <article
                      key={plan.id}
                      className={`plan-card ${span} ${featured ? "plan-card-featured" : ""}`}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div>
                          <p className="plan-kicker">
                            {featured
                              ? "Most common starting point"
                              : `Operating set ${String(index + 1).padStart(2, "0")}`}
                          </p>
                          <h3>{plan.name}</h3>
                        </div>
                        {featured ? (
                          <Sparkles className="size-6 text-lime-signal" />
                        ) : (
                          <span className="plan-number">0{index + 1}</span>
                        )}
                      </div>
                      <p className="plan-outcome">{context.outcome}</p>
                      <p className="plan-for">{context.for}</p>
                      <div className="my-6 h-px bg-current opacity-10" />
                      <ul className="grid gap-2.5">
                        {(plan.features ?? []).map((feature) => (
                          <li key={feature}>
                            <Check className="size-4" />
                            {feature}
                          </li>
                        ))}
                      </ul>
                      <div className="mt-8">
                        <p className="plan-price">{formatPrice(plan)}</p>
                        <p className="mt-1 text-xs opacity-60">
                          Confirmed after a short rollout review
                        </p>
                      </div>
                      <Link
                        href={`/signup?plan=${encodeURIComponent(plan.code)}`}
                        className={featured ? "plan-action plan-action-featured" : "plan-action"}
                      >
                        Explore {plan.name}
                        <ArrowRight className="size-4" />
                      </Link>
                    </article>
                  );
                })}
          </div>
        </div>
      </section>

      <section className="bg-white">
        <div className="mx-auto max-w-7xl px-6 py-24">
          <ScrollReveal className="grid gap-8 lg:grid-cols-[0.8fr_1.2fr]">
            <div>
              <Eyebrow>What shapes your quote</Eyebrow>
              <h2 className="mt-4 max-w-[14ch] text-4xl font-bold leading-tight tracking-[-0.04em] text-navy-deep sm:text-5xl">
                Clear inputs before a single commitment.
              </h2>
            </div>
            <div className="grid gap-px overflow-hidden rounded-2xl border border-slate-200 bg-slate-200 sm:grid-cols-3">
              {[
                {
                  icon: Layers3,
                  title: "Operating scope",
                  copy: "Which modules and workflows should launch together.",
                },
                {
                  icon: CircleDollarSign,
                  title: "School scale",
                  copy: "Learners, campuses, data migration and communication volume.",
                },
                {
                  icon: ShieldCheck,
                  title: "Service level",
                  copy: "Training, rollout guidance, integrations and support response.",
                },
              ].map(({ icon: Icon, title, copy }) => (
                <article key={title} className="bg-white p-7">
                  <Icon className="size-6 text-cobalt" />
                  <h3 className="mt-10 text-xl font-bold text-navy-deep">{title}</h3>
                  <p className="mt-3 text-sm leading-6 text-slate-600">{copy}</p>
                </article>
              ))}
            </div>
          </ScrollReveal>
        </div>
      </section>

      <section className="bg-cool-mist">
        <ScrollReveal className="mx-auto flex max-w-7xl flex-col gap-8 px-6 py-20 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <Eyebrow>Make the first decision small</Eyebrow>
            <h2 className="mt-3 max-w-[22ch] text-4xl font-bold tracking-[-0.04em] text-navy-deep">
              Tell us the pressure point. We will map the sensible starting set.
            </h2>
          </div>
          <div className="flex flex-wrap gap-3">
            <Link href="/signup" className="cta-solid">
              Start your rollout review <ArrowRight className="size-4" />
            </Link>
            <Link href="/contact" className="cta-outline">
              Ask a pricing question
            </Link>
          </div>
        </ScrollReveal>
      </section>
    </div>
  );
}
