"use client";

import Link from "next/link";
import Image from "next/image";
import { usePathname } from "next/navigation";
import { ArrowRight, LogIn, Menu as MenuIcon } from "lucide-react";

const nav = [
  { href: "/features", label: "Platform" },
  { href: "/#how-it-works", label: "How it works" },
  { href: "/pricing", label: "Pricing" },
  { href: "/blog", label: "Resources" },
  { href: "/about", label: "About" },
  { href: "/contact", label: "Contact" },
];

const appUrl = process.env.NEXT_PUBLIC_APP_URL ?? "https://app.auraedu.com";

function Logo() {
  return (
    <Link href="/" className="block shrink-0" aria-label="AuraEDU home">
      <Image
        src="/brand/auraedu-logo-light.svg"
        alt="AuraEDU"
        width={208}
        height={48}
        priority
        className="brand-lockup h-8 w-auto"
      />
    </Link>
  );
}

export function SiteHeader() {
  const pathname = usePathname();
  return (
    <header className="site-header-shell sticky top-0 z-50 border-b">
      <div className="mx-auto flex h-[72px] max-w-[1440px] items-center gap-6 px-6 sm:px-10 lg:px-16">
        <Logo />
        <nav className="ml-auto hidden items-center gap-7 lg:flex" aria-label="Primary">
          {nav.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              aria-current={pathname === item.href ? "page" : undefined}
              className="header-nav-link"
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <Link
          href={`${appUrl}/login`}
          className="ml-2 hidden items-center gap-1.5 text-xs font-semibold text-slate-300 hover:text-white xl:flex"
        >
          Sign in <LogIn className="size-3.5" aria-hidden="true" />
        </Link>
        <Link
          href="/signup"
          className="cta-primary ml-auto !hidden min-h-10 px-4 py-2 text-xs sm:!inline-flex lg:ml-4"
        >
          Start your school <ArrowRight className="size-3.5" aria-hidden="true" />
        </Link>
        <details className="group relative ml-auto lg:hidden">
          <summary className="grid size-11 cursor-pointer list-none place-items-center rounded-lg border border-white/20 text-white marker:content-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-lime-signal">
            <MenuIcon className="size-5" aria-hidden="true" />
            <span className="sr-only">Open navigation</span>
          </summary>
          <nav
            className="absolute right-0 top-14 z-50 grid min-w-64 gap-1 rounded-xl border border-white/10 bg-navy-deep p-2 shadow-2xl"
            aria-label="Mobile primary"
          >
            {nav.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                aria-current={pathname === item.href ? "page" : undefined}
                className="rounded-lg px-4 py-3 text-sm font-medium text-slate-300 hover:bg-white/10 hover:text-white aria-[current=page]:bg-white/10 aria-[current=page]:text-white"
              >
                {item.label}
              </Link>
            ))}
            <Link
              href={`${appUrl}/login`}
              className="flex items-center justify-between rounded-lg px-4 py-3 text-sm font-medium text-slate-300 hover:bg-white/10 hover:text-white"
            >
              Sign in <LogIn className="size-4" />
            </Link>
            <Link
              href="/signup"
              className="mt-1 flex items-center justify-between rounded-lg bg-lime-signal px-4 py-3 text-sm font-bold text-navy-deep"
            >
              Start your school <ArrowRight className="size-4" aria-hidden="true" />
            </Link>
          </nav>
        </details>
      </div>
    </header>
  );
}
