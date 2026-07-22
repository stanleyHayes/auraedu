"use client";

import { usePathname } from "next/navigation";
import { FeatureDisabled } from "@auraedu/flags";
import type { FeatureFlag } from "@auraedu/shared-types";
import { checkRouteFeature } from "@/lib/features";

/**
 * Route feature gate that re-evaluates on every navigation. Layouts are
 * preserved across client-side navigations, so a server-side pathname check in
 * a layout only runs on the first full-page load — a soft navigation would
 * reach a disabled feature unchecked. As a client component inside the
 * persistent layout, this guard re-renders with the new pathname on every
 * soft navigation and swaps the page for the FeatureDisabled notice (agent_plan
 * §2 rule 6: disabled features stay hidden in the frontend).
 */
export function RouteFeatureGuard({
  flags,
  children,
}: {
  flags: FeatureFlag[];
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const routeFeature = checkRouteFeature(pathname ?? "", flags);
  if (!routeFeature.enabled) {
    return <FeatureDisabled feature={routeFeature.feature!} />;
  }
  return <>{children}</>;
}
