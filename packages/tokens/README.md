# `@auraedu/tokens`

The shared visual constants for AuraEDU web and native clients. The TypeScript API exposes semantic spacing, radius, typography, motion and colour values; `tokens.css` provides the OKLCH web variables and `globals.css` applies the base browser theme.

## Rules

- Consume semantic variables instead of adding application-specific colour literals.
- Preserve equivalent hex values for React Native when changing an OKLCH web colour.
- Keep focus, contrast and reduced-motion behaviour intact across light and dark themes.
- Product applications may apply tenant branding at runtime, but must retain readable foregrounds and accessible control states.

## Usage

```ts
import { tokens, theme } from "@auraedu/tokens";
```

```css
@import "@auraedu/tokens/tokens.css";
@import "@auraedu/tokens/globals.css";
```

Verify changes with `pnpm --filter @auraedu/tokens typecheck`, `lint`, and `test`.
