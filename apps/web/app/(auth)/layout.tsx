import { Watermark } from "@auraedu/ui";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid min-h-screen lg:grid-cols-[45%_1fr]">
      <div className="relative hidden flex-col justify-between overflow-hidden bg-[var(--color-navy)] p-10 text-[var(--color-cream)] lg:flex">
        <Watermark className="pointer-events-none absolute -left-10 bottom-20 text-[16rem] opacity-[0.06]">
          Aura
        </Watermark>
        <div className="relative z-10 font-sans text-2xl font-extrabold tracking-tight">AuraEDU</div>
        <div className="relative z-10">
          <h2 className="font-heading text-4xl font-extrabold leading-tight">
            Run your school from one place.
          </h2>
          <p className="mt-4 text-lg opacity-90">
            Admissions to report cards — one platform, every role, every device.
          </p>
        </div>
        <p className="relative z-10 text-sm opacity-75">© {new Date().getFullYear()} AuraEDU</p>
      </div>
      <div className="flex items-center justify-center bg-background p-6">
        {children}
      </div>
    </div>
  );
}
