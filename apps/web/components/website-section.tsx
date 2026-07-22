import Link from "next/link";
import { Button, Reveal } from "@auraedu/ui";
import { ArrowRight, Mail, Phone, MapPin, Sparkles, type LucideIcon } from "lucide-react";
import type { WebsiteSection, FeatureItem } from "@/lib/website";

const ICONS: Record<string, LucideIcon> = {
  mail: Mail,
  email: Mail,
  phone: Phone,
  map: MapPin,
  address: MapPin,
};

function resolveIcon(key?: string): LucideIcon | undefined {
  if (!key) return undefined;
  return ICONS[key.toLowerCase()];
}

function getText(content: Record<string, unknown>, ...keys: string[]): string | undefined {
  for (const key of keys) {
    const value = content[key];
    if (typeof value === "string") return value;
  }
  return undefined;
}

function getItems(content: Record<string, unknown>): FeatureItem[] {
  const value = content.items;
  if (Array.isArray(value)) {
    return value.filter((item): item is FeatureItem => item !== null && typeof item === "object");
  }
  return [];
}

export function WebsiteSection({ section }: { section: WebsiteSection }) {
  switch (section.type) {
    case "hero":
      return <HeroSection section={section} />;
    case "text":
      return <TextSection section={section} />;
    case "features":
    case "gallery":
      return <FeaturesSection section={section} />;
    case "cta":
    case "call_to_action":
      return <CTASection section={section} />;
    case "contact":
      return <ContactSection section={section} />;
    default:
      return null;
  }
}

function HeroSection({ section }: { section: WebsiteSection }) {
  const content =
    typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content, "headline", "title") ?? "";
  const body = getText(content, "body", "subtitle");
  const ctaLabel = getText(content, "cta_label", "button_label");
  const ctaUrl = getText(content, "cta_url", "button_url");

  return (
    <section className="school-site-hero relative isolate overflow-hidden py-24 text-white lg:py-32">
      <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[var(--color-signal)] via-[var(--color-teal-bright)] to-[var(--color-brand)]" />
      <div className="mx-auto grid max-w-7xl gap-12 px-6 lg:grid-cols-[minmax(0,1fr)_20rem] lg:items-end">
        <Reveal>
          <div>
            <p className="inline-flex items-center gap-2 rounded-full border border-white/14 bg-white/[0.07] px-3 py-2 font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--color-teal-bright)]">
              <Sparkles className="size-3.5" aria-hidden="true" /> School experience
            </p>
            <h1 className="mt-7 max-w-4xl text-balance font-heading text-4xl font-black leading-[1.03] tracking-[-0.035em] sm:text-5xl lg:text-7xl">
              {title}
            </h1>
            {body ? <p className="mt-6 max-w-2xl text-lg leading-8 text-white/68">{body}</p> : null}
            {ctaLabel && ctaUrl ? (
              <div className="mt-9">
                <Button asChild variant="gold" pill className="h-12 px-6">
                  <Link href={String(ctaUrl)}>
                    {ctaLabel}
                    <ArrowRight className="size-4" aria-hidden="true" />
                  </Link>
                </Button>
              </div>
            ) : null}
          </div>
        </Reveal>
        <Reveal delay={120} variant="right">
          <aside className="rounded-3xl border border-white/12 bg-white/[0.07] p-6 backdrop-blur-sm">
            <p className="font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--color-signal)]">
              Connected school life
            </p>
            <p className="mt-4 text-2xl font-extrabold leading-tight">
              One trusted place to discover, apply and stay informed.
            </p>
            <div className="mt-6 h-1 w-16 rounded-full bg-gradient-to-r from-[var(--color-signal)] to-[var(--color-teal-bright)]" />
          </aside>
        </Reveal>
      </div>
    </section>
  );
}

function TextSection({ section }: { section: WebsiteSection }) {
  const content =
    typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content, "title", "heading");
  const body = getText(content, "body", "text", "content");

  return (
    <section className="py-18 lg:py-24">
      <div className="mx-auto max-w-7xl px-6">
        <Reveal>
          <div className="grid gap-8 border-t border-[var(--border)] pt-8 lg:grid-cols-[15rem_minmax(0,1fr)] lg:gap-16">
            <p className="font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--primary)]">
              The story
            </p>
            <div>
              {title ? (
                <h2 className="max-w-3xl font-heading text-3xl font-black tracking-tight text-[var(--foreground)] lg:text-4xl">
                  {title}
                </h2>
              ) : null}
              {body ? (
                <div
                  className={`max-w-3xl text-base leading-8 text-[var(--muted-foreground)] ${title ? "mt-5" : ""}`}
                >
                  {body}
                </div>
              ) : null}
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function FeaturesSection({ section }: { section: WebsiteSection }) {
  const content =
    typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content, "title", "heading");
  const body = getText(content, "body", "subtitle");
  const items = getItems(content);

  return (
    <section className="py-20 lg:py-24">
      <div className="mx-auto max-w-7xl px-6">
        {title ? (
          <h2 className="text-balance text-center font-heading text-3xl font-black tracking-tight text-[var(--foreground)] lg:text-4xl">
            {title}
          </h2>
        ) : null}
        {body ? (
          <p className="mx-auto mt-3 max-w-2xl text-center text-[var(--muted-foreground)]">
            {body}
          </p>
        ) : null}
        {items.length > 0 ? (
          <div className="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((item, index) => (
              <Reveal key={index} delay={Math.min(index * 60, 240)}>
                <FeatureCard item={item} index={index} />
              </Reveal>
            ))}
          </div>
        ) : null}
      </div>
    </section>
  );
}

function FeatureCard({ item, index }: { item: FeatureItem; index: number }) {
  const Icon = resolveIcon(item.icon);
  return (
    <article className="school-site-card relative h-full overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6">
      <span className="absolute right-5 top-4 font-mono text-[10px] font-black tracking-[0.16em] text-[var(--muted-foreground)]/45">
        {String(index + 1).padStart(2, "0")}
      </span>
      {Icon ? (
        <div className="mb-4 inline-flex rounded-[var(--radius-sm)] bg-[var(--accent)] p-2.5 text-[var(--primary)]">
          <Icon className="size-5" aria-hidden="true" />
        </div>
      ) : null}
      <h3 className="font-sans text-lg font-semibold text-[var(--foreground)]">
        {item.title ?? "Feature"}
      </h3>
      {item.description ? (
        <p className="mt-2 text-sm text-[var(--muted-foreground)]">{item.description}</p>
      ) : null}
    </article>
  );
}

function CTASection({ section }: { section: WebsiteSection }) {
  const content =
    typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content, "title", "headline", "heading") ?? "";
  const body = getText(content, "body", "text", "subtitle");
  const ctaLabel = getText(content, "cta_label", "button_label");
  const ctaUrl = getText(content, "cta_url", "button_url");

  return (
    <section className="py-16 lg:py-20">
      <div className="mx-auto max-w-4xl px-6">
        <div className="school-site-hero relative overflow-hidden rounded-3xl px-8 py-12 text-center text-white shadow-2xl lg:px-16 lg:py-16">
          <p className="font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--color-signal)]">
            Your next step
          </p>
          <h2 className="mt-4 text-balance font-heading text-3xl font-black tracking-tight lg:text-4xl">
            {title}
          </h2>
          {body ? <p className="mx-auto mt-4 max-w-xl opacity-90">{body}</p> : null}
          {ctaLabel && ctaUrl ? (
            <div className="mt-6">
              <Button variant="gold" pill asChild>
                <Link href={String(ctaUrl)}>
                  {ctaLabel}
                  <ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </Button>
            </div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function ContactSection({ section }: { section: WebsiteSection }) {
  const content =
    typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content, "title", "heading");
  const email = getText(content, "email", "contact_email");
  const phone = getText(content, "phone", "contact_phone", "telephone");
  const address = getText(content, "address", "location");

  return (
    <section className="py-16 lg:py-20">
      <div className="mx-auto max-w-4xl px-6">
        {title ? (
          <h2 className="text-center font-heading text-2xl font-bold text-[var(--foreground)]">
            {title}
          </h2>
        ) : null}
        <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {email ? (
            <ContactCard icon={Mail} label="Email" value={email} href={`mailto:${email}`} />
          ) : null}
          {phone ? (
            <ContactCard icon={Phone} label="Phone" value={phone} href={`tel:${phone}`} />
          ) : null}
          {address ? <ContactCard icon={MapPin} label="Address" value={address} /> : null}
        </div>
      </div>
    </section>
  );
}

function ContactCard({
  icon: Icon,
  label,
  value,
  href,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  href?: string;
}) {
  const body = (
    <div className="school-site-card flex h-full items-start gap-4 rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5">
      <div className="rounded-[var(--radius-sm)] bg-[var(--accent)] p-2 text-[var(--primary)]">
        <Icon className="size-5" aria-hidden="true" />
      </div>
      <div>
        <p className="text-xs font-medium uppercase tracking-wider text-[var(--muted-foreground)]">
          {label}
        </p>
        <p className="mt-1 font-medium text-[var(--foreground)]">{value}</p>
      </div>
    </div>
  );

  if (href) {
    return (
      <a href={href} className="block transition-opacity hover:opacity-80">
        {body}
      </a>
    );
  }
  return body;
}
