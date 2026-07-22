import * as React from "react";
import { cn } from "../lib/cn";
import { Skeleton } from "./skeleton";

export function PageHeaderSkeleton({ className }: { className?: string }) {
  return (
    <div
      aria-hidden="true"
      className={cn(
        "relative overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-sm sm:p-7",
        className,
      )}
    >
      <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[var(--portal-accent,var(--color-brand))] via-[var(--portal-accent-soft,var(--color-teal-bright))] to-[var(--portal-signal,var(--color-signal))] opacity-60" />
      <Skeleton className="size-12 rounded-xl" />
      <Skeleton className="mt-5 h-2.5 w-28" />
      <Skeleton className="mt-3 h-8 w-full max-w-md" />
      <Skeleton className="mt-3 h-4 w-full max-w-2xl" />
    </div>
  );
}

export function StatsSkeleton({ count = 4, className }: { count?: number; className?: string }) {
  return (
    <div aria-hidden="true" className={cn("grid gap-4 sm:grid-cols-2 lg:grid-cols-4", className)}>
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="card relative overflow-hidden rounded-2xl p-5">
          <Skeleton className="h-2.5 w-24" />
          <Skeleton className="mt-4 h-9 w-20" />
          <Skeleton className="mt-3 h-3 w-32" />
          <span className="absolute -right-7 -top-8 size-20 rounded-full bg-[var(--portal-accent,var(--color-brand))]/5" />
        </div>
      ))}
    </div>
  );
}

export function CardGridSkeleton({ count = 2, className }: { count?: number; className?: string }) {
  return (
    <div aria-hidden="true" className={cn("grid gap-6 md:grid-cols-2", className)}>
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="card rounded-2xl p-5">
          <div className="flex items-center gap-3">
            <Skeleton className="size-10 rounded-xl" />
            <div className="flex-1">
              <Skeleton className="h-4 w-36" />
              <Skeleton className="mt-2 h-3 w-4/5" />
            </div>
          </div>
          <div className="mt-6 space-y-3">
            <Skeleton className="h-12 w-full rounded-xl" />
            <Skeleton className="h-12 w-full rounded-xl" />
            <Skeleton className="h-12 w-4/5 rounded-xl" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function TableSkeleton({ rows = 5, className }: { rows?: number; className?: string }) {
  return (
    <div aria-hidden="true" className={cn("card overflow-hidden rounded-2xl", className)}>
      <div className="flex items-center justify-between border-b border-[var(--border)] p-5">
        <div>
          <Skeleton className="h-4 w-32" />
          <Skeleton className="mt-2 h-3 w-52" />
        </div>
        <Skeleton className="h-10 w-28 rounded-xl" />
      </div>
      <div className="divide-y divide-[var(--border)] px-5">
        {Array.from({ length: rows }, (_, index) => (
          <div key={index} className="grid grid-cols-[minmax(8rem,1.4fr)_1fr_7rem] gap-6 py-4">
            <Skeleton className="h-3.5 w-full" />
            <Skeleton className="h-3.5 w-4/5" />
            <Skeleton className="h-7 w-full rounded-full" />
          </div>
        ))}
      </div>
    </div>
  );
}
