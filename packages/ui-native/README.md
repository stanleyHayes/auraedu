# `@auraedu/ui-native`

Shared React Native presentation primitives for AuraEDU's teacher, parent and student mobile experiences. The package owns the cobalt/midnight/signal palette binding, runtime brand context, reduced-motion-aware screen transitions, ambient surfaces, page introductions, module cards, primary actions and accessible skeleton loading state.

`apps/mobile` supplies the authenticated tenant's validated primary colour to `ThemeProvider`; this package does not fetch tenants, hold sessions or implement role authorization. Feature and ownership enforcement remain in the app and backend services.

```tsx
import { Screen, PageIntro, ModuleCard } from "@auraedu/ui-native";

export function Work() {
  return (
    <Screen>
      <PageIntro eyebrow="My work" title="Ready for today" />
      <ModuleCard title="Attendance" copy="Open your assigned class." href="/(app)/attendance" />
    </Screen>
  );
}
```

Run `pnpm --filter @auraedu/ui-native typecheck`, `lint` and `test`. Mobile accessibility regression tests also scan this package so motion fallbacks and control semantics cannot silently diverge from the app.
