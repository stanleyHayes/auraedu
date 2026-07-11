"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Skeleton } from "@auraedu/ui";
import { createGatewayClient } from "@auraedu/api-client";
import type { components } from "@auraedu/shared-types/openapi/billing.v1";

type Plan = components["schemas"]["Plan"];
type PlanList = components["schemas"]["PlanList"];

const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
const client = createGatewayClient({ baseUrl: `${apiBase}/api/v1` });

const placeholderPlans: Plan[] = [
  {
    id: "00000000-0000-0000-0000-000000000001",
    key: "starter",
    name: "Starter",
    price_monthly: 99,
    features: ["Up to 200 students", "Attendance", "Assessments", "Report cards", "Email support"],
  },
  {
    id: "00000000-0000-0000-0000-000000000002",
    key: "growth",
    name: "Growth",
    price_monthly: 249,
    features: ["Up to 1,000 students", "All Starter modules", "Fees & payments", "Parent portal", "Priority support"],
  },
  {
    id: "00000000-0000-0000-0000-000000000003",
    key: "professional",
    name: "Professional",
    price_monthly: 499,
    features: ["Unlimited students", "All Growth modules", "Analytics", "API access", "Dedicated onboarding"],
  },
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

function formatPrice(amount: number | null | undefined) {
  if (amount == null) return "Custom";
  return new Intl.NumberFormat("en-GH", { style: "currency", currency: "GHS", maximumFractionDigits: 0 }).format(
    amount,
  );
}

export default function PricingPage() {
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    client
      .get<PlanList>("/billing/plans")
      .then((list) => {
        if (cancelled) return;
        setPlans(list.data?.length ? list.data : placeholderPlans);
      })
      .catch(() => {
        if (cancelled) return;
        setPlans(placeholderPlans);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="mx-auto max-w-6xl px-6 py-16">
      <div className="flex flex-col gap-3.5 text-center">
        <span className="inline-flex items-center justify-center gap-2.5 font-mono text-xs uppercase tracking-[0.16em] text-muted-foreground">
          <Tick className="w-3.5 text-primary" />
          Simple pricing
        </span>
        <h1 className="text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl lg:text-5xl">
          Plans that scale with your school.
        </h1>
        <p className="mx-auto max-w-[56ch] text-muted-foreground">
          Choose a plan, start a trial, and switch on the modules you need. No setup fees.
        </p>
      </div>

      <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {loading
          ? Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="rounded-lg border border-border bg-surface p-6">
                <Skeleton className="h-6 w-24" />
                <Skeleton className="mt-4 h-10 w-32" />
                <div className="mt-6 space-y-2">
                  {Array.from({ length: 5 }).map((__, j) => (
                    <Skeleton key={j} className="h-4 w-full" />
                  ))}
                </div>
                <Skeleton className="mt-6 h-10 w-full" />
              </div>
            ))
          : plans.map((plan) => (
              <div
                key={plan.id}
                className="flex flex-col rounded-lg border border-border bg-surface p-6 transition-colors hover:border-[var(--color-brand)]/40"
              >
                <h2 className="font-display text-xl font-extrabold">{plan.name}</h2>
                <p className="mt-2 text-3xl font-extrabold tracking-tight">{formatPrice(plan.price_monthly)}</p>
                <p className="text-sm text-muted-foreground">{plan.price_monthly == null ? "" : "per month"}</p>
                <ul className="mt-6 flex-1 space-y-2.5">
                  {(plan.features ?? []).map((feature) => (
                    <li key={feature} className="flex items-start gap-2 text-sm text-foreground">
                      <Tick className="mt-0.5 w-3.5 shrink-0 text-primary" />
                      <span>{feature}</span>
                    </li>
                  ))}
                </ul>
                <Button asChild className="mt-6 h-11 w-full">
                  <Link href={`/signup?plan=${encodeURIComponent(plan.key)}`}>Start trial</Link>
                </Button>
              </div>
            ))}
      </div>

      <p className="mt-8 text-center text-sm text-muted-foreground">
        Need a custom enterprise plan?{" "}
        <Link href="/contact" className="font-semibold text-foreground underline underline-offset-4">
          Contact us
        </Link>
        .
      </p>
    </div>
  );
}
