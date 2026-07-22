"use client";

import { AlertTriangle, RotateCcw } from "lucide-react";
import { Button, Watermark } from "@auraedu/ui";

export function PortalRouteError({ reset }: { reset: () => void }) {
  return (
    <section role="alert" className="portal-hero card relative isolate overflow-hidden p-7 sm:p-10">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.035]">
        Retry
      </Watermark>
      <span className="grid size-14 place-items-center rounded-2xl bg-[var(--color-warn)]/12 text-[var(--color-warn)]">
        <AlertTriangle className="size-7" aria-hidden="true" />
      </span>
      <p className="mt-6 font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--primary)]">
        Workspace interrupted
      </p>
      <h2 className="mt-2 max-w-xl font-heading text-2xl font-extrabold tracking-tight sm:text-3xl">
        This page could not finish loading.
      </h2>
      <p className="mt-3 max-w-2xl text-sm leading-6 text-[var(--muted-foreground)]">
        Your saved records have not been changed. Retry the secure request, or return to this page
        after checking your connection.
      </p>
      <Button className="mt-6" onClick={reset}>
        <RotateCcw className="size-4" aria-hidden="true" />
        Try again
      </Button>
    </section>
  );
}
