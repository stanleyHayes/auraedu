export function WorkspaceField({
  defaultValue = "",
  required = false,
}: {
  defaultValue?: string;
  required?: boolean;
}) {
  return (
    <div>
      <label htmlFor="tenant" className="mb-1.5 block text-sm font-semibold">
        School workspace
      </label>
      <input
        id="tenant"
        name="tenant"
        defaultValue={defaultValue}
        autoComplete="organization"
        inputMode="url"
        pattern="[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?"
        placeholder="e.g. upshs"
        aria-describedby="tenant-help"
        required={required}
        className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm lowercase text-[var(--foreground)] shadow-sm placeholder:text-[var(--muted-foreground)] focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
      />
      <p id="tenant-help" className="mt-1.5 text-xs text-muted-foreground">
        Leave blank when you opened your school&apos;s own portal link.
      </p>
    </div>
  );
}
