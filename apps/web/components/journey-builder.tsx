"use client";

import { useId, useState } from "react";
import { ArrowDown, GitBranch, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";

interface TemplateOption {
  id: string;
  name: string;
  channel: string;
}

interface StepDraft {
  key: string;
  channel: "email" | "sms" | "whatsapp" | "in_app";
  condition: "always" | "equals" | "not_equals";
}

const events = [
  ["lead.created.v1", "New enquiry"],
  ["lead.interaction_created.v1", "Prospect replied"],
  ["lead.scored.v1", "Lead score changed"],
  ["application.started.v1", "Application started"],
  ["application.submitted.v1", "Application submitted"],
  ["application.admitted.v1", "Applicant admitted"],
  ["offer.issued.v1", "Offer issued"],
  ["offer.accepted.v1", "Offer accepted"],
] as const;

const conditionFields = [
  "source",
  "stage",
  "campaign_id",
  "programme_id",
  "intake_id",
  "score",
  "confidence",
  "channel",
  "direction",
];

export function JourneyBuilder({
  templates,
  action,
}: {
  templates: TemplateOption[];
  action: (formData: FormData) => Promise<void>;
}) {
  const id = useId();
  const [steps, setSteps] = useState<StepDraft[]>([
    { key: `${id}-1`, channel: "email", condition: "always" },
  ]);

  function updateStep(index: number, patch: Partial<StepDraft>) {
    setSteps((current) =>
      current.map((step, position) => (position === index ? { ...step, ...patch } : step)),
    );
  }

  return (
    <form action={action} className="space-y-6">
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Journey name">
          <input
            required
            minLength={3}
            maxLength={160}
            name="name"
            className="journey-input"
            placeholder="Application completion support"
          />
        </Field>
        <Field label="Starts when">
          <select required name="trigger_event" className="journey-input">
            {events.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Institution timezone">
          <input required name="timezone" defaultValue="Africa/Accra" className="journey-input" />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Quiet from">
            <input type="time" name="quiet_start" defaultValue="20:00" className="journey-input" />
          </Field>
          <Field label="Quiet until">
            <input type="time" name="quiet_end" defaultValue="07:00" className="journey-input" />
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-3 md:col-span-2">
          <Field label="Frequency window">
            <div className="relative">
              <input
                required
                type="number"
                min={1}
                max={720}
                name="frequency_window_hours"
                defaultValue={168}
                className="journey-input pr-16"
              />
              <span className="pointer-events-none absolute right-3 top-3 text-xs text-muted-foreground">
                hours
              </span>
            </div>
          </Field>
          <Field label="Maximum messages">
            <input
              required
              type="number"
              min={1}
              max={100}
              name="frequency_limit"
              defaultValue={3}
              className="journey-input"
            />
          </Field>
        </div>
      </div>

      <fieldset className="rounded-xl border border-border bg-background/60 p-4">
        <legend className="px-2 text-sm font-semibold">Stop this journey when</legend>
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          {events.map(([value, label]) => (
            <label
              key={value}
              className="flex min-h-11 items-center gap-2 rounded-lg border border-border px-3 text-sm transition hover:border-primary/50 hover:bg-primary/5"
            >
              <input
                type="checkbox"
                name="cancel_on_events"
                value={value}
                className="size-4 accent-primary"
              />
              {label}
            </label>
          ))}
        </div>
      </fieldset>

      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 className="font-heading text-lg font-bold">Journey steps</h3>
            <p className="text-sm text-muted-foreground">
              Delays accumulate from the trigger. Conditions create safe, deterministic branches.
            </p>
          </div>
          <Button
            type="button"
            variant="secondary"
            disabled={steps.length >= 10}
            onClick={() =>
              setSteps((current) => [
                ...current,
                { key: `${id}-${Date.now()}`, channel: "email", condition: "always" },
              ])
            }
          >
            <Plus className="mr-2 size-4" /> Add step
          </Button>
        </div>

        {steps.map((step, index) => {
          const channelTemplates = templates.filter(
            (template) => template.channel === step.channel,
          );
          return (
            <div key={step.key}>
              {index > 0 ? (
                <div className="flex h-8 items-center pl-8 text-primary">
                  <ArrowDown className="size-4" />
                </div>
              ) : null}
              <article className="relative overflow-hidden rounded-xl border border-border bg-surface p-5 shadow-sm">
                <div className="absolute inset-y-0 left-0 w-1 bg-gradient-to-b from-primary to-[var(--portal-accent)]" />
                <div className="mb-4 flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <span className="grid size-9 place-items-center rounded-full bg-primary/10 text-sm font-bold text-primary">
                      {index + 1}
                    </span>
                    <div>
                      <h4 className="font-semibold">Communication step</h4>
                      <p className="text-xs text-muted-foreground">
                        Consent and tenant features are checked again at delivery.
                      </p>
                    </div>
                  </div>
                  {steps.length > 1 ? (
                    <button
                      type="button"
                      onClick={() =>
                        setSteps((current) => current.filter((_, position) => position !== index))
                      }
                      className="rounded-md p-2 text-muted-foreground transition hover:bg-destructive/10 hover:text-destructive"
                      aria-label={`Remove step ${index + 1}`}
                    >
                      <Trash2 className="size-4" />
                    </button>
                  ) : null}
                </div>
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                  <Field label="Channel">
                    <select
                      name="step_channel"
                      value={step.channel}
                      onChange={(event) =>
                        updateStep(index, { channel: event.target.value as StepDraft["channel"] })
                      }
                      className="journey-input"
                    >
                      <option value="email">Email</option>
                      <option value="sms">SMS</option>
                      <option value="whatsapp">WhatsApp</option>
                      <option value="in_app">In-app</option>
                    </select>
                  </Field>
                  <Field label="Approved template">
                    <select
                      required
                      name="step_template_id"
                      className="journey-input"
                      key={step.channel}
                      defaultValue=""
                    >
                      <option value="" disabled>
                        {channelTemplates.length ? "Choose template" : "No matching template"}
                      </option>
                      {channelTemplates.map((template) => (
                        <option key={template.id} value={template.id}>
                          {template.name}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Wait after previous step">
                    <div className="relative">
                      <input
                        required
                        type="number"
                        min={0}
                        max={129600}
                        name="step_delay_minutes"
                        defaultValue={index === 0 ? 0 : 1440}
                        className="journey-input pr-16"
                      />
                      <span className="pointer-events-none absolute right-3 top-3 text-xs text-muted-foreground">
                        minutes
                      </span>
                    </div>
                  </Field>
                  <Field label="Branch rule">
                    <select
                      name="step_condition_operator"
                      value={step.condition}
                      onChange={(event) =>
                        updateStep(index, {
                          condition: event.target.value as StepDraft["condition"],
                        })
                      }
                      className="journey-input"
                    >
                      <option value="always">Always continue</option>
                      <option value="equals">Field equals</option>
                      <option value="not_equals">Field does not equal</option>
                    </select>
                  </Field>
                  {step.condition !== "always" ? (
                    <div className="grid gap-4 md:col-span-2 md:grid-cols-2 lg:col-span-4">
                      <Field label="Approved event field">
                        <select name="step_condition_field" className="journey-input">
                          {conditionFields.map((field) => (
                            <option key={field} value={field}>
                              {field.replaceAll("_", " ")}
                            </option>
                          ))}
                        </select>
                      </Field>
                      <Field label="Expected value">
                        <input
                          required
                          maxLength={160}
                          name="step_condition_value"
                          className="journey-input"
                          placeholder="submitted"
                        />
                      </Field>
                    </div>
                  ) : (
                    <>
                      <input type="hidden" name="step_condition_field" value="" />
                      <input type="hidden" name="step_condition_value" value="" />
                    </>
                  )}
                </div>
              </article>
            </div>
          );
        })}
      </div>

      <div className="flex flex-col gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3 text-sm">
          <ShieldCheck className="mt-0.5 size-5 shrink-0 text-primary" />
          <p>
            <strong>Review boundary:</strong> saving creates a draft. A different authorised person
            must activate it.
          </p>
        </div>
        <Button type="submit" disabled={templates.length === 0} className="shrink-0">
          <GitBranch className="mr-2 size-4" /> Save journey draft
        </Button>
      </div>
    </form>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="text-sm font-semibold text-foreground">
      {label}
      {children}
    </label>
  );
}
