"use client";

import * as React from "react";
import { CalendarCheck, ClipboardList } from "lucide-react";
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
import { markAttendanceBulkAction, type TeacherActionResult } from "@/lib/teacher-actions";
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
  classes: AcademicClass[];
  years: AcademicYear[];
  students: Student[];
}

export function TeacherAttendanceClient({
  initialRecords,
  classes,
  years,
  students,
}: TeacherAttendanceClientProps) {
  const [state, formAction, pending] = useActionState<TeacherActionResult, FormData>(
    markAttendanceBulkAction,
    {},
  );
  const formRef = React.useRef<HTMLFormElement>(null);

  const currentYear = years.find((y) => y.is_current) ?? years[0];
  const [classId, setClassId] = React.useState(classes[0]?.id ?? "");
  const [yearId, setYearId] = React.useState(
    classes[0]?.academic_year_id ?? currentYear?.id ?? "",
  );
  const today = React.useMemo(() => new Date().toISOString().split("T")[0], []);

  function onClassChange(nextClassId: string) {
    setClassId(nextClassId);
    const nextClass = classes.find((c) => c.id === nextClassId);
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

  // The students API has no class filter, so the roster shows every active
  // student; submitted records are tagged with the selected class.
  const roster = students.filter((s) => !s.status || s.status === "active");

  return (
    <div className="space-y-6">
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-sans font-semibold tracking-tight">Attendance records</h3>
        <div className="mt-4">
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
                {classes.length === 0 ? <option value="">No classes available</option> : null}
                {classes.map((c) => (
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
                {years.length === 0 ? <option value="">No years available</option> : null}
                {years.map((y) => (
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

          {roster.length > 0 ? (
            <div className="divide-y divide-[var(--border)] rounded-[var(--radius-sm)] border border-[var(--border)]">
              {roster.map((s) => (
                <div
                  key={s.id}
                  className="flex items-center justify-between gap-4 px-4 py-2.5"
                >
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
              title="No students to mark"
              description="Active students will appear here once they are enrolled."
            />
          )}

          <div className="flex justify-end">
            <Button
              type="submit"
              loading={pending}
              loadingLabel="Saving"
              disabled={roster.length === 0 || !classId || !yearId}
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
