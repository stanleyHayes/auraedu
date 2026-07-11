import type { Metadata } from "next";
import { fetchPageBySlug } from "@/lib/website";
import { fetchTenantBranding } from "@/lib/tenant";
import { WebsiteSection } from "@/components/website-section";

interface PublicHomePageProps {
  params: Promise<{ tenant: string }>;
}

export async function generateMetadata({ params }: PublicHomePageProps): Promise<Metadata> {
  const { tenant: tenantCode } = await params;
  const [tenant, page] = await Promise.all([
    fetchTenantBranding(tenantCode),
    fetchPageBySlug(tenantCode, "home"),
  ]);

  return {
    title: page?.title ? `${page.title} — ${tenant.name}` : tenant.name,
    description: page?.meta_description ?? `Welcome to ${tenant.name}.`,
    robots: { index: true, follow: true },
  };
}

export default async function PublicHomePage({ params }: PublicHomePageProps) {
  const { tenant: tenantCode } = await params;
  const page = await fetchPageBySlug(tenantCode, "home");

  if (!page) {
    return <WelcomePlaceholder />;
  }

  return (
    <article>
      {page.sections && page.sections.length > 0 ? (
        page.sections.map((section) => <WebsiteSection key={section.id} section={section} />)
      ) : (
        <WelcomePlaceholder title={page.title} />
      )}
    </article>
  );
}

function WelcomePlaceholder({ title }: { title?: string }) {
  return (
    <section className="py-20 lg:py-28">
      <div className="mx-auto max-w-3xl px-6 text-center">
        <h1 className="font-display text-4xl font-extrabold tracking-tight text-[var(--foreground)] lg:text-5xl">
          {title ?? "Welcome"}
        </h1>
        <p className="mx-auto mt-6 max-w-xl text-lg text-[var(--muted-foreground)]">
          This school&apos;s public website is being set up. Check back soon for admissions news,
          announcements, and more.
        </p>
      </div>
    </section>
  );
}
