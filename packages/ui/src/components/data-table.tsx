import * as React from "react";
import { cn } from "../lib/cn";

export interface DataTableColumn<T> {
  key: string;
  header: React.ReactNode;
  cell: (row: T) => React.ReactNode;
  className?: string;
}

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  rows: T[];
  keyExtractor: (row: T) => string;
  caption?: string;
  className?: string;
  empty?: React.ReactNode;
}

/** Simple accessible table with ruled rows and a sticky header (DESIGN_SYSTEM §15). */
export function DataTable<T>({
  columns,
  rows,
  keyExtractor,
  caption,
  className,
  empty,
}: DataTableProps<T>) {
  if (rows.length === 0 && empty) {
    return <>{empty}</>;
  }

  return (
    <div
      className={cn(
        "overflow-x-auto rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)]",
        className,
      )}
    >
      <table className="w-full text-left text-sm">
        {caption ? <caption className="sr-only">{caption}</caption> : null}
        <thead className="bg-[var(--muted)]">
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                scope="col"
                className={cn(
                  "px-4 py-2.5 font-mono text-[10.5px] uppercase tracking-[0.12em] text-[var(--muted-foreground)]",
                  col.className,
                )}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr
              key={keyExtractor(row)}
              className={cn(
                "transition-colors hover:bg-[var(--muted)]/50",
                i !== rows.length - 1 && "border-b border-[var(--border)]",
              )}
            >
              {columns.map((col) => (
                <td
                  key={col.key}
                  className={cn("px-4 py-3 text-[var(--foreground)]", col.className)}
                >
                  {col.cell(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
