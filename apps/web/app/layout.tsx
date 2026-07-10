import type { Metadata } from "next";
import { Bricolage_Grotesque, Public_Sans, Spline_Sans_Mono } from "next/font/google";
import { DEFAULT_TENANT } from "@/lib/tenant";
import "./globals.css";

const display = Bricolage_Grotesque({ subsets: ["latin"], weight: ["600", "700", "800"], variable: "--font-bricolage", display: "swap" });
const body = Public_Sans({ subsets: ["latin"], weight: ["400", "500", "600", "700"], variable: "--font-public-sans", display: "swap" });
const mono = Spline_Sans_Mono({ subsets: ["latin"], weight: ["400", "500"], variable: "--font-spline-mono", display: "swap" });

export const metadata: Metadata = {
  title: "AuraEDU — School portal",
  description: "The AuraEDU school portal.",
  robots: { index: false },
};

// No-FOUC: set theme + the tenant's brand accent before hydration so first paint is correct.
const bootScript = `(function(){try{var r=document.documentElement;var m=localStorage.getItem('auraedu-theme')||(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');r.classList.toggle('dark',m==='dark');r.style.colorScheme=m;r.style.setProperty('--color-brand',${JSON.stringify(DEFAULT_TENANT.brand)});}catch(e){}})();`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${display.variable} ${body.variable} ${mono.variable}`} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: bootScript }} />
      </head>
      <body className="bg-background text-foreground">{children}</body>
    </html>
  );
}
