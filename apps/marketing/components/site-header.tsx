"use client";

import { Button, ThemeToggle } from "@auraedu/ui";

const nav = [
  { href: "#schools", label: "Schools" },
  { href: "#modules", label: "Modules" },
  { href: "#roles", label: "Who it's for" },
];

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-20 border-b border-border bg-background/85 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-5 px-6">
        <a href="#top" className="flex items-center gap-2.5 font-display text-xl font-extrabold tracking-tight">
          <span className="grid size-6 place-items-center rounded-md bg-foreground" aria-hidden="true">
            <svg viewBox="0 0 16 12" className="w-3.5 text-primary">
              <path d="M1 6.5 5.2 10.5 15 1" fill="none" stroke="currentColor" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </span>
          AuraEDU
        </a>
        <nav className="ml-3 hidden gap-6 md:flex" aria-label="Primary">
          {nav.map((n) => (
            <a key={n.href} href={n.href} className="text-sm text-muted-foreground transition-colors hover:text-foreground">
              {n.label}
            </a>
          ))}
        </nav>
        <span className="flex-1" />
        <ThemeToggle />
        <Button onClick={() => (window.location.hash = "#join")}>Sign up your school</Button>
      </div>
    </header>
  );
}
