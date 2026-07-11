import Link from "next/link";

function Logo() {
  return (
    <span className="flex items-center gap-2 font-display text-base font-extrabold text-foreground">
      <span className="grid size-5 place-items-center rounded bg-foreground" aria-hidden="true">
        <svg viewBox="0 0 16 12" className="w-3 text-primary">
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
    </span>
  );
}

const footerLinks = [
  { href: "/", label: "Home" },
  { href: "/pricing", label: "Pricing" },
  { href: "/about", label: "About" },
  { href: "/contact", label: "Contact" },
  { href: "/signup", label: "Sign up" },
];

export function SiteFooter() {
  return (
    <footer className="border-t border-border bg-background">
      <div className="mx-auto max-w-6xl px-6 py-10">
        <div className="flex flex-col items-start justify-between gap-6 sm:flex-row sm:items-center">
          <Logo />
          <nav aria-label="Footer" className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground">
            {footerLinks.map((l) => (
              <Link key={l.href} href={l.href} className="hover:text-foreground">
                {l.label}
              </Link>
            ))}
          </nav>
        </div>
        <div className="mt-8 border-t border-border pt-6 text-sm text-muted-foreground">
          <p className="font-mono">© {new Date().getFullYear()} AuraEDU · One platform, every school</p>
        </div>
      </div>
    </footer>
  );
}
