/**
 * Shared helpers for the superadmin tenant drill-down pages
 * (apps/web/app/(superadmin)/superadmin/tenants/[code]/*). Every drill-down list is
 * cursor-paginated through the gateway with a small, honest page size.
 */

export const DRILLDOWN_PAGE_SIZE = 25;

/** Build a list-endpoint query string: limit + active filters + optional cursor. */
export function drilldownQuery(
  filters: Record<string, string | undefined>,
  cursor?: string,
): string {
  const params = new URLSearchParams();
  params.set("limit", String(DRILLDOWN_PAGE_SIZE));
  for (const [key, value] of Object.entries(filters)) {
    const trimmed = value?.trim();
    if (trimmed) params.set(key, trimmed);
  }
  if (cursor) params.set("cursor", cursor);
  return params.toString();
}

/**
 * Build the "next page" href for a drill-down page, preserving the current filters and
 * swapping in the next cursor. Returns null when there is no next page.
 */
export function nextPageHref(
  pathname: string,
  current: Record<string, string | undefined>,
  nextCursor: string | null | undefined,
): string | null {
  if (!nextCursor) return null;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(current)) {
    const trimmed = value?.trim();
    if (trimmed && key !== "cursor") params.set(key, trimmed);
  }
  params.set("cursor", nextCursor);
  return `${pathname}?${params.toString()}`;
}

/** HTML date input (YYYY-MM-DD) → inclusive ISO day start for `from` filters. */
export function dateToIsoStart(date: string | undefined): string | undefined {
  const trimmed = date?.trim();
  return trimmed ? `${trimmed}T00:00:00.000Z` : undefined;
}

/** HTML date input (YYYY-MM-DD) → inclusive ISO day end for `to` filters. */
export function dateToIsoEnd(date: string | undefined): string | undefined {
  const trimmed = date?.trim();
  return trimmed ? `${trimmed}T23:59:59.999Z` : undefined;
}

/** Compact date-time for operations tables. */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString();
}

/** Read a string field out of an audit/event metadata bag, with an em-dash fallback. */
export function metadataString(
  metadata: Record<string, unknown> | null | undefined,
  key: string,
): string {
  const value = metadata?.[key];
  return typeof value === "string" && value ? value : "—";
}
