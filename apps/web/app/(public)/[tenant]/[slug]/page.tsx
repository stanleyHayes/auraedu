import type { Metadata } from "next";
import { fetchPageBySlug } from "@/lib/website";
import { fetchTenantBranding } from "@/lib/tenant";
import { WebsiteSection } from "@/components/website-section";

interface PublicPageProps {
  params: Promise<{ tenant: string; slug: string }>;
}

export async function generateMetadata({ params }: PublicPageProps): Promise<Metadata> {
  const { tenant: tenantCode, slug } = await params;
  const [tenant, page] = await Promise.all([
    fetchTenantBranding(tenantCode),
    fetchPageBySlug(tenantCode, slug),
  ]);

  return {
    title: page?.title ? `${page.title} — ${tenant.name}` : tenant.name,
    description: page?.meta_description ?? `Learn more about ${tenant.name}.`,
    robots: { index: true, follow: true },
  };
}

export default async function PublicPage({ params }: PublicPageProps) {
  const { tenant: tenantCode, slug } = await params;
  const page = await fetchPageBySlug(tenantCode, slug);

  if (!page) {
    return <PageNotFoundPlaceholder slug={slug} />;
  }

  return (
    <article>
      {page.sections && page.sections.length > 0 ? (
        page.sections.map((section) => <WebsiteSection key={section.id} section={section} />)
      ) : (
        <section className="py-16 lg:py-20">
          <div className="mx-auto max-w-3xl px-6 text-center">
            <h1 className="font-heading text-3xl font-bold text-[var(--foreground)]">
              {page.title}
            </h1>
            <p className="mt-4 text-[var(--muted-foreground)]">This page has no content yet.</p>
          </div>
        </section>
      )}
    </article>
  );
}

function PageNotFoundPlaceholder({ slug }: { slug: string }) {
  return (
    <section className="py-20 lg:py-28">
      <div className="mx-auto max-w-3xl px-6 text-center">
        <h1 className="font-heading text-4xl font-extrabold tracking-tight text-[var(--foreground)] lg:text-5xl">
          Page not found
        </h1>
        <p className="mx-auto mt-6 max-w-xl text-lg text-[var(--muted-foreground)]">
          We could not find the page &ldquo;{slug}&rdquo;. It may have been moved or is still being
          prepared.
        </p>
      </div>
    </section>
  );
}
