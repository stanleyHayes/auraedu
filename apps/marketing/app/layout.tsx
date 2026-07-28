import type { Metadata } from "next";
import "./globals.css";
import { SiteHeader } from "@/components/site-header";
import { SiteFooter } from "@/components/site-footer";

export const metadata: Metadata = {
  metadataBase: new URL("https://auraedu.com"),
  title: {
    default: "AuraEDU — the education operating system",
    template: "%s · AuraEDU",
  },
  description:
    "A tenant-isolated, configurable education operating system for school operations, learning workflows and family engagement.",
  icons: {
    icon: [
      { url: "/icon.svg", type: "image/svg+xml" },
      { url: "/favicon.ico", sizes: "any" },
    ],
    shortcut: "/icon.svg",
    apple: "/apple-icon.png",
  },
  openGraph: {
    type: "website",
    locale: "en_GH",
    siteName: "AuraEDU",
    title: "AuraEDU — the education operating system",
    description:
      "Run school operations, learning workflows and family engagement on one configurable platform.",
  },
  twitter: {
    card: "summary_large_image",
    title: "AuraEDU — the education operating system",
    description:
      "Run school operations, learning workflows and family engagement on one configurable platform.",
  },
};

// Organization + WebSite + SoftwareApplication structured data (JSON-LD). Static
// brand facts only, rendered once; the `<` escape guards the inline script
// against early termination.
const STRUCTURED_DATA = JSON.stringify({
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "@id": "https://auraedu.com/#organization",
      name: "AuraEDU",
      url: "https://auraedu.com",
      logo: "https://auraedu.com/icon.svg",
      description:
        "AuraEDU is a tenant-isolated, configurable education operating system for school operations, learning workflows and family engagement.",
    },
    {
      "@type": "WebSite",
      "@id": "https://auraedu.com/#website",
      url: "https://auraedu.com",
      name: "AuraEDU",
      publisher: { "@id": "https://auraedu.com/#organization" },
    },
    {
      "@type": "SoftwareApplication",
      "@id": "https://auraedu.com/#application",
      name: "AuraEDU",
      url: "https://auraedu.com",
      applicationCategory: "EducationalApplication",
      operatingSystem: "Web",
      description:
        "One configurable platform for school operations, teaching, finance, admissions, family engagement, trusted analytics and accountable AI.",
      publisher: { "@id": "https://auraedu.com/#organization" },
    },
  ],
}).replace(/</g, "\\u003c");

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="flex min-h-screen flex-col bg-background text-foreground">
        <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: STRUCTURED_DATA }} />
        <a className="skip-link" href="#main-content">
          Skip to content
        </a>
        <SiteHeader />
        <main id="main-content" className="flex-1" tabIndex={-1}>
          {children}
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
