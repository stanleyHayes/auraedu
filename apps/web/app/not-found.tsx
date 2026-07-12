import Link from "next/link";
import { Watermark } from "@auraedu/ui";

export const metadata = {
  title: "School not found — AuraEDU",
};

export default function NotFound() {
  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-background p-6 text-center">
      <Watermark className="pointer-events-none text-[22rem] opacity-[0.03]">404</Watermark>
      <div className="relative z-10">
        <h1 className="font-heading text-7xl font-extrabold tracking-tight text-brand">404</h1>
        <p className="mt-4 text-2xl font-semibold text-foreground">School not found</p>
        <p className="mt-2 max-w-md text-muted-foreground">
          We could not find a school matching this address. Please check the URL or contact your
          administrator.
        </p>
        <Link
          href="/"
          className="mt-8 inline-flex h-10 items-center justify-center rounded-full bg-brand px-6 text-sm font-medium text-white shadow-md transition-transform hover:-translate-y-px hover:shadow-lg"
        >
          Go home
        </Link>
      </div>
    </div>
  );
}
