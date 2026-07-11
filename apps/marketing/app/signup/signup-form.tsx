"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { Button, Input, Label, Select, Skeleton } from "@auraedu/ui";
import { createGatewayClient, ApiError } from "@auraedu/api-client";
import type { components as BillingComponents } from "@auraedu/shared-types/openapi/billing.v1";
import type { components as TenantComponents } from "@auraedu/shared-types/openapi/tenant.v1";

type Plan = BillingComponents["schemas"]["Plan"];
type PlanList = BillingComponents["schemas"]["PlanList"];
type TenantCreate = TenantComponents["schemas"]["TenantCreate"];
type Tenant = TenantComponents["schemas"]["Tenant"];

const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";
const client = createGatewayClient({ baseUrl: `${apiBase}/api/v1` });

const defaultPlans: Plan[] = [
  { id: "starter-id", key: "starter", name: "Starter", price_monthly: 99, features: [] },
  { id: "growth-id", key: "growth", name: "Growth", price_monthly: 249, features: [] },
  {
    id: "professional-id",
    key: "professional",
    name: "Professional",
    price_monthly: 499,
    features: [],
  },
];

const countries = [
  { code: "GH", name: "Ghana" },
  { code: "NG", name: "Nigeria" },
  { code: "KE", name: "Kenya" },
  { code: "ZA", name: "South Africa" },
  { code: "GB", name: "United Kingdom" },
  { code: "US", name: "United States" },
  { code: "CA", name: "Canada" },
  { code: "OTHER", name: "Other" },
];

function slugify(name: string) {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 50);
}

function generateUuid() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

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

export function SignupForm() {
  const searchParams = useSearchParams();
  const queryPlan = searchParams.get("plan") ?? "";

  const [plans, setPlans] = useState<Plan[]>(defaultPlans);
  const [plansLoading, setPlansLoading] = useState(true);

  const [schoolName, setSchoolName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [country, setCountry] = useState("GH");
  const [planKey, setPlanKey] = useState(queryPlan || "starter");

  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);

  useEffect(() => {
    let cancelled = false;
    client
      .get<PlanList>("/billing/plans")
      .then((list) => {
        if (cancelled) return;
        const fetched = list.data?.length ? list.data : defaultPlans;
        setPlans(fetched);
        if (!queryPlan && fetched[0]) setPlanKey(fetched[0].key);
      })
      .catch(() => {
        if (cancelled) return;
        setPlans(defaultPlans);
      })
      .finally(() => {
        if (!cancelled) setPlansLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [queryPlan]);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setResult(null);

    try {
      const tenantCode = slugify(schoolName) || generateUuid();
      const payload: TenantCreate = {
        tenant_code: tenantCode,
        name: schoolName,
        short: schoolName,
        status: "onboarding",
        domain: null,
        plan: planKey as TenantCreate["plan"],
        branding: {
          logo_url: null,
          brand: { primary: "#C6402F", secondary: null },
        },
      };

      const tenant = await client.post<Tenant & { id?: string }>("/tenants", payload);
      const tenantId = tenant.id ?? generateUuid();

      await client.post("/billing/subscriptions", {
        tenant_id: tenantId,
        plan_key: planKey,
        billing_email: email,
      });

      setResult({
        ok: true,
        message: `Your school “${tenant.name ?? schoolName}” is set up. We will email ${email} with next steps.`,
      });
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Something went wrong. Please try again.";
      setResult({ ok: false, message });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto max-w-2xl px-6 py-16">
      <div className="text-center">
        <Eyebrow>Sign up your school</Eyebrow>
        <h1 className="mt-4 text-balance font-heading text-3xl font-extrabold tracking-tight sm:text-4xl lg:text-5xl">
          Create your AuraEDU school.
        </h1>
        <p className="mx-auto mt-3 max-w-[56ch] text-muted-foreground">
          Start a free trial. No credit card required.
        </p>
      </div>

      <div className="mt-10 rounded-lg border border-border bg-surface p-6 sm:p-8">
        {result?.ok ? (
          <div className="py-6 text-center" role="status">
            <div className="mx-auto grid size-12 place-items-center rounded-full bg-[var(--color-brand-tint)] text-primary">
              <Tick className="w-6" />
            </div>
            <h2 className="mt-4 font-heading text-xl font-extrabold">School created</h2>
            <p className="mx-auto mt-2 max-w-[46ch] text-sm text-muted-foreground">
              {result.message}
            </p>
            <Button asChild className="mt-6 h-11 px-6">
              <Link href="/">Back to home</Link>
            </Button>
          </div>
        ) : (
          <form onSubmit={(e) => void handleSubmit(e)} className="space-y-5">
            <div>
              <Label htmlFor="schoolName" required>
                School name
              </Label>
              <Input
                id="schoolName"
                value={schoolName}
                onChange={(e) => setSchoolName(e.target.value)}
                required
                placeholder="e.g. University Practice Senior High School"
              />
            </div>

            <div>
              <Label htmlFor="email" required>
                Admin email
              </Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                placeholder="admin@school.edu.gh"
              />
            </div>

            <div className="grid gap-5 sm:grid-cols-2">
              <div>
                <Label htmlFor="phone">Phone</Label>
                <Input
                  id="phone"
                  type="tel"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="+233 20 000 0000"
                />
              </div>
              <div>
                <Label htmlFor="country">Country</Label>
                <Select id="country" value={country} onChange={(e) => setCountry(e.target.value)}>
                  {countries.map((c) => (
                    <option key={c.code} value={c.code}>
                      {c.name}
                    </option>
                  ))}
                </Select>
              </div>
            </div>

            <div>
              <Label htmlFor="plan" required>
                Choose a plan
              </Label>
              {plansLoading ? (
                <Skeleton className="h-11 w-full" />
              ) : (
                <Select
                  id="plan"
                  value={planKey}
                  onChange={(e) => setPlanKey(e.target.value)}
                  required
                >
                  {plans.map((p) => (
                    <option key={p.key} value={p.key}>
                      {p.name}
                    </option>
                  ))}
                </Select>
              )}
            </div>

            {result && !result.ok ? (
              <div className="rounded-md border border-[var(--color-crit)]/30 bg-[var(--color-crit)]/10 p-3 text-sm text-[var(--color-crit)]">
                {result.message}
              </div>
            ) : null}

            <Button
              type="submit"
              loading={submitting}
              loadingLabel="Creating school"
              className="h-11 w-full"
            >
              Create school & start trial
            </Button>

            <p className="text-center text-xs text-muted-foreground">
              By signing up, you agree to our{" "}
              <Link href="/" className="underline underline-offset-2">
                Terms of Service
              </Link>{" "}
              and{" "}
              <Link href="/" className="underline underline-offset-2">
                Privacy Policy
              </Link>
              .
            </p>
          </form>
        )}
      </div>
    </div>
  );
}
