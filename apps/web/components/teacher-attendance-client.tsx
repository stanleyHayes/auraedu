"use client";

import * as React from "react";
import { CalendarCheck, ClipboardList } from "lucide-react";
import { useActionState } from "react";
import { Button, DataTable, EmptyState, type DataTableColumn } from "@auraedu/ui";
import { recordAttendance, type ActionResult } from "@/lib/actions";
import type { AttendanceRecord } from "@/app/(teacher)/teacher/attendance/page";

const STATUS_OPTIONS = ["present", "absent", "late", "excused"];

const columns: DataTableColumn<AttendanceRecord>[] = [
  { key: "student_id", header: "Student ID", cell: (r) => r.student_id },
  { key: "date", header: "Date", cell: (r) => r.date },
  { key: "status", header: "Status", cell: (r) => <span className="capitalize">{r.status}</span> },
];

interface TeacherAttendanceClientProps {
  initialRecords: AttendanceRecord[];
}

export function TeacherAttendanceClient({ initialRecords }: TeacherAttendanceClientProps) {
  const [state, formAction, pending] = useActionState<ActionResult | undefined, FormData>(
    recordAttendance,
    undefined,
  );
  const formRef = React.useRef<HTMLFormElement>(null);

  React.useEffect(() => {
    if (state?.success) {
      formRef.current?.reset();
    }
  }, [state]);

  const today = React.useMemo(() => new Date().toISOString().split("T")[0], []);

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
          Record attendance for a single student.
        </p>

        <form
          ref={formRef}
          action={formAction}
          className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4"
        >
          <div>
            <label htmlFor="student_id" className="mb-1.5 block text-sm font-medium">
              Student ID
            </label>
            <input
              id="student_id"
              name="student_id"
              type="text"
              required
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            />
          </div>
          <div>
            <label htmlFor="academic_year_id" className="mb-1.5 block text-sm font-medium">
              Academic year ID
            </label>
            <input
              id="academic_year_id"
              name="academic_year_id"
              type="text"
              required
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            />
          </div>
          <div>
            <label htmlFor="date" className="mb-1.5 block text-sm font-medium">
              Date
            </label>
            <input
              id="date"
              name="date"
              type="date"
              required
              defaultValue={today}
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            />
          </div>
          <div>
            <label htmlFor="status" className="mb-1.5 block text-sm font-medium">
              Status
            </label>
            <select
              id="status"
              name="status"
              required
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            >
              {STATUS_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s.charAt(0).toUpperCase() + s.slice(1)}
                </option>
              ))}
            </select>
          </div>
          <div className="flex items-end sm:col-span-2 lg:col-span-4">
            <Button
              type="submit"
              loading={pending}
              loadingLabel="Saving"
              className="w-full sm:w-auto"
            >
              <CalendarCheck className="size-4" />
              Save attendance
            </Button>
          </div>
        </form>

        {state?.error ? (
          <p role="alert" className="mt-4 text-sm text-[var(--color-crit)]">
            {state.error}
          </p>
        ) : null}
        {state?.success ? (
          <p role="status" className="mt-4 text-sm text-[var(--color-ok)]">
            Attendance recorded.
          </p>
        ) : null}
      </section>
    </div>
  );
}
