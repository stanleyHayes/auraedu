import { Watermark } from "@auraedu/ui";
import { AuraEduLogo } from "@/components/auraedu-logo";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="auth-stage grid min-h-screen lg:grid-cols-[48%_1fr]">
      <div className="auth-story relative hidden flex-col justify-between overflow-hidden p-12 text-[var(--color-cream)] lg:flex">
        <span
          aria-hidden="true"
          className="absolute -right-24 -top-24 size-96 rounded-full bg-[var(--color-brand)]/25 blur-3xl"
        />
        <span
          aria-hidden="true"
          className="absolute -bottom-32 left-8 size-80 rounded-full bg-[var(--color-forest)]/20 blur-3xl"
        />
        <Watermark className="pointer-events-none absolute -left-10 bottom-20 text-[16rem] opacity-[0.06]">
          Aura
        </Watermark>
        <AuraEduLogo tone="light" priority className="relative z-10 h-10" />
        <div className="relative z-10">
          <p className="font-mono text-[10px] font-bold uppercase tracking-[0.2em] text-[var(--color-signal)]">
            The education operating system
          </p>
          <h2 className="mt-5 max-w-xl font-heading text-5xl font-extrabold leading-[1.05] tracking-tight">
            Every school day, beautifully orchestrated.
          </h2>
          <p className="mt-5 max-w-lg text-lg leading-8 text-white/72">
            One calm command centre for leaders, teachers, families and learners—from first enquiry
            to lifelong outcomes.
          </p>
          <div className="mt-8 flex gap-3">
            <span className="h-1 w-16 rounded-full bg-[var(--color-signal)]" />
            <span className="h-1 w-8 rounded-full bg-[var(--color-brand)]" />
            <span className="h-1 w-5 rounded-full bg-[var(--color-teal-bright)]" />
          </div>
        </div>
        <p className="relative z-10 text-sm opacity-75">© {new Date().getFullYear()} AuraEDU</p>
      </div>
      <div className="relative flex items-center justify-center overflow-hidden bg-background p-6 sm:p-10">
        <span
          aria-hidden="true"
          className="absolute -right-32 -top-32 size-80 rounded-full bg-[var(--color-brand)]/10 blur-3xl"
        />
        {children}
      </div>
    </div>
  );
}
