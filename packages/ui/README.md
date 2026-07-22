# `@auraedu/ui`

The shared React design system for every authenticated AuraEDU web portal. It implements the cobalt, midnight, mineral and signal visual language while preserving tenant branding, dark mode, keyboard focus, accessible labels, reduced motion, and responsive behaviour.

Available primitives include buttons and fields, page headers, stat cards, tables, sheets, empty states, skeletons, user and notification menus, navigation, watermarking, theme controls, and the shared `Reveal` motion primitive.

## Rules

- Extend a shared primitive here when the pattern belongs in more than one portal.
- Compose product screens from semantic tokens; do not fork the marketing site or hard-code a second theme.
- Motion must communicate hierarchy or state and must have a reduced-motion path.
- Tenant colours may accent the system but cannot reduce text or control contrast.

Verify changes with the package `typecheck`, `lint`, and `test` scripts and with the consuming web production build.
