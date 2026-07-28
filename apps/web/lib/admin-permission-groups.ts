/**
 * Client-safe helpers for the per-user permission editor. Server actions live in
 * admin-permission-actions.ts — a "use server" module may only export async functions.
 */

export interface PermissionGroup {
  resource: string;
  label: string;
  keys: string[];
}

function humanize(resource: string): string {
  return resource
    .split(".")
    .map((part) => part.replaceAll("_", " "))
    .join(" · ");
}

/**
 * Group catalogue keys by resource (everything before the final dot), so
 * `crm.lead.read` and `crm.lead.create` sit together under "crm · lead".
 */
export function groupPermissionsByResource(keys: string[]): PermissionGroup[] {
  const groups = new Map<string, string[]>();
  for (const key of keys) {
    const cut = key.lastIndexOf(".");
    const resource = cut > 0 ? key.slice(0, cut) : key;
    const list = groups.get(resource);
    if (list) {
      list.push(key);
    } else {
      groups.set(resource, [key]);
    }
  }
  return [...groups.entries()]
    .map(([resource, groupKeys]) => ({
      resource,
      label: humanize(resource),
      keys: [...groupKeys].sort(),
    }))
    .sort((a, b) => a.label.localeCompare(b.label));
}

/** Short action label for a key within its group, e.g. `read` from `students.read`. */
export function permissionActionLabel(key: string): string {
  const cut = key.lastIndexOf(".");
  return (cut > 0 ? key.slice(cut + 1) : key).replaceAll("_", " ");
}
