"use client";

import { useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { ArrowRight, Check, ClipboardCheck, LockKeyhole, Map, Sparkles } from "lucide-react";
import { Button, Input, Label, Select } from "@auraedu/ui";
import { Eyebrow } from "@/components/brand-primitives";

const plans = [
  { key: "starter", name: "Starter" },
  { key: "growth", name: "Growth" },
  { key: "professional", name: "Professional" },
  { key: "ai_plus", name: "AI Plus" },
  { key: "enterprise", name: "Enterprise" },
];

const countries = [
  { code: "GH", name: "Ghana" },
  { code: "NG", name: "Nigeria" },
  { code: "KE", name: "Kenya" },
  { code: "ZA", name: "South Africa" },
  { code: "GB", name: "United Kingdom" },
  { code: "US", name: "United States" },
  { code: "CA", name: "Canada" },
  { code: "ZZ", name: "Other" },
];

function value(data: FormData, key: string) {
  const field = data.get(key);
  return typeof field === "string" ? field : "";
}

export function SignupForm() {
  const searchParams = useSearchParams();
  const requestedPlan = searchParams.get("plan") ?? "starter";
  const defaultPlan = plans.some((plan) => plan.key === requestedPlan) ? requestedPlan : "starter";
  const [receipt, setReceipt] = useState<{ request_id: string; submitted_at: string } | null>(null);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const requestKey = useRef<string>(crypto.randomUUID());

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    const data = new FormData(event.currentTarget);
    try {
      const response = await fetch("/api/onboarding", {
        method: "POST",
        headers: { "Content-Type": "application/json", "Idempotency-Key": requestKey.current },
        body: JSON.stringify({
          school_name: value(data, "schoolName"),
          administrator_name: value(data, "adminName"),
          email: value(data, "email"),
          phone: value(data, "phone") || null,
          country_code: value(data, "country"),
          plan: value(data, "plan"),
          priorities: value(data, "priorities") || null,
          privacy_notice_version: "2026-07-18",
          accepted_terms: data.get("acceptedTerms") === "on",
          website: value(data, "website"),
        }),
      });
      const result = (await response.json()) as {
        request_id?: string;
        submitted_at?: string;
        message?: string;
      };
      if (!response.ok || !result.request_id || !result.submitted_at) {
        throw new Error(result.message ?? "We could not accept the request. Please try again.");
      }
      setReceipt({ request_id: result.request_id, submitted_at: result.submitted_at });
    } catch (submissionError) {
      setError(
        submissionError instanceof Error
          ? submissionError.message
          : "We could not accept the request. Please try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="signup-stage">
      <div className="mx-auto grid max-w-[1440px] lg:grid-cols-[0.76fr_1.24fr]">
        <aside className="signup-aside text-white">
          <div>
            <Eyebrow inverse>Start your school setup</Eyebrow>
            <h1 className="mt-5 max-w-[11ch] text-balance text-[clamp(3.5rem,6vw,6.4rem)] font-bold leading-[0.88] tracking-[-0.06em]">
              Begin with the school you <span className="text-teal-bright">actually run.</span>
            </h1>
            <p className="mt-7 max-w-lg text-lg leading-8 text-slate-300">
              No instant tenant. No generic setup wizard. We review the operating context first,
              then agree on the right starting set.
            </p>
          </div>
          <div className="signup-steps">
            {[
              {
                icon: ClipboardCheck,
                number: "01",
                title: "Share the real context",
                copy: "Your school, current pressure points and people involved.",
              },
              {
                icon: Map,
                number: "02",
                title: "Map the smallest useful launch",
                copy: "The modules, sequence and support that belong together.",
              },
              {
                icon: LockKeyhole,
                number: "03",
                title: "Provision after approval",
                copy: "Identity, permissions and the school boundary are established deliberately.",
              },
            ].map(({ icon: Icon, number, title, copy }) => (
              <div key={number} className="signup-step">
                <span>
                  <Icon className="size-5" />
                </span>
                <div>
                  <small>{number}</small>
                  <strong>{title}</strong>
                  <p>{copy}</p>
                </div>
              </div>
            ))}
          </div>
        </aside>

        <main className="signup-main">
          {receipt ? (
            <div className="contact-success" role="status">
              <span className="contact-success-icon">
                <Check className="size-7" />
              </span>
              <p className="resource-meta">Request received</p>
              <h2>Your school review is now in the queue.</h2>
              <p>
                Our onboarding team will review your school, confirm the plan and contact you before
                a tenant is provisioned. Keep this reference for follow-up.
              </p>
              <p className="mt-5 w-fit rounded-md border border-slate-200 bg-cool-mist px-3 py-2 font-mono text-xs text-navy-deep">
                {receipt.request_id}
              </p>
              <Button asChild className="mt-7 h-11 px-6">
                <Link href="/">
                  Back to home <ArrowRight className="size-4" />
                </Link>
              </Button>
            </div>
          ) : (
            <>
              <div className="max-w-2xl">
                <p className="resource-meta">School context</p>
                <h2 className="mt-3 text-4xl font-bold tracking-[-0.04em] text-navy-deep sm:text-5xl">
                  Help us understand the first move.
                </h2>
                <p className="mt-4 leading-7 text-slate-600">
                  This creates a review request only. We will confirm scope, onboarding and
                  commercial terms before anything is provisioned.
                </p>
              </div>
              <form onSubmit={(event) => void handleSubmit(event)} className="signup-form mt-10">
                <div
                  className="absolute -left-[10000px] top-auto size-px overflow-hidden"
                  aria-hidden="true"
                >
                  <Label htmlFor="website">Website</Label>
                  <Input id="website" name="website" tabIndex={-1} autoComplete="off" />
                </div>
                <div>
                  <Label htmlFor="schoolName" required>
                    School name
                  </Label>
                  <Input
                    id="schoolName"
                    name="schoolName"
                    required
                    placeholder="e.g. University Practice Senior High School"
                  />
                </div>
                <div className="grid gap-5 sm:grid-cols-2">
                  <div>
                    <Label htmlFor="adminName" required>
                      Administrator name
                    </Label>
                    <Input id="adminName" name="adminName" required placeholder="Your name" />
                  </div>
                  <div>
                    <Label htmlFor="email" required>
                      Work email
                    </Label>
                    <Input
                      id="email"
                      name="email"
                      type="email"
                      required
                      placeholder="admin@school.edu.gh"
                    />
                  </div>
                </div>
                <div className="grid gap-5 sm:grid-cols-2">
                  <div>
                    <Label htmlFor="phone">Phone</Label>
                    <Input id="phone" name="phone" type="tel" placeholder="+233 20 000 0000" />
                  </div>
                  <div>
                    <Label htmlFor="country">Country</Label>
                    <Select id="country" name="country" defaultValue="GH">
                      {countries.map((country) => (
                        <option key={country.code} value={country.code}>
                          {country.name}
                        </option>
                      ))}
                    </Select>
                  </div>
                </div>
                <div>
                  <Label htmlFor="plan" required>
                    Plan interest
                  </Label>
                  <Select id="plan" name="plan" defaultValue={defaultPlan} required>
                    {plans.map((plan) => (
                      <option key={plan.key} value={plan.key}>
                        {plan.name}
                      </option>
                    ))}
                  </Select>
                </div>
                <div>
                  <Label htmlFor="priorities">What should AuraEDU improve first?</Label>
                  <textarea
                    id="priorities"
                    name="priorities"
                    rows={4}
                    placeholder="Attendance, report cards, fees, parent communication…"
                    className="marketing-textarea"
                  />
                </div>
                <label className="flex items-start gap-3 rounded-xl bg-cool-mist p-4 text-sm leading-relaxed text-slate-600">
                  <input
                    name="acceptedTerms"
                    type="checkbox"
                    required
                    className="mt-1 size-4 rounded border-border accent-[var(--color-brand)]"
                  />
                  <span>
                    I confirm I am authorized to request onboarding for this school and agree that
                    AuraEDU may use these details to review and contact us about setup.
                  </span>
                </label>
                {error ? (
                  <p role="alert" className="text-sm font-medium text-destructive">
                    {error}
                  </p>
                ) : null}
                <Button
                  type="submit"
                  className="h-12 w-full"
                  disabled={submitting}
                  aria-busy={submitting}
                >
                  {submitting ? (
                    "Submitting securely…"
                  ) : (
                    <>
                      Submit for review <ArrowRight className="size-4" />
                    </>
                  )}
                </Button>
                <p className="flex items-center justify-center gap-2 text-center text-xs leading-relaxed text-slate-500">
                  <Sparkles className="size-3.5 text-teal-strong" />
                  No workspace, subscription or charge is created at this stage.
                </p>
              </form>
            </>
          )}
        </main>
      </div>
    </div>
  );
}
