"use client";

import Link from "next/link";
import { Button, ThemeToggle } from "@auraedu/ui";

const nav = [
  { href: "/", label: "Home" },
  { href: "/#features", label: "Features" },
  { href: "/pricing", label: "Pricing" },
  { href: "/about", label: "About" },
  { href: "/contact", label: "Contact" },
];

function Logo() {
  return (
    <Link
      href="/"
      className="flex items-center gap-2.5 font-display text-xl font-extrabold tracking-tight"
    >
      <span className="grid size-6 place-items-center rounded-md bg-foreground" aria-hidden="true">
        <svg viewBox="0 0 16 12" className="w-3.5 text-primary">
          <path
            d="M1 6.5 5.2 10.5 15 1"
            fill="none"
            stroke="currentColor"
            strokeWidth={2.4}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </span>
      AuraEDU
    </Link>
  );
}

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-30 border-b border-border bg-background/85 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-5 px-6">
        <Logo />
        <nav className="ml-3 hidden gap-6 md:flex" aria-label="Primary">
          {nav.map((n) => (
            <Link
              key={n.href}
              href={n.href}
              className="text-sm text-muted-foreground transition-colors hover:text-foreground"
            >
              {n.label}
            </Link>
          ))}
        </nav>
        <span className="flex-1" />
        <ThemeToggle />
        <Button asChild>
          <Link href="/signup">Sign up your school</Link>
        </Button>
      </div>
    </header>
  );
}
