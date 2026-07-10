import { FeatureGate, FeatureDisabled } from "@auraedu/flags";

export default function ParentOverview() {
  return (
    <div className="space-y-6">
      <FeatureGate feature="attendance" fallback={<FeatureDisabled feature="attendance" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Attendance</h2>
          <p className="text-sm text-muted-foreground">View your children's attendance records.</p>
        </div>
      </FeatureGate>
      <FeatureGate feature="report_cards" fallback={<FeatureDisabled feature="report_cards" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Report Cards</h2>
          <p className="text-sm text-muted-foreground">Download term report cards.</p>
        </div>
      </FeatureGate>
    </div>
  );
}
