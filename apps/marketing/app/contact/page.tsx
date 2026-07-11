"use client";

import { useState } from "react";
import { Button, Input, Label, Select } from "@auraedu/ui";

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

export default function ContactPage() {
  const [submitted, setSubmitted] = useState(false);

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitted(true);
  }

  return (
    <div className="mx-auto max-w-2xl px-6 py-16">
      <div className="text-center">
        <Eyebrow>Contact</Eyebrow>
        <h1 className="mt-4 text-balance font-display text-3xl font-extrabold tracking-tight sm:text-4xl lg:text-5xl">
          Talk to the AuraEDU team.
        </h1>
        <p className="mx-auto mt-3 max-w-[56ch] text-muted-foreground">
          Ask about pricing, onboarding, or a demo for your school. We reply within one working day.
        </p>
      </div>

      <div className="mt-10 rounded-lg border border-border bg-surface p-6 sm:p-8">
        {submitted ? (
          <div className="py-8 text-center" role="status">
            <div className="mx-auto grid size-12 place-items-center rounded-full bg-[var(--color-brand-tint)] text-primary">
              <Tick className="w-6" />
            </div>
            <h2 className="mt-4 font-display text-xl font-extrabold">Message sent</h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Thank you for reaching out. We will get back to you shortly.
            </p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-5">
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
            <div>
              <Label htmlFor="email" required>
                Email
              </Label>
              <Input id="email" name="email" type="email" required placeholder="you@school.edu.gh" />
            </div>
            <div>
              <Label htmlFor="phone">Phone</Label>
              <Input id="phone" name="phone" type="tel" placeholder="+233 20 000 0000" />
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
                Message
              </Label>
              <textarea
                id="message"
                name="message"
                required
                rows={4}
                placeholder="How can we help?"
                className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--surface)] px-3.5 py-2.5 text-sm text-[var(--foreground)] shadow-sm placeholder:text-[var(--muted-foreground)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
              />
            </div>
            <Button type="submit" className="h-11 w-full sm:w-auto sm:px-8">
              Send message
            </Button>
          </form>
        )}
      </div>
    </div>
  );
}
