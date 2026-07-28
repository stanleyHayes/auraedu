"use client";

import * as React from "react";
import { Download, FileUp, UploadCloud } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { importStudentsAction, type AdminImportActionResult } from "@/lib/admin-import-actions";
import {
  previewCsv,
  studentImportTemplateCsv,
  STUDENT_IMPORT_HEADERS,
} from "@/lib/admin-import-template";

type ImportResult = OpenAPI.student_v1.components["schemas"]["ImportResult"];

export function AdminStudentImportDialog() {
  const [open, setOpen] = React.useState(false);
  return (
    <>
      <Button type="button" variant="secondary" onClick={() => setOpen(true)}>
        <UploadCloud className="size-4" /> Import CSV
      </Button>
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-3xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="relative overflow-hidden border-b border-[var(--border)] bg-[color-mix(in_oklab,var(--surface)_88%,var(--portal-accent-soft))] px-6 py-6">
            <span className="absolute -right-10 -top-14 size-36 rounded-full bg-[var(--portal-accent)]/10 blur-2xl" />
            <FileUp className="relative size-6 text-[var(--portal-accent)]" />
            <h2 className="relative mt-3 text-xl font-black tracking-tight">
              Bulk import students
            </h2>
            <p className="relative mt-1 max-w-xl text-sm leading-6 text-[var(--muted-foreground)]">
              Upload a CSV of learners and their guardians. Guardians sharing an email address are
              created once and linked to each of their children; every rejected row comes back with
              its row number and reason.
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <ImportWorkspace onDone={() => setOpen(false)} />
          </div>
        </div>
      </Sheet>
    </>
  );
}

function ImportWorkspace({ onDone }: { onDone: () => void }) {
  const [state, formAction, pending] = React.useActionState<AdminImportActionResult, FormData>(
    importStudentsAction,
    {},
  );
  const [csvText, setCsvText] = React.useState("");
  const [fileName, setFileName] = React.useState<string | null>(null);
  const [fileError, setFileError] = React.useState<string | null>(null);
  const fileInput = React.useRef<HTMLInputElement>(null);

  const preview = React.useMemo(() => previewCsv(csvText), [csvText]);
  const missingHeaders = React.useMemo(() => {
    if (!preview) return [];
    return ["first_name", "last_name"].filter((header) => !preview.headers.includes(header));
  }, [preview]);

  async function readFile(file: File) {
    setFileError(null);
    if (file.size > 2 * 1024 * 1024) {
      setFileError("The CSV is larger than 2 MB. Split it into smaller batches.");
      return;
    }
    setFileName(file.name);
    setCsvText(await file.text());
  }

  function downloadTemplate() {
    const blob = new Blob([studentImportTemplateCsv()], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "students-import-template.csv";
    anchor.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-dashed border-border p-4">
        <div className="flex flex-wrap items-center gap-2">
          <input
            ref={fileInput}
            type="file"
            accept=".csv,text/csv,text/plain"
            className="sr-only"
            aria-label="Choose CSV file"
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (file) void readFile(file);
              event.target.value = "";
            }}
          />
          <Button type="button" variant="secondary" onClick={() => fileInput.current?.click()}>
            <FileUp className="size-4" /> Choose CSV file
          </Button>
          <Button type="button" variant="ghost" onClick={downloadTemplate}>
            <Download className="size-4" /> Download template
          </Button>
          {fileName ? (
            <span className="text-sm font-semibold text-muted-foreground">{fileName}</span>
          ) : null}
        </div>
        <p className="mt-3 text-xs leading-5 text-[var(--muted-foreground)]">
          Expected columns: <code className="font-mono">{STUDENT_IMPORT_HEADERS.join(", ")}</code>.
          Only <code className="font-mono">first_name</code> and{" "}
          <code className="font-mono">last_name</code> are required; leave optional cells empty.
        </p>
        {fileError ? (
          <p role="alert" className="mt-2 text-sm text-red-700">
            {fileError}
          </p>
        ) : null}
      </div>

      <div className="space-y-2">
        <label htmlFor="csv_text" className="text-sm font-semibold">
          Or paste the CSV contents
        </label>
        <textarea
          id="csv_text"
          rows={6}
          value={csvText}
          onChange={(event) => {
            setCsvText(event.target.value);
            setFileName(null);
          }}
          placeholder={studentImportTemplateCsv()}
          className="w-full rounded-md border border-border bg-background px-3 py-2 font-mono text-xs leading-5 outline-none transition focus:border-primary focus:ring-2 focus:ring-primary/20"
        />
      </div>

      {preview ? (
        <div className="space-y-2">
          <div className="flex items-baseline justify-between gap-3">
            <h3 className="text-sm font-bold">
              Preview · {preview.rowCount} row{preview.rowCount === 1 ? "" : "s"} to import
            </h3>
            {missingHeaders.length > 0 ? (
              <p role="alert" className="text-xs font-semibold text-red-700">
                Missing required column{missingHeaders.length === 1 ? "" : "s"}:{" "}
                {missingHeaders.join(", ")}
              </p>
            ) : null}
          </div>
          <div className="overflow-x-auto rounded-xl border border-border">
            <table className="w-full min-w-max text-left text-xs">
              <thead className="bg-muted/60">
                <tr>
                  <th className="px-2 py-1.5 font-semibold text-muted-foreground">#</th>
                  {preview.headers.map((header) => (
                    <th key={header} className="px-2 py-1.5 font-mono font-semibold">
                      {header}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {preview.rows.map((row, rowIndex) => (
                  <tr key={rowIndex} className="border-t border-border">
                    <td className="px-2 py-1.5 font-mono text-muted-foreground">{rowIndex + 1}</td>
                    {preview.headers.map((_, cellIndex) => (
                      <td key={cellIndex} className="max-w-40 truncate px-2 py-1.5">
                        {row[cellIndex] ?? ""}
                      </td>
                    ))}
                  </tr>
                ))}
                {preview.rows.length === 0 ? (
                  <tr className="border-t border-border">
                    <td
                      colSpan={preview.headers.length + 1}
                      className="px-2 py-3 text-center text-muted-foreground"
                    >
                      The header row is present but there are no data rows to import.
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
          {preview.rowCount > preview.rows.length ? (
            <p className="text-xs text-muted-foreground">
              Showing the first {preview.rows.length} rows; all {preview.rowCount} will be
              submitted.
            </p>
          ) : null}
        </div>
      ) : null}

      {state.error ? (
        <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
          {state.error}
        </p>
      ) : null}

      {state.result ? <ImportSummary result={state.result} /> : null}

      <form action={formAction} className="flex items-center justify-end gap-3">
        <input type="hidden" name="csv_text" value={csvText} />
        <Button type="button" variant="ghost" onClick={onDone}>
          Close
        </Button>
        <Button
          type="submit"
          loading={pending}
          loadingLabel="Importing"
          disabled={!preview || preview.rowCount === 0 || missingHeaders.length > 0}
        >
          Import {preview && preview.rowCount > 0 ? `${preview.rowCount} rows` : "students"}
        </Button>
      </form>
    </div>
  );
}

function ImportSummary({ result }: { result: ImportResult }) {
  const errors = result.errors ?? [];
  const failed = errors.length;
  return (
    <div className="space-y-3 rounded-xl border border-border bg-background/60 p-4">
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Metric label="Students created" value={result.students_created} />
        <Metric label="Guardians created" value={result.guardians_created} />
        <Metric label="Links created" value={result.links_created} />
        <Metric label="Rows rejected" value={failed} danger={failed > 0} />
      </div>
      {failed > 0 ? (
        <div className="space-y-1.5">
          <p className="text-xs font-semibold text-red-700">
            Fix these rows in your CSV and re-import only them — the rows above were already
            created:
          </p>
          <ul className="max-h-48 space-y-1 overflow-y-auto text-xs leading-5">
            {errors.map((error) => (
              <li key={error.row} className="rounded-lg bg-red-500/10 px-3 py-1.5 text-red-800">
                <span className="font-mono font-bold">Row {error.row}</span> — {error.message}
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <p className="text-xs font-semibold text-emerald-700">
          Every row imported cleanly. The new learners are listed on the students page.
        </p>
      )}
    </div>
  );
}

function Metric({
  label,
  value,
  danger = false,
}: {
  label: string;
  value: number;
  danger?: boolean;
}) {
  return (
    <div
      className={`rounded-lg border px-3 py-2 ${danger ? "border-red-200 bg-red-50 text-red-900" : "border-border bg-background/70"}`}
    >
      <div className="text-lg font-bold">{value}</div>
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground">{label}</div>
    </div>
  );
}
