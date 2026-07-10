export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid min-h-screen lg:grid-cols-[45%_1fr]">
      <div className="hidden flex-col justify-between bg-[var(--primary)] p-10 text-[var(--primary-foreground)] lg:flex">
        <div className="font-display text-2xl font-extrabold tracking-tight">AuraEDU</div>
        <div>
          <h2 className="font-display text-4xl font-extrabold leading-tight">Run your school from one place.</h2>
          <p className="mt-4 text-lg opacity-90">Admissions to report cards — one platform, every role, every device.</p>
        </div>
        <p className="text-sm opacity-75">© {new Date().getFullYear()} AuraEDU</p>
      </div>
      <div className="flex items-center justify-center bg-background p-6">
        <div className="w-full max-w-[420px]">{children}</div>
      </div>
    </div>
  );
}
