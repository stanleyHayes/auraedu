import Link from "next/link";
import { ArrowRight } from "lucide-react";

export interface SuperadminPaginationProps {
  /** href for the next page, or null when the current page is the last. */
  nextHref: string | null;
}

/** Cursor pagination footer for superadmin drill-down tables. */
export function SuperadminPagination({ nextHref }: SuperadminPaginationProps) {
  if (!nextHref) return null;
  return (
    <div className="flex justify-end">
      <Link
        href={nextHref}
        className="inline-flex items-center gap-2 text-sm font-bold text-[var(--primary)] hover:underline"
      >
        Next page <ArrowRight className="size-4" />
      </Link>
    </div>
  );
}
