import type { Metadata } from "next";
import { Fraunces, Outfit, Spline_Sans_Mono } from "next/font/google";
import "./globals.css";
import { SiteHeader } from "@/components/site-header";
import { SiteFooter } from "@/components/site-footer";

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
  title: "AuraEDU — one platform, every school",
  description:
    "A multi-tenant school operating system for Ghana: admissions, registers, results, report cards and fees — every school its own brand, data, and features.",
};

// No-FOUC: set the theme class before hydration so first paint is correct.
const themeScript = `(function(){try{var m=localStorage.getItem('auraedu-theme')||(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');var r=document.documentElement;r.classList.toggle('dark',m==='dark');r.style.colorScheme=m;}catch(e){}})();`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html
      lang="en"
      className={`${fraunces.variable} ${outfit.variable} ${mono.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="flex min-h-screen flex-col bg-background text-foreground">
        <SiteHeader />
        <main className="flex-1">{children}</main>
        <SiteFooter />
      </body>
    </html>
  );
}
