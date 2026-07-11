"use client";

import * as React from "react";

export interface FeatureFlag {
  feature_key: string;
  is_enabled: boolean;
  plan_required?: string;
}

export interface FeatureSnapshot {
  tenantCode: string;
  flags: FeatureFlag[];
}

export interface FeatureFlagsProviderProps {
  snapshot: FeatureSnapshot;
  children: React.ReactNode;
}

const FeatureContext = React.createContext<FeatureSnapshot | null>(null);

export function FeatureFlagsProvider({ snapshot, children }: FeatureFlagsProviderProps) {
  return <FeatureContext.Provider value={snapshot}>{children}</FeatureContext.Provider>;
}

export function useFeatureSnapshot(): FeatureSnapshot | null {
  return React.useContext(FeatureContext);
}

function buildFlagMap(snapshot: FeatureSnapshot | null): Map<string, FeatureFlag> {
  const map = new Map<string, FeatureFlag>();
  if (!snapshot) return map;
  for (const flag of snapshot.flags) map.set(flag.feature_key, flag);
  return map;
}

export function isFeatureEnabled(snapshot: FeatureSnapshot | null, key: string): boolean {
  return buildFlagMap(snapshot).get(key)?.is_enabled ?? false;
}

export function useFeature(key: string): boolean {
  const snapshot = useFeatureSnapshot();
  return isFeatureEnabled(snapshot, key);
}

/**
 * Like `useFeature`, but treats a missing/null snapshot as "enabled".
 * Use for minimal shells where the feature-flag backend may not yet be returning
 * flags for every module.
 */
export function useFeatureStubbed(key: string): boolean {
  const snapshot = useFeatureSnapshot();
  if (!snapshot || snapshot.flags.length === 0) return true;
  return isFeatureEnabled(snapshot, key);
}

export interface FeatureGateProps {
  feature: string;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export function FeatureGate({ feature, children, fallback = null }: FeatureGateProps) {
  const enabled = useFeature(feature);
  return enabled ? <>{children}</> : <>{fallback}</>;
}

export interface FeatureGateStubProps {
  feature: string;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export function FeatureGateStub({ feature, children, fallback = null }: FeatureGateStubProps) {
  const enabled = useFeatureStubbed(feature);
  return enabled ? <>{children}</> : <>{fallback}</>;
}

export interface FeatureDisabledProps {
  feature: string;
  title?: string;
  description?: string;
  action?: React.ReactNode;
}

export function FeatureDisabled({
  feature,
  title = "Feature unavailable",
  description = "This feature is not enabled for your school.",
  action,
}: FeatureDisabledProps) {
  const snapshot = useFeatureSnapshot();
  const flag = snapshot ? buildFlagMap(snapshot).get(feature) : undefined;
  const upgradeHint = flag?.plan_required
    ? ` Upgrade to the ${flag.plan_required} plan to unlock it.`
    : "";

  return (
    <div role="status" className="rounded-lg border border-border bg-surface p-6 text-center">
      <h3 className="font-display text-lg font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground">
        {description}
        {upgradeHint}
      </p>
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}
