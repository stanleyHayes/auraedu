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

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="flex min-h-screen flex-col bg-background text-foreground">
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
