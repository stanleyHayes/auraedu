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

/** Accessible table with ruled rows, sticky header, and row entrance animations. */
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
        "overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] shadow-sm",
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
                  "px-4 py-3 font-mono text-[10.5px] uppercase tracking-[0.12em] text-[var(--muted-foreground)]",
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
                "transition-colors hover:bg-[var(--muted)]/60 motion-safe:animate-[slide-up_0.35s_ease-out_both]",
                i !== rows.length - 1 && "border-b border-[var(--border)]",
              )}
              style={{ animationDelay: `${Math.min(i * 40, 400)}ms` }}
            >
              {columns.map((col) => (
                <td
                  key={col.key}
                  className={cn("px-4 py-3.5 text-[var(--foreground)]", col.className)}
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
