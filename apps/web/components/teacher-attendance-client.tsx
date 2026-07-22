"use client";

import * as React from "react";
import { AlertTriangle, CalendarCheck, ClipboardList, LoaderCircle } from "lucide-react";
import { useActionState } from "react";
import {
  Button,
  DataTable,
  EmptyState,
  Input,
  Label,
  Select,
  type DataTableColumn,
} from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import {
  loadTeacherClassRosterAction,
  markAttendanceBulkAction,
  type TeacherActionResult,
} from "@/lib/teacher-actions";
import type { AttendanceRecord } from "@/app/(teacher)/teacher/attendance/page";

type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Student = OpenAPI.student_v1.components["schemas"]["Student"];

const STATUS_OPTIONS = ["present", "absent", "late", "excused"] as const;

const columns: DataTableColumn<AttendanceRecord>[] = [
  { key: "student_id", header: "Student ID", cell: (r) => r.student_id },
  { key: "date", header: "Date", cell: (r) => r.date },
  { key: "status", header: "Status", cell: (r) => <span className="capitalize">{r.status}</span> },
];

interface TeacherAttendanceClientProps {
  initialRecords: AttendanceRecord[];
  recordsAvailable: boolean;
  classes: AcademicClass[] | null;
  years: AcademicYear[] | null;
  initialClassId: string;
  initialStudents: Student[] | null;
}

export function TeacherAttendanceClient({
  initialRecords,
  recordsAvailable,
  classes,
  years,
  initialClassId,
  initialStudents,
}: TeacherAttendanceClientProps) {
  const [state, formAction, pending] = useActionState<TeacherActionResult, FormData>(
    markAttendanceBulkAction,
    {},
  );
  const formRef = React.useRef<HTMLFormElement>(null);

  const classRows = classes ?? [];
  const yearRows = years ?? [];
  const currentYear = yearRows.find((y) => y.is_current) ?? yearRows[0];
  const [classId, setClassId] = React.useState(initialClassId);
  const [yearId, setYearId] = React.useState(
    classRows[0]?.academic_year_id ?? currentYear?.id ?? "",
  );
  const [rosters, setRosters] = React.useState<Record<string, Student[] | null>>(
    initialClassId ? { [initialClassId]: initialStudents } : {},
  );
  const [rosterLoading, setRosterLoading] = React.useState(false);
  const today = React.useMemo(() => new Date().toISOString().split("T")[0], []);

  function onClassChange(nextClassId: string) {
    setClassId(nextClassId);
    const nextClass = classRows.find((c) => c.id === nextClassId);
    if (nextClass) {
      setYearId(nextClass.academic_year_id);
    }
  }

  React.useEffect(() => {
    if (state.success) {
      // Reset per-student statuses to their defaults; keep class/year/date.
      formRef.current?.querySelectorAll("select[data-roster]").forEach((el) => {
        (el as HTMLSelectElement).value = "present";
      });
    }
  }, [state]);

  React.useEffect(() => {
    if (!classId || rosters[classId] !== undefined) return;
    let active = true;
    setRosterLoading(true);
    void loadTeacherClassRosterAction(classId).then((result) => {
      if (!active) return;
      setRosters((current) => ({
        ...current,
        [classId]: result.error ? null : (result.students ?? []),
      }));
      setRosterLoading(false);
    });
    return () => {
      active = false;
    };
  }, [classId, rosters]);

  const roster = classId ? rosters[classId] : [];

  return (
    <div className="space-y-6">
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-sans font-semibold tracking-tight">Attendance records</h3>
        <div className="mt-4">
          {recordsAvailable ? (
            <DataTable
              caption="Attendance records for the teacher's tenant"
              columns={columns}
              rows={initialRecords}
              keyExtractor={(r) => r.id}
              empty={
                <EmptyState
                  icon={<ClipboardList className="size-8" />}
                  title="No attendance records"
                  description="Records will appear here once attendance is recorded."
                />
              }
            />
          ) : (
            <EmptyState
              icon={<AlertTriangle className="size-8" />}
              title="Attendance records unavailable"
              description="The attendance service could not be reached. No empty register has been assumed."
            />
          )}
        </div>
      </section>

      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-sans font-semibold tracking-tight">Mark attendance</h3>
        <p className="mt-1 text-sm text-[var(--muted-foreground)]">
          Pick a class, set a status per student, and save the whole register at once.
        </p>

        <form ref={formRef} action={formAction} className="mt-4 space-y-5">
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-1.5">
              <Label htmlFor="class_id">Class</Label>
              <Select
                id="class_id"
                name="class_id"
                value={classId}
                onChange={(e) => onClassChange(e.target.value)}
                required
              >
                {classRows.length === 0 ? (
                  <option value="">
                    {classes === null ? "Classes unavailable" : "No assigned classes"}
                  </option>
                ) : null}
                {classRows.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="academic_year_id">Academic year</Label>
              <Select
                id="academic_year_id"
                name="academic_year_id"
                value={yearId}
                onChange={(e) => setYearId(e.target.value)}
                required
              >
                {yearRows.length === 0 ? (
                  <option value="">
                    {years === null ? "Years unavailable" : "No academic years"}
                  </option>
                ) : null}
                {yearRows.map((y) => (
                  <option key={y.id} value={y.id}>
                    {y.name}
                    {y.is_current ? " (current)" : ""}
                  </option>
                ))}
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="date">Date</Label>
              <Input id="date" name="date" type="date" required defaultValue={today} />
            </div>
          </div>

          {rosterLoading || roster === undefined ? (
            <div className="flex items-center justify-center gap-2 rounded-[var(--radius-sm)] border border-dashed border-[var(--border)] p-8 text-sm text-[var(--muted-foreground)]">
              <LoaderCircle className="size-4 animate-spin" /> Loading the assigned class register…
            </div>
          ) : roster === null ? (
            <EmptyState
              icon={<AlertTriangle className="size-8" />}
              title="Class register unavailable"
              description="The roster could not be loaded. Attendance cannot be submitted until the authoritative register is available."
            />
          ) : roster.length > 0 ? (
            <div className="divide-y divide-[var(--border)] rounded-[var(--radius-sm)] border border-[var(--border)]">
              {roster.map((s) => (
                <div key={s.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">
                      {s.first_name} {s.last_name}
                    </p>
                    <p className="text-xs text-[var(--muted-foreground)]">{s.student_code}</p>
                  </div>
                  <Select
                    aria-label={`Status for ${s.first_name} ${s.last_name}`}
                    name={`status_${s.id}`}
                    defaultValue="present"
                    data-roster
                    className="h-9 w-32"
                  >
                    {STATUS_OPTIONS.map((status) => (
                      <option key={status} value={status}>
                        {status.charAt(0).toUpperCase() + status.slice(1)}
                      </option>
                    ))}
                  </Select>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState
              icon={<ClipboardList className="size-8" />}
              title={classId ? "No active learners" : "No assigned class"}
              description={
                classId
                  ? "Active learners will appear here once they are enrolled in this class."
                  : "An assigned class is required before attendance can be marked."
              }
            />
          )}

          <div className="flex justify-end">
            <Button
              type="submit"
              loading={pending}
              loadingLabel="Saving"
              disabled={
                !roster ||
                roster.length === 0 ||
                !classId ||
                !yearId ||
                classes === null ||
                years === null
              }
            >
              <CalendarCheck className="size-4" />
              Save attendance
            </Button>
          </div>

          {state.error ? (
            <p role="alert" className="text-sm text-[var(--color-crit)]">
              {state.error}
            </p>
          ) : null}
          {state.success ? (
            <p role="status" className="text-sm text-[var(--color-ok)]">
              Attendance saved.
            </p>
          ) : null}
        </form>
      </section>
    </div>
  );
}
