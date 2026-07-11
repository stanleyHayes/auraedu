import Link from "next/link";
import { Button } from "@auraedu/ui";
import { Mail, Phone, MapPin, type LucideIcon } from "lucide-react";
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
  const content = typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content as Record<string, unknown>, "headline", "title") ?? "";
  const body = getText(content as Record<string, unknown>, "body", "subtitle");
  const ctaLabel = getText(content as Record<string, unknown>, "cta_label", "button_label");
  const ctaUrl = getText(content as Record<string, unknown>, "cta_url", "button_url");

  return (
    <section className="bg-[var(--surface)] py-20 lg:py-28">
      <div className="mx-auto max-w-5xl px-6 text-center">
        <h1 className="font-display text-4xl font-extrabold tracking-tight text-[var(--foreground)] lg:text-5xl">
          {title}
        </h1>
        {body ? <p className="mx-auto mt-6 max-w-2xl text-lg text-[var(--muted-foreground)]">{body}</p> : null}
        {ctaLabel && ctaUrl ? (
          <div className="mt-8">
            <Button asChild>
              <Link href={String(ctaUrl)}>{ctaLabel}</Link>
            </Button>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function TextSection({ section }: { section: WebsiteSection }) {
  const content = typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content as Record<string, unknown>, "title", "heading");
  const body = getText(content as Record<string, unknown>, "body", "text", "content");

  return (
    <section className="py-16 lg:py-20">
      <div className="mx-auto max-w-3xl px-6">
        {title ? <h2 className="font-display text-2xl font-bold text-[var(--foreground)]">{title}</h2> : null}
        {body ? (
          <div
            className={`text-[var(--muted-foreground)] ${title ? "mt-4" : ""}`}
            style={{ lineHeight: 1.7, maxWidth: "65ch" }}
          >
            {body}
          </div>
        ) : null}
      </div>
    </section>
  );
}

function FeaturesSection({ section }: { section: WebsiteSection }) {
  const content = typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content as Record<string, unknown>, "title", "heading");
  const body = getText(content as Record<string, unknown>, "body", "subtitle");
  const items = getItems(content as Record<string, unknown>);

  return (
    <section className="py-16 lg:py-20">
      <div className="mx-auto max-w-5xl px-6">
        {title ? (
          <h2 className="text-center font-display text-2xl font-bold text-[var(--foreground)]">{title}</h2>
        ) : null}
        {body ? (
          <p className="mx-auto mt-3 max-w-2xl text-center text-[var(--muted-foreground)]">{body}</p>
        ) : null}
        {items.length > 0 ? (
          <div className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((item, index) => (
              <FeatureCard key={index} item={item} />
            ))}
          </div>
        ) : null}
      </div>
    </section>
  );
}

function FeatureCard({ item }: { item: FeatureItem }) {
  const Icon = resolveIcon(item.icon);
  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-6 transition-shadow hover:shadow-sm">
      {Icon ? (
        <div className="mb-4 inline-flex rounded-[var(--radius-sm)] bg-[var(--accent)] p-2.5 text-[var(--primary)]">
          <Icon className="size-5" aria-hidden="true" />
        </div>
      ) : null}
      <h3 className="font-display text-lg font-semibold text-[var(--foreground)]">
        {item.title ?? "Feature"}
      </h3>
      {item.description ? <p className="mt-2 text-sm text-[var(--muted-foreground)]">{item.description}</p> : null}
    </div>
  );
}

function CTASection({ section }: { section: WebsiteSection }) {
  const content = typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content as Record<string, unknown>, "title", "headline", "heading") ?? "";
  const body = getText(content as Record<string, unknown>, "body", "text", "subtitle");
  const ctaLabel = getText(content as Record<string, unknown>, "cta_label", "button_label");
  const ctaUrl = getText(content as Record<string, unknown>, "cta_url", "button_url");

  return (
    <section className="py-16 lg:py-20">
      <div className="mx-auto max-w-4xl px-6">
        <div className="rounded-[var(--radius-lg)] bg-[var(--primary)] px-8 py-12 text-center text-[var(--primary-foreground)] lg:px-16 lg:py-16">
          <h2 className="font-display text-2xl font-bold lg:text-3xl">{title}</h2>
          {body ? <p className="mx-auto mt-4 max-w-xl opacity-90">{body}</p> : null}
          {ctaLabel && ctaUrl ? (
            <div className="mt-6">
              <Button variant="secondary" asChild>
                <Link href={String(ctaUrl)}>{ctaLabel}</Link>
              </Button>
            </div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function ContactSection({ section }: { section: WebsiteSection }) {
  const content = typeof section.content === "object" && section.content !== null ? section.content : {};
  const title = getText(content as Record<string, unknown>, "title", "heading");
  const email = getText(content as Record<string, unknown>, "email", "contact_email");
  const phone = getText(content as Record<string, unknown>, "phone", "contact_phone", "telephone");
  const address = getText(content as Record<string, unknown>, "address", "location");

  return (
    <section className="py-16 lg:py-20">
      <div className="mx-auto max-w-4xl px-6">
        {title ? (
          <h2 className="text-center font-display text-2xl font-bold text-[var(--foreground)]">{title}</h2>
        ) : null}
        <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {email ? (
            <ContactCard
              icon={Mail}
              label="Email"
              value={email}
              href={`mailto:${email}`}
            />
          ) : null}
          {phone ? (
            <ContactCard
              icon={Phone}
              label="Phone"
              value={phone}
              href={`tel:${phone}`}
            />
          ) : null}
          {address ? (
            <ContactCard
              icon={MapPin}
              label="Address"
              value={address}
            />
          ) : null}
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
    <div className="flex items-start gap-4 rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
      <div className="rounded-[var(--radius-sm)] bg-[var(--accent)] p-2 text-[var(--primary)]">
        <Icon className="size-5" aria-hidden="true" />
      </div>
      <div>
        <p className="text-xs font-medium uppercase tracking-wider text-[var(--muted-foreground)]">{label}</p>
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
