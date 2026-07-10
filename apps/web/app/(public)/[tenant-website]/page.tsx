import { notFound } from "next/navigation";
import { fetchTenantBranding, makeFallbackTenant } from "@/lib/tenant";

interface PublicWebsitePageProps {
  params: Promise<{ "tenant-website": string }>;
}

export default async function PublicWebsitePage({ params }: PublicWebsitePageProps) {
  const { "tenant-website": code } = await params;
  const tenant = await fetchTenantBranding(code).catch(() => makeFallbackTenant(code));

  if (!tenant) {
    notFound();
  }

  return (
    <div className="min-h-screen bg-background p-8">
      <header className="mx-auto max-w-5xl">
        <h1 className="font-display text-4xl font-extrabold text-[var(--primary)]">{tenant.name}</h1>
        <p className="mt-2 text-lg text-muted-foreground">Welcome to the {tenant.short} public website.</p>
      </header>
      <main className="mx-auto mt-10 max-w-5xl">
        <p className="text-foreground">
          This is a scaffold for the per-school public website. Content will be served by the Website
          Service and rendered here with tenant branding.
        </p>
      </main>
    </div>
  );
}
