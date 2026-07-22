# `@auraedu/flags`

The React feature-flag SDK used by AuraEDU frontends. `FeatureFlagsProvider` supplies a tenant snapshot; `useFeature` and `FeatureGate` render only enabled production features. Stub-aware variants exist solely for routes that deliberately expose a typed placeholder.

## Usage

```tsx
<FeatureFlagsProvider snapshot={snapshot}>
  <FeatureGate feature="analytics.executive">
    <ExecutiveAnalytics />
  </FeatureGate>
</FeatureFlagsProvider>
```

Unknown, absent, or malformed flags are disabled. UI gates are not authorization: APIs must independently enforce the same capability and the actor's permission. Verify with the package `typecheck`, `lint`, and `test` scripts.
