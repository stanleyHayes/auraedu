import Link from "next/link";
import Image from "next/image";
import { ArrowRight, ArrowUpRight } from "lucide-react";

const groups = [
  {
    title: "Platform",
    links: [
      ["/features", "Platform map"],
      ["/pricing", "Plans"],
      ["/signup", "Start a school"],
    ],
  },
  {
    title: "Company",
    links: [
      ["/about", "Why AuraEDU"],
      ["/blog", "Platform notes"],
      ["/contact", "Contact"],
    ],
  },
  {
    title: "Start",
    links: [
      ["/contact", "Talk through a workflow"],
      ["/signup", "Request onboarding"],
      [`${process.env.NEXT_PUBLIC_APP_URL ?? "https://app.auraedu.com"}/login`, "Sign in"],
    ],
  },
  {
    title: "Trust",
    links: [
      ["/security", "Security"],
      ["/privacy", "Privacy"],
      ["/accessibility", "Accessibility"],
    ],
  },
] as const;

export function SiteFooter() {
  return (
    <footer className="site-footer-shell">
      <div className="mx-auto max-w-7xl px-6 pt-16">
        <div className="footer-manifesto">
          <p>
            One school day.
            <br />
            <span>One dependable rhythm.</span>
          </p>
          <Link href="/signup" className="cta-primary">
            Start your school <ArrowRight className="size-4" />
          </Link>
        </div>
      </div>
      <div className="mx-auto grid max-w-7xl gap-12 px-6 py-16 sm:grid-cols-2 lg:grid-cols-5">
        <div className="md:pr-8">
          <Link href="/" className="block w-fit" aria-label="AuraEDU home">
            <Image
              src="/brand/auraedu-logo-light.svg"
              alt="AuraEDU"
              width={208}
              height={48}
              className="brand-lockup h-9 w-auto"
            />
          </Link>
          <p className="mt-5 max-w-md text-sm leading-6 text-slate-400">
            One configurable education operating system for school operations, learning, families,
            growth, trusted data and accountable AI.
          </p>
          <p className="mt-8 inline-flex items-center gap-2 rounded-full border border-white/10 px-3 py-2 text-[11px] font-semibold text-slate-300">
            <span className="size-1.5 rounded-full bg-lime-signal" />
            Platform foundations active
          </p>
        </div>
        {groups.map((group) => (
          <nav key={group.title} aria-label={group.title}>
            <p className="text-xs font-bold uppercase tracking-[0.16em] text-teal-bright">
              {group.title}
            </p>
            <ul className="mt-4 grid gap-3">
              {group.links.map(([href, label]) => (
                <li key={href}>
                  <Link
                    href={href}
                    className="inline-flex items-center gap-1.5 text-sm text-slate-300 hover:text-white"
                  >
                    {label}
                    <ArrowUpRight className="size-3.5" aria-hidden="true" />
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>
      <div className="border-t border-white/10">
        <div className="mx-auto flex max-w-7xl flex-col gap-2 px-6 py-6 text-xs text-slate-500 sm:flex-row sm:items-center sm:justify-between">
          <p>© {new Date().getFullYear()} AuraEDU</p>
          <p>The education operating system · Built for distinct schools</p>
        </div>
      </div>
    </footer>
  );
}
