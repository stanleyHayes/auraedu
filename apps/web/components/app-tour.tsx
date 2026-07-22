"use client";

import * as React from "react";
import { ArrowLeft, ArrowRight, Compass, X } from "lucide-react";

export const REPLAY_TOUR_EVENT = "auraedu:replay-tour";

export function dispatchReplayTour() {
  window.dispatchEvent(new CustomEvent(REPLAY_TOUR_EVENT));
}

interface TourStep {
  selector: string | string[];
  eyebrow: string;
  title: string;
  description: string;
}

const STEPS: TourStep[] = [
  {
    selector: ["desktop-navigation", "mobile-navigation"],
    eyebrow: "Your workspace",
    title: "Move through school work",
    description:
      "The navigation shows only the modules enabled for this school and permitted for your role.",
  },
  {
    selector: "page-header",
    eyebrow: "Page context",
    title: "Know where you are",
    description:
      "Every page begins with its purpose, current context and a help guide you can also listen to.",
  },
  {
    selector: "primary-actions",
    eyebrow: "Focused action",
    title: "Do the next important thing",
    description:
      "When a page has a primary action, it appears here so the workflow stays clear and deliberate.",
  },
  {
    selector: "theme-toggle",
    eyebrow: "Personal comfort",
    title: "Choose your viewing mode",
    description:
      "Switch between light and dark themes. AuraEDU remembers the choice and respects reduced motion.",
  },
  {
    selector: "notifications",
    eyebrow: "Stay current",
    title: "See school updates",
    description:
      "Tenant-scoped alerts, delivery outcomes and important school notices appear in this space.",
  },
  {
    selector: "user-menu",
    eyebrow: "Your account",
    title: "Profile, guide and sign out",
    description:
      "Open the account menu to revisit this tour, read the full guide or end your secure session.",
  },
];

interface AppTourProps {
  tenantId: string;
  userId: string;
  mode: string;
  autoStart?: boolean;
}

type NavigatorWithConnection = Navigator & { connection?: { saveData?: boolean } };

export function AppTour({ tenantId, userId, mode, autoStart = false }: AppTourProps) {
  const [open, setOpen] = React.useState(false);
  const [stepIndex, setStepIndex] = React.useState(0);
  const [rect, setRect] = React.useState<DOMRect | null>(null);
  const dialogRef = React.useRef<HTMLDivElement>(null);
  const completionKey = `auraedu-tour-complete:${mode}:${tenantId}:${userId}`;

  const findVisibleTarget = React.useCallback((step: TourStep): HTMLElement | null => {
    const selectors = Array.isArray(step.selector) ? step.selector : [step.selector];
    for (const selector of selectors) {
      const nodes = document.querySelectorAll<HTMLElement>(`[data-tour="${selector}"]`);
      for (const node of nodes) {
        const bounds = node.getBoundingClientRect();
        const style = window.getComputedStyle(node);
        if (
          bounds.width > 0 &&
          bounds.height > 0 &&
          style.display !== "none" &&
          style.visibility !== "hidden"
        )
          return node;
      }
    }
    return null;
  }, []);

  const moveToAvailableStep = React.useCallback(
    (requested: number, direction = 1) => {
      let next = requested;
      while (next >= 0 && next < STEPS.length && !findVisibleTarget(STEPS[next]!))
        next += direction;
      if (next < 0 || next >= STEPS.length) return null;
      setStepIndex(next);
      return next;
    },
    [findVisibleTarget],
  );

  const finish = React.useCallback(() => {
    window.localStorage.setItem(completionKey, "1");
    setOpen(false);
  }, [completionKey]);

  React.useEffect(() => {
    function replay() {
      setOpen(true);
      setStepIndex(0);
    }
    window.addEventListener(REPLAY_TOUR_EVENT, replay);
    return () => window.removeEventListener(REPLAY_TOUR_EVENT, replay);
  }, []);

  React.useEffect(() => {
    if (!autoStart || window.localStorage.getItem(completionKey)) return;
    if ((navigator as NavigatorWithConnection).connection?.saveData) return;
    const timer = window.setTimeout(() => {
      setOpen(true);
      setStepIndex(0);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [autoStart, completionKey]);

  React.useEffect(() => {
    if (!open) return;
    const available = moveToAvailableStep(stepIndex, 1);
    if (available === null) {
      finish();
      return;
    }
    const availableIndex = available;
    const availableStep = STEPS.at(availableIndex);
    if (!availableStep) {
      finish();
      return;
    }
    const measuredStep: TourStep = availableStep;

    let frame = 0;
    function measure() {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => {
        const target = findVisibleTarget(measuredStep);
        if (!target) return;
        const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        target.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "nearest" });
        setRect(target.getBoundingClientRect());
      });
    }
    measure();
    window.addEventListener("resize", measure);
    window.addEventListener("scroll", measure, true);
    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener("resize", measure);
      window.removeEventListener("scroll", measure, true);
    };
  }, [findVisibleTarget, finish, moveToAvailableStep, open, stepIndex]);

  React.useEffect(() => {
    if (!open) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") finish();
      if (event.key === "ArrowRight" && moveToAvailableStep(stepIndex + 1, 1) === null) finish();
      if (event.key === "ArrowLeft") moveToAvailableStep(stepIndex - 1, -1);
    }
    window.addEventListener("keydown", onKeyDown);
    dialogRef.current?.focus();
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [finish, moveToAvailableStep, open, stepIndex]);

  if (!open || !rect) return null;

  const step = STEPS[stepIndex]!;
  const margin = 16;
  const panelWidth = Math.min(360, window.innerWidth - margin * 2);
  const targetCentre = rect.left + rect.width / 2;
  const left = Math.max(
    margin,
    Math.min(targetCentre - panelWidth / 2, window.innerWidth - panelWidth - margin),
  );
  const fitsBelow = rect.bottom + 236 < window.innerHeight;
  const top = fitsBelow ? rect.bottom + 16 : Math.max(margin, rect.top - 220);
  const hasPrevious = STEPS.slice(0, stepIndex).some((candidate) => findVisibleTarget(candidate));
  const hasNext = STEPS.slice(stepIndex + 1).some((candidate) => findVisibleTarget(candidate));

  return (
    <div
      ref={dialogRef}
      role="dialog"
      aria-modal="true"
      aria-label="AuraEDU tour"
      tabIndex={-1}
      className="fixed inset-0 z-[300] outline-none"
    >
      <div
        aria-hidden="true"
        className="fixed rounded-xl border-2 border-[var(--primary)] bg-[color-mix(in_oklch,var(--primary)_10%,transparent)] transition-[left,top,width,height] duration-200 motion-reduce:transition-none"
        style={{
          left: rect.left - 6,
          top: rect.top - 6,
          width: rect.width + 12,
          height: rect.height + 12,
          boxShadow: "0 0 0 9999px rgba(4, 6, 15, 0.64)",
        }}
      />
      <section
        className="fixed overflow-hidden rounded-2xl border border-white/15 bg-[var(--color-navy)] text-white shadow-2xl motion-safe:animate-[slide-up_220ms_var(--ease-out-quart)]"
        style={{ left, top, width: panelWidth }}
      >
        <div className="h-1 bg-gradient-to-r from-[var(--color-brand)] via-[var(--color-teal-bright)] to-[var(--color-signal)]" />
        <div className="p-5">
          <div className="flex items-start justify-between gap-4">
            <span className="grid size-9 place-items-center rounded-xl bg-white/10 text-[var(--color-signal)]">
              <Compass className="size-[18px]" aria-hidden="true" />
            </span>
            <button
              type="button"
              onClick={finish}
              aria-label="Close tour and do not show again"
              className="grid size-8 place-items-center rounded-full text-white/60 hover:bg-white/10 hover:text-white"
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          </div>
          <p className="mt-4 font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--color-teal-bright)]">
            {step.eyebrow} · {stepIndex + 1} of {STEPS.length}
          </p>
          <h2 className="mt-1 font-heading text-xl font-extrabold">{step.title}</h2>
          <p className="mt-2 text-sm leading-6 text-white/70">{step.description}</p>
          <div className="mt-5 flex items-center justify-between gap-3">
            <button
              type="button"
              onClick={() => moveToAvailableStep(stepIndex - 1, -1)}
              disabled={!hasPrevious}
              className="inline-flex h-9 items-center gap-2 rounded-full px-3 text-xs font-bold text-white/70 hover:bg-white/10 disabled:opacity-30"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              Back
            </button>
            <button
              type="button"
              onClick={() => (hasNext ? moveToAvailableStep(stepIndex + 1, 1) : finish())}
              className="inline-flex h-10 items-center gap-2 rounded-full bg-[var(--color-signal)] px-5 text-xs font-black text-[var(--color-navy)] hover:brightness-105"
            >
              {hasNext ? "Next" : "Finish"}
              <ArrowRight className="size-4" aria-hidden="true" />
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
