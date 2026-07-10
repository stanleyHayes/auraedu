import { FeatureGate, FeatureDisabled } from "@auraedu/flags";

export default function AdminOverview() {
  return (
    <div className="space-y-6">
      <FeatureGate feature="student_management" fallback={<FeatureDisabled feature="student_management" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Students</h2>
          <p className="text-sm text-muted-foreground">Student management module placeholder.</p>
        </div>
      </FeatureGate>
      <FeatureGate feature="staff_management" fallback={<FeatureDisabled feature="staff_management" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Staff</h2>
          <p className="text-sm text-muted-foreground">Staff management module placeholder.</p>
        </div>
      </FeatureGate>
    </div>
  );
}
