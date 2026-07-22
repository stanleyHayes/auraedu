import type { ReactNode } from "react";

export function BrandTick({ className = "" }: { className?: string }) {
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

export function Eyebrow({ children, inverse = false }: { children: ReactNode; inverse?: boolean }) {
  return (
    <span
      className={`inline-flex items-center gap-2.5 font-mono text-xs uppercase tracking-[0.16em] ${
        inverse ? "text-ink-200" : "text-muted-foreground"
      }`}
    >
      <BrandTick className="w-3.5 text-primary" />
      {children}
    </span>
  );
}
