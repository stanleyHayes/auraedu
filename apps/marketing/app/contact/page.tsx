"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { ArrowRight, Check, Clock3, Mail, MapPin, MessageCircle, ShieldCheck } from "lucide-react";
import { Button, Input, Label, Select } from "@auraedu/ui";
import { Eyebrow } from "@/components/brand-primitives";

const CONTACT_EMAIL = "hello@auraedu.com";

// Placeholder channels — replace with the confirmed business number and routed
// mailboxes before launch; they are marked as placeholders on the page.
const WHATSAPP_URL = "https://wa.me/233200000000";

const mailboxes = [
  {
    label: "Admissions & onboarding",
    email: "admissions@auraedu.com",
    note: "New schools, demos and rollout planning.",
  },
  {
    label: "Support",
    email: "support@auraedu.com",
    note: "Live schools needing a hand with daily work.",
  },
  {
    label: "Billing",
    email: "billing@auraedu.com",
    note: "Invoices, payments and commercial questions.",
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
  { code: "ZZ", name: "Other" },
];

function field(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value : "";
}

export default function ContactPage() {
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
      const response = await fetch("/api/contact", {
        method: "POST",
        headers: { "Content-Type": "application/json", "Idempotency-Key": requestKey.current },
        body: JSON.stringify({
          name: field(data, "name"),
          school: field(data, "school"),
          email: field(data, "email"),
          phone: field(data, "phone") || null,
          country: field(data, "country"),
          interest: field(data, "interest"),
          message: field(data, "message"),
          accepted_terms: data.get("acceptedTerms") === "on",
          website: field(data, "website"),
        }),
      });
      const result = (await response.json()) as {
        request_id?: string;
        submitted_at?: string;
        message?: string;
      };
      if (!response.ok || !result.request_id || !result.submitted_at) {
        throw new Error(result.message ?? "We could not accept the message. Please try again.");
      }
      setReceipt({ request_id: result.request_id, submitted_at: result.submitted_at });
    } catch (submissionError) {
      setError(
        submissionError instanceof Error
          ? submissionError.message
          : "We could not accept the message. Please try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="contact-stage">
      <div className="mx-auto grid min-h-[calc(100vh-72px)] max-w-[1440px] lg:grid-cols-[0.78fr_1.22fr]">
        <aside className="contact-aside text-white">
          <div>
            <Eyebrow inverse>Start a useful conversation</Eyebrow>
            <h1 className="mt-5 max-w-[10ch] text-balance text-[clamp(3.5rem,6vw,6.4rem)] font-bold leading-[0.88] tracking-[-0.06em]">
              Tell us where school gets <span className="text-teal-bright">hard to see.</span>
            </h1>
            <p className="mt-7 max-w-lg text-lg leading-8 text-slate-300">
              Bring the broken handoff, the repeated task or the decision that arrives without
              enough context. We will help you find the sensible first move.
            </p>
          </div>
          <div className="mt-14 grid gap-3">
            <div className="contact-assurance">
              <Clock3 className="size-5 text-lime-signal" />
              <div>
                <strong>A thoughtful first response</strong>
                <span>We reply with questions, not a generic sales sequence.</span>
              </div>
            </div>
            <div className="contact-assurance">
              <ShieldCheck className="size-5 text-lime-signal" />
              <div>
                <strong>No automatic provisioning</strong>
                <span>Your school stays in control of what happens next.</span>
              </div>
            </div>
            <div className="contact-assurance">
              <MapPin className="size-5 text-lime-signal" />
              <div>
                <strong>Built from Ghana, designed to travel</strong>
                <span>Local school realities shape the platform.</span>
              </div>
            </div>
          </div>
          <a
            href={WHATSAPP_URL}
            target="_blank"
            rel="noreferrer"
            className="mt-12 flex w-fit items-center gap-4 rounded-xl border border-white/15 bg-white/5 px-5 py-4 transition-colors hover:border-lime-signal/50"
          >
            <span className="grid size-11 place-items-center rounded-lg bg-lime-signal text-navy-deep">
              <MessageCircle className="size-5" aria-hidden="true" />
            </span>
            <span>
              <strong className="block text-sm font-bold">Chat on WhatsApp</strong>
              <span className="mt-0.5 block text-xs text-slate-400">
                Fastest for a quick question (placeholder number — official line announced at
                launch)
              </span>
            </span>
          </a>
          <a
            href={`mailto:${CONTACT_EMAIL}`}
            className="mt-6 inline-flex items-center gap-3 text-sm font-semibold text-white"
          >
            <Mail className="size-4 text-teal-bright" />
            {CONTACT_EMAIL}
          </a>
        </aside>

        <div className="contact-main">
          {receipt ? (
            <div className="contact-success" role="status">
              <span className="contact-success-icon">
                <Check className="size-7" />
              </span>
              <p className="resource-meta">Message received</p>
              <h2>Your message is in the review queue.</h2>
              <p>
                A real person will read this and reply to the email you gave us. Keep this reference
                if you follow up.
              </p>
              <p className="mt-5 w-fit rounded-md border border-slate-200 bg-cool-mist px-3 py-2 font-mono text-xs text-navy-deep">
                {receipt.request_id}
              </p>
              <Button asChild className="mt-7 h-11 px-6">
                <Link href="/features">
                  Explore the platform <ArrowRight className="size-4" />
                </Link>
              </Button>
            </div>
          ) : (
            <>
              <div className="max-w-2xl">
                <p className="resource-meta">A few useful details</p>
                <h2 className="mt-3 text-4xl font-bold tracking-[-0.04em] text-navy-deep sm:text-5xl">
                  What should change first?
                </h2>
                <p className="mt-4 leading-7 text-slate-600">
                  This sends your message straight to our review queue—securely, and only once, no
                  matter how many times you tap submit.
                </p>
              </div>
              <form onSubmit={(event) => void handleSubmit(event)} className="contact-form mt-10">
                <div
                  className="absolute -left-[10000px] top-auto size-px overflow-hidden"
                  aria-hidden="true"
                >
                  <Label htmlFor="website">Website</Label>
                  <Input id="website" name="website" tabIndex={-1} autoComplete="off" />
                </div>
                <div className="grid gap-5 sm:grid-cols-2">
                  <div>
                    <Label htmlFor="name" required>
                      Name
                    </Label>
                    <Input id="name" name="name" required placeholder="Your name" />
                  </div>
                  <div>
                    <Label htmlFor="school" required>
                      School
                    </Label>
                    <Input id="school" name="school" required placeholder="School name" />
                  </div>
                </div>
                <div className="grid gap-5 sm:grid-cols-2">
                  <div>
                    <Label htmlFor="email" required>
                      Work email
                    </Label>
                    <Input
                      id="email"
                      name="email"
                      type="email"
                      required
                      placeholder="you@school.edu.gh"
                    />
                  </div>
                  <div>
                    <Label htmlFor="phone">Phone</Label>
                    <Input id="phone" name="phone" type="tel" placeholder="+233 20 000 0000" />
                  </div>
                </div>
                <div className="grid gap-5 sm:grid-cols-2">
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
                  <div>
                    <Label htmlFor="interest" required>
                      I am interested in
                    </Label>
                    <Select id="interest" name="interest" defaultValue="demo" required>
                      <option value="demo">Book a demo</option>
                      <option value="pricing">Pricing question</option>
                      <option value="support">Support</option>
                      <option value="other">Other</option>
                    </Select>
                  </div>
                </div>
                <div>
                  <Label htmlFor="message" required>
                    What is happening today?
                  </Label>
                  <textarea
                    id="message"
                    name="message"
                    required
                    rows={5}
                    minLength={10}
                    placeholder="Tell us about the workflow, who it affects and what a better day would look like…"
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
                    I agree that AuraEDU may use these details to review and respond to this
                    enquiry.
                  </span>
                </label>
                {error ? (
                  <p role="alert" className="text-sm font-medium text-destructive">
                    {error}
                  </p>
                ) : null}
                <div className="flex flex-col gap-4 border-t border-slate-200 pt-6 sm:flex-row sm:items-center sm:justify-between">
                  <Button
                    type="submit"
                    className="h-12 px-7"
                    disabled={submitting}
                    aria-busy={submitting}
                  >
                    {submitting ? (
                      "Sending securely…"
                    ) : (
                      <>
                        Send my message <ArrowRight className="size-4" />
                      </>
                    )}
                  </Button>
                  <p className="max-w-xs text-xs leading-5 text-slate-500">
                    Delivered once, even on a flaky connection. Nothing is provisioned or charged.
                  </p>
                </div>
              </form>

              <div className="mt-10 max-w-[760px]">
                <p className="resource-meta">Or reach the right desk directly</p>
                <div className="mt-5 grid gap-3 sm:grid-cols-3">
                  {mailboxes.map((mailbox) => (
                    <a
                      key={mailbox.email}
                      href={`mailto:${mailbox.email}`}
                      className="group rounded-xl border border-slate-200 bg-white p-4 transition-colors hover:border-cobalt"
                    >
                      <strong className="block text-sm font-bold text-navy-deep group-hover:text-cobalt">
                        {mailbox.label}
                      </strong>
                      <span className="mt-1 block text-xs leading-5 text-slate-500">
                        {mailbox.note}
                      </span>
                      <span className="mt-3 inline-flex items-center gap-1.5 text-xs font-semibold text-teal-strong">
                        <Mail className="size-3.5" aria-hidden="true" />
                        {mailbox.email}
                      </span>
                    </a>
                  ))}
                </div>
                <p className="mt-3 text-[11px] leading-5 text-slate-400">
                  Department mailboxes are placeholders pending final routing — {CONTACT_EMAIL}{" "}
                  always reaches us.
                </p>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
