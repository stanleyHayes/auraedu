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
        "overflow-x-auto rounded-2xl border border-[var(--border)] bg-[var(--surface)] shadow-[0_10px_32px_color-mix(in_oklab,var(--color-navy)_5%,transparent)]",
        className,
      )}
    >
      <table className="w-full text-left text-sm">
        {caption ? <caption className="sr-only">{caption}</caption> : null}
        <thead className="bg-[color-mix(in_oklab,var(--muted)_82%,var(--portal-accent,var(--color-brand)))]">
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
                "transition-colors hover:bg-[color-mix(in_oklab,var(--muted)_76%,var(--portal-accent,var(--color-brand)))] motion-safe:animate-[slide-up_0.35s_ease-out_both]",
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
