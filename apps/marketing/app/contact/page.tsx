"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowRight, Check, Clock3, Mail, MapPin, ShieldCheck } from "lucide-react";
import { Button, Input, Label, Select } from "@auraedu/ui";
import { Eyebrow } from "@/components/brand-primitives";

const CONTACT_EMAIL = "hello@auraedu.com";
const interestLabels: Record<string, string> = {
  demo: "Book a demo",
  pricing: "Pricing question",
  support: "Support",
  other: "Other",
};
function field(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value : "";
}

export default function ContactPage() {
  const [submitted, setSubmitted] = useState(false);
  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const name = field(data, "name");
    const school = field(data, "school");
    const email = field(data, "email");
    const phone = field(data, "phone");
    const interest = field(data, "interest");
    const message = field(data, "message");
    const subject = encodeURIComponent(
      `[${interestLabels[interest] ?? "Contact"}] ${school} — ${name}`,
    );
    const body = encodeURIComponent(
      `${message}\n\n—\nName: ${name}\nSchool: ${school}\nEmail: ${email}\nPhone: ${phone || "—"}`,
    );
    window.location.href = `mailto:${CONTACT_EMAIL}?subject=${subject}&body=${body}`;
    setSubmitted(true);
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
            href={`mailto:${CONTACT_EMAIL}`}
            className="mt-12 inline-flex items-center gap-3 text-sm font-semibold text-white"
          >
            <Mail className="size-4 text-teal-bright" />
            {CONTACT_EMAIL}
          </a>
        </aside>

        <div className="contact-main">
          {submitted ? (
            <div className="contact-success" role="status">
              <span className="contact-success-icon">
                <Check className="size-7" />
              </span>
              <p className="resource-meta">Message prepared</p>
              <h2>Your email is ready to send.</h2>
              <p>
                Your mail app should have opened with the details pre-filled. Nothing leaves your
                device until you press send there.
              </p>
              <div className="mt-7 flex flex-wrap gap-3">
                <a href={`mailto:${CONTACT_EMAIL}`} className="cta-solid">
                  Open email again <ArrowRight className="size-4" />
                </a>
                <Link href="/features" className="cta-outline">
                  Explore the platform
                </Link>
              </div>
            </div>
          ) : (
            <>
              <div className="max-w-2xl">
                <p className="resource-meta">A few useful details</p>
                <h2 className="mt-3 text-4xl font-bold tracking-[-0.04em] text-navy-deep sm:text-5xl">
                  What should change first?
                </h2>
                <p className="mt-4 leading-7 text-slate-600">
                  This opens a prepared email so your message stays transparent and under your
                  control.
                </p>
              </div>
              <form onSubmit={handleSubmit} className="contact-form mt-10">
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
                <div>
                  <Label htmlFor="interest">I am interested in</Label>
                  <Select id="interest" name="interest" defaultValue="">
                    <option value="" disabled>
                      Select an option
                    </option>
                    <option value="demo">Book a demo</option>
                    <option value="pricing">Pricing question</option>
                    <option value="support">Support</option>
                    <option value="other">Other</option>
                  </Select>
                </div>
                <div>
                  <Label htmlFor="message" required>
                    What is happening today?
                  </Label>
                  <textarea
                    id="message"
                    name="message"
                    required
                    rows={6}
                    placeholder="Tell us about the workflow, who it affects and what a better day would look like…"
                    className="marketing-textarea"
                  />
                </div>
                <div className="flex flex-col gap-4 border-t border-slate-200 pt-6 sm:flex-row sm:items-center sm:justify-between">
                  <Button type="submit" className="h-12 px-7">
                    Prepare my message <ArrowRight className="size-4" />
                  </Button>
                  <p className="max-w-xs text-xs leading-5 text-slate-500">
                    Nothing is sent automatically. Your mail app opens for final review.
                  </p>
                </div>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
