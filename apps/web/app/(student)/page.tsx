import { FeatureGate, FeatureDisabled } from "@auraedu/flags";

export default function StudentOverview() {
  return (
    <div className="space-y-6">
      <FeatureGate feature="assignments" fallback={<FeatureDisabled feature="assignments" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Assignments</h2>
          <p className="text-sm text-muted-foreground">View and submit your assignments.</p>
        </div>
      </FeatureGate>
      <FeatureGate feature="assessments" fallback={<FeatureDisabled feature="assessments" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Results</h2>
          <p className="text-sm text-muted-foreground">Check your latest assessment results.</p>
        </div>
      </FeatureGate>
    </div>
  );
}
