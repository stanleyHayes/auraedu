import type { Metadata } from "next";
import { Fraunces, Outfit, Spline_Sans_Mono } from "next/font/google";
import { headers } from "next/headers";
import { notFound } from "next/navigation";
import { FeatureFlagsProvider, type FeatureSnapshot } from "@auraedu/flags";
import {
  getTenantCodeFromHeaders,
  isTenantNotFound,
  fetchTenantBranding,
  toFeatureSnapshot,
} from "@/lib/tenant";
import "./globals.css";

const fraunces = Fraunces({
  subsets: ["latin"],
  weight: ["600", "700", "800"],
  variable: "--font-fraunces",
  display: "swap",
});
const outfit = Outfit({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-outfit",
  display: "swap",
});
const mono = Spline_Sans_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  variable: "--font-spline-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "AuraEDU — School portal",
  description: "The AuraEDU school portal.",
  robots: { index: false },
};

const bootScript = `(function(){try{var r=document.documentElement;var m=localStorage.getItem('auraedu-theme')||(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');r.classList.toggle('dark',m==='dark');r.style.colorScheme=m;}catch(e){}})();`;

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);

  if (!tenantCode || isTenantNotFound(requestHeaders)) {
    notFound();
  }

  let tenant;
  try {
    tenant = await fetchTenantBranding(tenantCode);
  } catch {
    notFound();
  }

  const snapshot: FeatureSnapshot = toFeatureSnapshot(tenant);

  const brand = tenant.branding.brand.primary;
  const secondary = tenant.branding.brand.secondary;
  const tenantTheme = `:root { --color-brand: ${brand};${secondary ? ` --color-brand-secondary: ${secondary};` : ""} }`;

  return (
    <html
      lang="en"
      className={`${fraunces.variable} ${outfit.variable} ${mono.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: bootScript }} />
        <style id="tenant-theme" dangerouslySetInnerHTML={{ __html: tenantTheme }} />
      </head>
      <body className="bg-background text-foreground">
        <FeatureFlagsProvider snapshot={snapshot}>{children}</FeatureFlagsProvider>
      </body>
    </html>
  );
}
