import { FeatureGate, FeatureDisabled } from "@auraedu/flags";

export default function TeacherOverview() {
  return (
    <div className="space-y-6">
      <FeatureGate feature="attendance" fallback={<FeatureDisabled feature="attendance" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Attendance</h2>
          <p className="text-sm text-muted-foreground">Mark attendance for your classes.</p>
        </div>
      </FeatureGate>
      <FeatureGate feature="assessments" fallback={<FeatureDisabled feature="assessments" />}>
        <div className="rounded-[var(--radius-md)] border border-border bg-surface p-4">
          <h2 className="font-semibold">Assessments</h2>
          <p className="text-sm text-muted-foreground">Record and manage assessment scores.</p>
        </div>
      </FeatureGate>
    </div>
  );
}
