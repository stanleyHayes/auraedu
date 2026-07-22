export interface FeatureFlag {
  feature_key: string;
  is_enabled: boolean;
  plan_required?: string;
}

export interface FeatureSnapshot {
  tenantCode: string;
  flags: FeatureFlag[];
}

export function buildFlagMap(snapshot: FeatureSnapshot | null): Map<string, FeatureFlag> {
  const map = new Map<string, FeatureFlag>();
  if (!snapshot) return map;
  for (const flag of snapshot.flags) map.set(flag.feature_key, flag);
  return map;
}

export function isFeatureEnabled(snapshot: FeatureSnapshot | null, key: string): boolean {
  return buildFlagMap(snapshot).get(key)?.is_enabled ?? false;
}
