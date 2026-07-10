export default function SuperAdminOverview() {
  return (
    <div className="space-y-6">
      <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
        <h2 className="font-semibold">Tenants</h2>
        <p className="text-sm text-muted-foreground">Create and manage schools on the platform.</p>
      </div>
      <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
        <h2 className="font-semibold">Feature Flags</h2>
        <p className="text-sm text-muted-foreground">Toggle features per tenant and plan.</p>
      </div>
    </div>
  );
}
