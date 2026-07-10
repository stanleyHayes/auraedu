# AuraEDU — Design System & UX Specification

**Version:** 1.0
**Status:** Mandatory for all frontend work (EP-06 Web Shell, EP-46 Design System, EP-40–EP-45 portals)
**Distilled from two shipping reference codebases:** `aura` (Ashesi "AURA" Classroom Booking System — Next.js 16 + Tailwind v4 + shadcn/Radix, pure-CSS motion) and `uposa` (UPOSA alumni monorepo — Vite/React + Tailwind v4 + Framer Motion + daisyUI).

> **Why this document exists.** Both reference apps independently arrived at the *same* signature UX: a View-Transitions circular theme reveal, a bespoke spotlight product tour driven by `data-tour` attributes, Web-Speech-API page narration, rich icon+title+description dropdown rows, a collapsible grouped sidebar with tree-connector lines and persisted state, skeletons instead of spinners, and reduced-motion honoured everywhere. That convergence is the strongest possible signal that these are the **AuraEDU house style**. This spec makes them mandatory and shows how to build each one — **re-skinned per tenant**, which is the one thing neither reference app had to solve.

---

## Table of Contents
1. [Principles](#1-principles)
2. [Stack](#2-stack)
3. [Per-Tenant Theming Architecture](#3-per-tenant-theming-architecture) ← the AuraEDU-specific part
4. [Design Tokens](#4-design-tokens)
5. [Motion System & Animation Policy](#5-motion-system--animation-policy)
6. [Theme Toggle — circular reveal](#6-theme-toggle--circular-reveal)
7. [Navigation — collapsible grouped sidebar](#7-navigation--collapsible-grouped-sidebar)
8. [Top bar — user menu & notifications](#8-top-bar--user-menu--notifications)
9. [PageHeader + help popover](#9-pageheader--help-popover)
10. [“Show me around” — product tour](#10-show-me-around--product-tour)
11. [Text-to-speech page guide](#11-text-to-speech-page-guide)
12. [Buttons — wave-dot loading](#12-buttons--wave-dot-loading)
13. [Skeletons — no spinners](#13-skeletons--no-spinners)
14. [Empty states](#14-empty-states)
15. [Pagination](#15-pagination)
16. [Overview / dashboard](#16-overview--dashboard)
17. [Auth pages](#17-auth-pages)
18. [Public website (marketing) motion](#18-public-website-marketing-motion)
19. [Accessibility & reduced-motion checklist](#19-accessibility--reduced-motion-checklist)
20. [Component inventory → plan mapping](#20-component-inventory--plan-mapping)
21. [Appendix — concrete values](#appendix--concrete-values-copy-these)

---

## 1. Principles

1. **Tokens, never hex.** Components reference semantic CSS variables (`--primary`, `--ring`, `--accent`, `--foreground`, `--border`), never literal colours. This is what makes per-tenant re-skinning free (§3). *No component ever hardcodes a brand colour* — the same rule as the plan's "no school-specific hardcoding".
2. **One pattern, one component.** `AppSidebar`, `PageHeader`, `EmptyState`, `UserMenu`, `ThemeToggle`, `Button`, `Skeleton*`, `DataPagination` are shared components in `packages/ui` — never re-implemented per page or per portal.
3. **Motion is purposeful and always optional.** Default 150–250 ms; one easing (`--ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1)`). Every animation degrades to instant/static under `prefers-reduced-motion` via a **global CSS kill-switch** plus `motion-safe:` guards.
4. **Skeletons over spinners.** Loading states mirror the real layout; no "Loading…" text, no bare circular spinners in app surfaces.
5. **Feature-aware everywhere.** Nav items, routes, dashboard tiles, and menu entries are gated by `useFeature(key)` / `<FeatureGate>` — a disabled module never renders (ties to plan §2 rule 6, EP-02 `packages/feature-flags`).
6. **Accessibility is non-negotiable (WCAG 2.2 AA):** full keyboard operability, visible focus ring (`--ring`), correct ARIA, contrast ≥ 4.5:1, screen-reader labels, decorative elements `aria-hidden`.
7. **Responsive:** sidebar on desktop ≥ 1024 px; collapses to a Radix Sheet drawer below. Touch targets ≥ 44 px.

---

## 2. Stack

Matches `agent_plan.md` §3 exactly, confirmed by the `aura` reference app.

| Concern | Web (`apps/web`, `apps/marketing`) | Mobile (`apps/mobile`) |
|---|---|---|
| Framework | Next.js 16 (App Router) + React 19 | **Expo SDK 54+ / React Native 0.81+**, Expo Router |
| Styling | **Tailwind CSS v4** (CSS-first `@theme`), OKLCH | **NativeWind v4** (Tailwind for RN), **hex** token mirror |
| Primitives | **shadcn/ui + Radix** (`@radix-ui/react-*`) | RN primitives + `packages/ui-native` |
| Icons | `lucide-react` | `lucide-react-native` |
| Motion | **Pure CSS** + **View Transitions API** (portals); **Framer Motion** on public/marketing only (§18) | RN `Animated`/Reanimated; `expo-router` shared-element transitions |
| TTS | native `window.speechSynthesis` | `expo-speech` |
| Push | (web push optional) | **`expo-notifications`** → Notification Service |
| Fonts | `next/font` → **Outfit** | `expo-font` → Outfit |
| Theme state | custom (localStorage + no-FOUC inline script) — **not** `next-themes` | `AsyncStorage` + `useColorScheme` |
| Media | **Cloudinary** (`next-cloudinary`) | **Cloudinary** (`cloudinary-react-native` / signed URLs) |

> **Motion policy in one line:** dashboards and portals ship *zero* animation-library JS (CSS + View Transitions only, like `aura`); Framer Motion is allowed *only* in `app/(public)/` and `apps/marketing` for scroll-reveal/parallax (like `uposa` marketing). This keeps the authenticated app bundle lean.
>
> **Shared design source:** both web and mobile read the **same design tokens** from `packages/tokens` — web consumes the OKLCH values, mobile consumes a **hex mirror** (NativeWind/RN can't evaluate `oklch()`, exactly the split the `aura` repo already ships in `dark-tints.ts`). Change a token once, both platforms follow.

---

## 3. Per-Tenant Theming Architecture

**This is the part the reference apps didn't have to solve.** `aura` hardcodes Ashesi maroon; `uposa` hardcodes navy/gold. AuraEDU serves many schools, each with its own logo and colours (spec §1, §10), so the palette must be **injected at runtime per tenant** — with *zero* component changes. The trick: everything is already a CSS variable, so we just override the variables.

### 3.1 Three layers

```
┌ Layer 1 — STRUCTURE (fixed, ships in packages/ui) ───────────────┐
│ tokens.css @theme: type scale, radius, spacing, motion easing +  │
│ keyframes, neutral ink/paper ramps. Brand slots declared as vars │
│ with sensible DEFAULTS: --brand, --brand-dark, --brand-tint …    │
├ Layer 2 — SEMANTIC (fixed, shadcn binding in globals.css) ───────┤
│ --primary: var(--brand);  --ring: var(--brand);                  │
│ --accent: var(--brand-tint);  active-nav/link/focus = --brand    │
│ light `:root` + dark `.dark` both defined                        │
├ Layer 3 — TENANT OVERRIDE (per request, injected by web shell) ──┤
│ :root{ --brand:<tenant primary>; --brand-dark:…; --brand-tint:… }│
│ derived shades via color-mix(); logo + display font also swapped │
└──────────────────────────────────────────────────────────────────┘
```

Because Layer 2 points `--primary`/`--ring`/`--accent` at the Layer-1 brand vars, **overriding the three brand vars in Layer 3 re-skins the entire app** — buttons, active nav, focus rings, badges, the sidebar indicator, the maroon-tinted PageHeader icon chip — all follow automatically. No component knows which tenant it is.

### 3.2 Tenant brand model
Tenant Service (`EP-05`, `tenant_db` branding) stores a tiny palette per tenant. Keep it minimal and **derive** the rest so schools only pick 1–2 colours:

```jsonc
// GET /api/v1/tenant/branding  →
{
  "logo_url": "https://…/upshs/logo.svg",
  "brand": { "primary": "#7B1113", "secondary": "#1E7D52" },  // hex from the school
  "display_font": "Outfit"          // optional; defaults to Outfit
}
```

At **onboarding** (plan EP-52), convert the hex to OKLCH and precompute `dark`/`tint` so runtime is a pure variable set. Derive with `color-mix` so a single primary yields a full ramp:

```css
:root {
  --brand:       oklch(0.377 0.14 27);                                  /* tenant primary  */
  --brand-dark:  color-mix(in oklch, var(--brand) 82%, black);          /* hovers/pressed  */
  --brand-tint:  color-mix(in oklch, var(--brand) 14%, var(--color-paper-50)); /* chips/bg */
  --brand-contrast: oklch(0.985 0 0);                                   /* text on brand   */
}
```

### 3.3 Injection without a flash of default theme (FOUC)
Two mechanisms, exactly mirroring `aura`'s no-FOUC pattern but tenant-aware:

1. **Server-rendered `<style>`** — the root layout resolves the tenant (middleware, plan `AURA-6.1`), fetches branding (cached in Redis/Edge Config), and emits an inline `<style id="tenant-theme">:root{--brand:…;--brand-dark:…;--brand-tint:…}</style>` **in `<head>` before any painted content**. SSR therefore paints the correct brand on first frame.
2. **Blocking inline script for mode** — a tiny IIFE (also in `aura`) sets the light/dark class before hydration:

```html
<!-- app/layout.tsx, injected before React -->
<script dangerouslySetInnerHTML={{ __html: `
  (function () {
    try {
      var m = localStorage.getItem('auraedu-theme')
        || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
      var r = document.documentElement;
      r.classList.toggle('dark', m === 'dark');
      r.style.colorScheme = m;
    } catch (e) {}
  })();
`}} />
```
`<html suppressHydrationWarning>` because the script mutates the element before hydration.

### 3.4 Mode × (optional) tenant tint
- **Mode** = `.dark` class on `<html>` (light/dark), stored in `localStorage['auraedu-theme']`, seeded from `prefers-color-scheme`. Same contract `next-themes` uses, but hand-rolled (no dependency), matching `aura`.
- `aura` additionally supports 9 named **dark tints** via `data-dark-tint`. For AuraEDU the tenant brand hue *is* the tint, so we don't ship a 9-tint picker; instead **dark surfaces derive from the tenant hue** with `color-mix` (e.g. `--card` in dark = `color-mix(in oklch, var(--brand) 8%, var(--color-ink-900))`). Optionally expose a per-user "reduce brand tint in dark" toggle later.

### 3.5 Storage & sync keys
`auraedu-theme` (mode). Cross-tab + same-tab sync via a `storage` listener plus a custom `auraedu-theme-change` event dispatched by `ThemeToggle` (the `aura` pattern). Tenant brand is **not** in localStorage (it comes from the server per tenant) so switching schools never shows a stale palette.

---

## 4. Design Tokens

Author in Tailwind v4 `@theme` (`packages/ui/src/styles/tokens.css`). Use **OKLCH** so light/dark/tenant hues stay perceptually matched. Structure below is lifted from `aura` and generalised (maroon → `--brand`).

```css
@theme {
  /* Typography */
  --font-sans: var(--font-outfit), ui-sans-serif, system-ui, sans-serif;
  --font-mono: ui-monospace, SFMono-Regular, Menlo, monospace;
  --text-xs: 0.75rem; /* … */ --text-4xl: 2.25rem; --text-5xl: 3.25rem;
  --radius-sm: 0.375rem; --radius-lg: 0.625rem; --radius-2xl: 1.5rem;

  /* Brand slots — DEFAULTS; Layer-3 overrides per tenant */
  --brand:        oklch(0.55 0.13 255);   /* neutral default until a tenant loads */
  --brand-dark:   oklch(0.46 0.12 255);
  --brand-tint:   oklch(0.93 0.03 255);
  --brand-contrast: oklch(0.985 0 0);

  /* Warm ink neutrals (text) */
  --color-ink-50: oklch(0.97 0.004 60); /* … */ --color-ink-950: oklch(0.252 0.006 45);
  /* Warm paper neutrals (surfaces) */
  --color-paper-50: oklch(0.988 0.003 106); /* app bg */
  --color-paper-300: oklch(0.90 0.004 80);  /* border */
  --color-paper-600: oklch(0.55 0.006 60);  /* muted text */
  --color-paper-950: oklch(0.20 0.006 60);

  /* Status — kept DISTINCT from brand so approved/pending/rejected never read as brand */
  --color-success: oklch(0.525 0.11 159);
  --color-warning: oklch(0.614 0.13 70);
  --color-danger:  oklch(0.5 0.182 30);
  --color-info:    oklch(0.55 0.13 255);

  /* Motion */
  --ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1);
  --animate-fade-in:  fade-in  160ms var(--ease-out-quart);
  --animate-slide-up: slide-up 200ms var(--ease-out-quart);
}
```

shadcn semantic binding (`packages/ui/src/styles/globals.css`) — the **only** file that references brand vars; components use the semantic names:

```css
@import "tailwindcss";
@import "./tokens.css";
@custom-variant dark (&:where(.dark, .dark *));   /* class-based dark, no next-themes */

@theme inline {                 /* map shadcn names → indirection vars */
  --color-background: var(--background);  --color-foreground: var(--foreground);
  --color-primary: var(--primary);        --color-primary-foreground: var(--primary-foreground);
  --color-accent: var(--accent);          --color-ring: var(--ring);
  --color-border: var(--border);          --radius: var(--radius-lg);
}
:root {                          /* LIGHT */
  --background: var(--color-paper-50); --foreground: var(--color-ink-950);
  --card: oklch(1 0 0);
  --primary: var(--brand); --primary-foreground: var(--brand-contrast);
  --accent: var(--brand-tint); --ring: var(--brand);
  --border: var(--color-paper-300);
}
.dark {                          /* DARK — surfaces derive from tenant hue */
  --background: var(--color-paper-950); --foreground: var(--color-paper-50);
  --card: color-mix(in oklch, var(--brand) 8%, var(--color-ink-950));
  --primary: var(--brand); --primary-foreground: var(--brand-contrast);
  --accent: color-mix(in oklch, var(--brand) 24%, var(--color-ink-900));
  --ring: var(--brand-tint); --border: color-mix(in oklch, var(--brand) 12%, var(--color-ink-900));
}
```

---

## 5. Motion System & Animation Policy

- **One easing constant** for everything: `--ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1)`. (Only the button wave uses `ease-in-out`.)
- **Keyframe catalogue** (ship in `tokens.css`): `fade-in` (opacity), `slide-up` (opacity+translateY 0.5rem), `page-enter` (opacity+translateY 0.75rem, 360 ms), `button-wave` (§12), `empty-state-float` (§14), `splash-progress`, plus the theme-reveal (§6).
- **Global reduced-motion kill-switch** — the single most important rule; put it in `globals.css`:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
    scroll-behavior: auto !important;
  }
}
```
- **Four layers of reduced-motion defense** (from `aura`): (1) the global kill-switch above; (2) `motion-safe:` Tailwind variant on discretionary animations (button wave, page-enter); (3) component `@media` blocks that set `animation: none` (empty-icon, splash, theme reveal); (4) JS guards (`matchMedia('(prefers-reduced-motion: reduce)')`) before `startViewTransition` and before smooth tour scrolling.
- **Route transition:** wrap route content in `motion-safe:animate-[page-enter_360ms_var(--ease-out-quart)]` inside each route group's `template.tsx` (Next re-instantiates `template.tsx` per navigation, so it replays — `layout.tsx` would not).

---

## 6. Theme Toggle — circular reveal

A light/dark toggle in the top bar. On click, the new theme **wipes in as a circle expanding from the cursor** via the View Transitions API. Verbatim technique from `aura` (`uposa` uses the same approach with `documentElement.animate`).

```tsx
function toggle(e: React.MouseEvent<HTMLButtonElement>) {
  const next = !dark;
  const root = document.documentElement;
  root.style.setProperty("--theme-reveal-x", `${e.clientX}px`);
  root.style.setProperty("--theme-reveal-y", `${e.clientY}px`);

  const canVT = typeof document.startViewTransition === "function"
    && !matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (!canVT) { applyTheme(next); return; }              // instant fallback

  root.classList.add("theme-reveal");
  const t = document.startViewTransition(() => applyTheme(next));
  void t.finished.finally(() => root.classList.remove("theme-reveal"));
}
// applyTheme: toggle('.dark'), set colorScheme, write localStorage['auraedu-theme'],
//             dispatch new Event('auraedu-theme-change'), setState.
```
```css
html.theme-reveal::view-transition-new(root) {
  animation: aura-theme-reveal 480ms var(--ease-out-quart);   /* aura uses 800ms; 400–500ms feels snappier */
}
html.theme-reveal::view-transition-old(root) { animation: none; }  /* new theme circles over the old */
@keyframes aura-theme-reveal {
  from { clip-path: circle(0        at var(--theme-reveal-x) var(--theme-reveal-y)); }
  to   { clip-path: circle(150vmax  at var(--theme-reveal-x) var(--theme-reveal-y)); }
}
@media (prefers-reduced-motion: reduce) {
  html.theme-reveal::view-transition-new(root) { animation: none; }
}
```
Button: `role=button`, `aria-pressed={dark}`, `aria-label` toggles "Switch to light/dark theme", Sun/Moon lucide swap, maroon→brand focus ring.

---

## 7. Navigation — collapsible grouped sidebar

Left sidebar on desktop; Radix Sheet drawer on mobile. This is the richest pattern; build it as `AppSidebar` (+ `NavGroup`, `NavItem`, `NavConnector`) in `packages/ui`, fed **server-side** with permission- and feature-filtered nav data.

### 7.1 Data model & server filtering
Nav sections are declared in the (server) route layout and filtered before render so **disabled/unpermitted groups never reach the client** (ties to plan RBAC + feature-flag rules). Icons cross the server→client boundary as **string keys** resolved via a client registry (functions can't be serialized):

```ts
const ICONS = { dashboard: LayoutDashboard, students: Users, attendance: CalendarCheck, /*…*/ };
type IconKey = keyof typeof ICONS;
interface NavItem { href: string; label: string; icon: IconKey; exact?: boolean;
                    permission?: string; feature?: string }
interface NavSection { heading?: string; icon?: IconKey; items: NavItem[] }

// server: drop items lacking permission OR whose feature flag is off, then drop empty groups
const sections = all
  .map(s => ({ ...s, items: s.items.filter(i =>
      (!i.permission || perms.has(i.permission)) &&
      (!i.feature    || features.enabled(i.feature)) ) }))
  .filter(s => s.items.length > 0);
```

### 7.2 Collapse to icon-rail (persisted)
- Widths: expanded `w-72` (288 px), collapsed `w-20` (80 px). Animate `transition-[width] duration-200 ease-out`.
- Toggle button shows `ChevronsLeft`/`ChevronsRight` (« / »), `aria-pressed={collapsed}`, dynamic `aria-label`. Logo drops its wordmark when collapsed.
- Collapsed rail = icon-only 44 px (`size-11`) targets with native `title` tooltip + `sr-only` label.
- Persist to `localStorage['auraedu-sidebar-collapsed']` (string), lazy-init in `useState` (SSR-guard `typeof window`). *Known caveat:* SSR renders expanded, client may flip on hydrate → cookie-drive the initial width if the flash matters.

### 7.3 Collapsible groups (persisted, auto-open active)
- Group header is a real `<button aria-expanded aria-controls>`; chevron `ChevronDown` rotates `-rotate-90` when closed.
- **Animate open/close with the CSS grid-rows trick** (no JS height measuring):

```tsx
<div className={cn("grid transition-[grid-template-rows,opacity] duration-200 ease-out",
  open ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0")}>
  <div className="min-h-0 overflow-hidden">{items}</div>
</div>
```
- Per-group state in `localStorage['auraedu-nav-groups']` (JSON `Record<id,boolean>`, **default open**). The group containing the active route is **force-open** regardless of stored state: `const open = hasActiveItem || (stored[id] ?? true)`.

### 7.4 The connector line (curved tree elbow) — required
Each item under a group shows an SVG "branch" from the group trunk into the item. **Inline SVG** (preferred over CSS `::before` for the smooth curve + active-colour transition). Geometry straight from `aura`:

```tsx
<svg viewBox="0 0 24 40" preserveAspectRatio="none" aria-hidden
  className={cn("absolute left-1 top-0 h-full w-5 text-[var(--border)] transition-colors",
                active && "text-[var(--primary)]")}>
  {isLast
    ? <path d="M7 0 V17 Q7 23 13 23 H24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
    : <path d="M7 0 V40 M7 23 Q7 23 13 23 H24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>}
</svg>
```
- Trunk at x=7, elbow turns at y=23 via quadratic `Q7 23 13 23` (soft ~6-unit corner), branch to x=24. `preserveAspectRatio="none"` + `h-full` on a `min-h-10` row maps the 40-unit viewBox 1:1 to a 40 px row.
- **Last item** stops at the elbow (`V17` then curve); **middle items** run the trunk full height (`V40`) plus the branch.
- Colour: `--border` at rest → `--primary` (tenant brand) when the item is active, animated by `transition-colors`.
- *Lighter alternative* (from `uposa`): an L-shaped `::before` — `before:-left-[13px] before:h-1/2 before:w-3 before:border-b before:border-l before:border-[color-mix(in_oklch,var(--brand)_30%,transparent)]`. Use SVG when you want the curve; use `::before` when you want zero markup.

### 7.5 Active state
Active link: `bg-[var(--accent)] text-[var(--primary)] shadow-sm` + an absolutely-positioned brand pill `absolute left-0 top-2 h-6 w-1 rounded-r-full bg-[var(--primary)]`. `aria-current="page"`. Active detection: `item.exact ? pathname === href : pathname === href || pathname.startsWith(href + "/")`.

### 7.6 Mobile drawer
Radix `Sheet` (Dialog) → free focus-trap, Esc-close, scroll-lock, focus restore. `left` side, `w-72 h-dvh`. **Close on navigate** imperatively: pass `onNavigate={() => setMobileOpen(false)}` to each link's `onClick` (desktop rail passes none, so it stays put). Overlay `fade-in 160ms`, `z-210` / content `z-211`.

### 7.7 Notification badges in nav (optional, from `uposa`)
A nav item may carry `notifTypes: string[]`; count matching unread notifications and render a red count pill (or a dot when the rail is collapsed). Nice for "Approvals", "Messages".

---

## 8. Top bar — user menu & notifications

### 8.1 UserMenu (rich rows) — Radix DropdownMenu on the avatar
Every row is **icon + title + description** (this is the headline pattern both apps share). Build one `MenuRow`:

```tsx
function MenuRow({ icon: Icon, title, description }) {
  return (<>
    <Icon className="mt-0.5 size-4 shrink-0" aria-hidden />
    <span className="flex min-w-0 flex-col">
      <span className="text-sm font-medium text-[var(--foreground)]">{title}</span>
      <span className="text-xs leading-4 text-[var(--muted-foreground)]">{description}</span>
    </span>
  </>);
}
```
- Trigger: avatar button with initials fallback, `aria-label="Account menu"`.
- Content `align="end" w-80`; a non-interactive header label (name, truncated email, role label); `DropdownMenuSeparator`; then rows with `className="items-start gap-3 p-3"` (top-aligns the icon to two-line text). Navigation rows use `asChild` + Next `<Link>`.
- **Rows:** Profile · Settings · User guide · **Replay tour** (`onSelect={dispatchReplayTour}`) · —— · Sign out.
- **Sign-out gotcha:** call `event.preventDefault()` in `onSelect` so Radix doesn't close the menu before the async logout POST runs; then hard-navigate (`window.location.assign('/login')`) to discard React Query/session state. **Replay tour does NOT preventDefault** — let the menu close, then the tour opens.
- The menu deep-links into the correct area by role/route (e.g. `/admin/*` vs `/app/*`).

### 8.2 NotificationsBell — Radix Popover
Bell button with dynamic `aria-label` ("Notifications, N unread"), an `aria-hidden` badge capped "9+". Content `align="end" w-80 p-0`: header + "Mark all read", scrollable `max-h-80` list with per-type coloured icon tiles, empty state ("You're all caught up."), footer "View all". Live updates via `EventSource('/api/v1/notifications/stream')` (or 30 s poll) invalidating a React Query key. In AuraEDU this is **tenant-scoped** — the stream/endpoint already carries tenant context from the gateway.

### 8.3 Top-bar `data-tour` anchors
Wrap each control so the product tour (§10) can target it: `<span data-tour="theme-toggle">`, `data-tour="notifications"`, `data-tour="user-menu"`; the sidebar gets `data-tour="desktop-navigation"`, the hamburger `data-tour="mobile-navigation"`.

---

## 9. PageHeader + help popover

Every page opens with `<PageHeader icon title description help action />`.

- **Icon chip:** `grid size-14 place-items-center rounded-2xl bg-[var(--accent)] text-[var(--primary)]` holding a lucide icon (`aria-hidden` — the `<h1>` carries meaning). Re-skins per tenant automatically (accent = brand tint).
- **Title** `<h1>` (Outfit 600), **description** one sentence (`data-page-description` hook).
- **Help "?"** — a `HelpCircle` button beside the title opening a Radix Popover with a **numbered "how to…" walkthrough** (`<ol>`). `aria-describedby` points at a hidden transcript for AT. The popover footer hosts the TTS control (§11).
- **Primary action** right-aligned (the page's main verb; omit on read-only pages).
- **Auto-wire:** if `help` is omitted, derive it from a route registry (`getPageGuide({ title, description })`) so *every* page gets contextual help for free; pass explicit `help` to override.
- Root carries `data-tour="page-header"`; actions carry `data-tour="primary-actions"`.

---

## 10. “Show me around” — product tour

A bespoke spotlight tour — **no library** (both apps confirm: no joyride/driver/shepherd/intro.js). Build as `AppTour` mounted once in the app shell.

- **Targets are DOM `data-tour` attributes**, not React refs — any component becomes a step just by adding the attribute. A step's `selector` may be an array (`['desktop-navigation','mobile-navigation']`) and the first *visible* match wins (handles desktop/mobile fork); if none found, auto-skip to the next step.
- **Spotlight without SVG masks:** position a transparent, brand-outlined box over the target with a huge outer shadow that dims everything else:
  ```tsx
  <div className="absolute rounded-lg border-2 border-[var(--primary)] bg-[color-mix(in_oklch,var(--primary)_10%,transparent)]"
       style={{ ...rect, boxShadow: "0 0 0 9999px rgba(4,6,15,0.58)" }} />
  ```
- **Smart panel placement:** below the target if `below + 220 < innerHeight` else above; clamp horizontal centre within 16 px margins; width `min(360, innerWidth-32)`. Recompute rect on mount + `resize` + capture-phase `scroll` (rAF before reading).
- **First-login auto-start, persisted per user *and tenant*:** key `auraedu-tour-complete:<mode>:<tenantId>:<userId>`. Auto-open after ~650 ms on the dashboard if the flag is unset and Save-Data is off. Finishing / "Skip and don't show again" / X all set the flag.
- **Replay via a window CustomEvent bus** (decouples the menu from the tour): `dispatchReplayTour()` fires `new CustomEvent('auraedu:replay-tour')`; `AppTour` listens and restarts at step 0. Replay does **not** clear the completion flag.
- **A11y & motion:** container `role="dialog" aria-modal="true" aria-label="AuraEDU tour"` `z-90`; keyboard `Esc`=finish, `←`/`→`=prev/next; scroll uses `behavior: reduce ? 'auto' : 'smooth'`; auto-start suppressed under `navigator.connection.saveData`.
- **Multi-tenant note:** tour step *content* can be tenant/feature-aware — only include steps for enabled modules (skip the "Fees" step for a school without `fees`).

---

## 11. Text-to-speech page guide

A per-page spoken guide via `window.speechSynthesis`, wired into the PageHeader help popover (§9). One content registry feeds **four surfaces**: the numbered `<ol>`, the spoken text, the hidden transcript, and a standalone `/guide` page.

```ts
// speak the current page's title + description + steps
function play(text: string) {
  if (!("speechSynthesis" in window)) return;      // feature-detect
  speechSynthesis.cancel();
  const u = new SpeechSynthesisUtterance(text);
  u.lang = "en-GB";                                 // British English (matches brand voice)
  u.onend = u.onerror = () => setSpeaking(false);
  setSpeaking(true); speechSynthesis.speak(u);
}
// stop: speechSynthesis.cancel(); cleanup on unmount: speechSynthesis.cancel()  (kills audio on route change)
```
- Toggle button swaps `Volume2` "Listen" ↔ `Square` "Stop" on `speaking`.
- **Spoken text = the same array that renders the `<ol>`**, `join(". ")` — audio and visible text never drift.
- **Transcript fallback:** a hidden `<div id=… data-page-guide hidden>` with the title/description/`<ol>`, referenced by the trigger's `aria-describedby` — full guide available to AT without audio.
- **Registry** (`lib/page-guides.ts`): `{ key, section, href, title, description, steps[] }[]`, resolved by title/description match (with an alias map for pages whose description has dynamic suffixes). Keep `PageHeader` dumb — it just forwards its own title/description and the registry finds the guide.
- Guide content should be **feature-aware** — don't narrate steps for disabled modules.

---

## 12. Buttons — wave-dot loading

Standard `Button` (shadcn `cva` + Radix `Slot`) gains a `loading` state rendered as **three dots animating in a wave**, width-stable.

```tsx
function ButtonWave({ label = "Loading" }) {
  return (
    <span className="inline-flex items-center gap-1" role="status" aria-live="polite">
      <span className="sr-only">{label}</span>
      {[0,1,2].map(i => (
        <span key={i} aria-hidden
          className="size-1.5 rounded-full bg-current motion-safe:animate-[button-wave_1s_ease-in-out_infinite]"
          style={{ animationDelay: `${i * 0.15}s` }} />
      ))}
    </span>
  );
}
```
```css
@keyframes button-wave {
  0%, 60%, 100% { transform: translateY(0);       opacity: 0.55; }
  30%           { transform: translateY(-0.28rem); opacity: 1; }
}
```
- Width stability: render real children `invisible` to reserve width, overlay the wave `absolute`.
- `disabled={disabled || loading}`, `aria-busy={loading}`; wrapper `role="status" aria-live="polite"` + `sr-only` label; dots `aria-hidden`.
- `motion-safe:` → reduced-motion users get three **static** dots.
- All variants (primary/secondary/ghost/destructive) support it; use for every async action (login, submit, approve, upload, save).

---

## 13. Skeletons — no spinners

One primitive + inline composition (don't over-engineer variants):

```tsx
function Skeleton({ className, ...p }) {
  return <div aria-hidden className={cn("animate-pulse rounded-md bg-[var(--muted)]", className)} {...p} />;
}
```
- Uses Tailwind's built-in `animate-pulse`; colour `--muted` flips with theme/tenant. Reduced-motion covered by the global kill-switch.
- **Per-route `loading.tsx`** renders a skeleton mirroring the page (dashboard → skeleton KPI tiles + chart panel; tables → skeleton rows; lists → `<Skeleton className="h-16 w-full"/>` ×N). Wrapper gets `aria-busy="true"`. Next.js streams it immediately on navigation.
- Compose named helpers at call sites if useful (`TableSkeleton`, `StatsSkeleton`, `CardGridSkeleton`) — `uposa` does this; keep them thin.

---

## 14. Empty states

`<EmptyState icon title description actions />` — four parts, centre-aligned, `role="status"`.
- **Animated icon:** gentle float — `.aura-empty-icon { animation: empty-state-float 2.8s var(--ease-out-quart) infinite }`, keyframe bobs `translateY(-0.35rem)` at 50%. `@media (prefers-reduced-motion) { animation: none }`.
- Title (short), description (guides to next step), 1–2 actions (primary = brand).
- Render on every empty list/result (no search results, no students, no notifications, empty report).

---

## 15. Pagination

Shared `<DataPagination>` on every list page. AuraEDU APIs are **cursor-based** (`?limit=&cursor=` → `{ data, next_cursor }`):
- Prev/Next via a **cursor stack** (push on next, pop on prev) + page-size selector (10/25/50); "Showing X–Y"; disable Next when `next_cursor` is null.
- Feed-style lists (notifications) may use "Load more" instead.
- Persist page size per list in localStorage; page-size buttons `aria-pressed`.

---

## 16. Overview / dashboard

Role-aware + feature-aware landing per portal. Three bands:
1. **KPI `StatCard` row** — label, big value, trend/sub-text, icon, links to detail; `SkeletonStat` while loading. Only tiles the user's role + tenant flags allow.
2. **Quick actions** — the role's top verbs (brand primary).
3. **Recent activity / at-a-glance** — recent items, mini chart, unread notifications, each a card linking deeper.

For AuraEDU each role (School Admin, Teacher, Parent, Student, Super Admin) gets its own Overview; tiles/actions are gated by RBAC **and** the tenant's enabled features.

---

## 17. Auth pages

Split-screen, brand-panel + form-panel, redesigned around the tenant (not a fixed brand):
- **Brand panel** (left ~45%, desktop ≥ 768 px): solid `--brand`, tenant logo + wordmark, tagline, a subtle `aria-hidden` motif. On mobile → a compact brand bar on top.
- **Form panel** (right): centred card (`max-w-[420px]`, rounded, soft shadow) on `--card` with the standard header anatomy (icon + title + description + help + action). The help "?" is wired to the TTS guide so even logged-out users get the spoken walk-through.
- Screens: **Login** (`KeyRound`), **MFA** (`ShieldCheck`), **Forgot password** (`MailQuestion`), **Reset password** (`LockKeyhole`) — each with its own how-to help.
- Forms: react-hook-form + shared zod schemas; inline field errors (`aria-describedby`); map API error codes to friendly copy; no user-enumeration on forgot-password; show/hide password + Caps-Lock hint + correct autocomplete tokens. Submit uses the §12 wave loader. Theme toggle available here too.
- **Multi-tenant:** the auth page resolves the tenant from the host and themes itself; "no public sign-up — contact your school administrator".

> Pass the auth icon as a **rendered `ReactNode`**, not a component, so a Server Component page can hand a lucide icon to the client auth header without breaking the function-prop boundary (the `aura` fix).

---

## 18. Marketing sites (company + per-school) motion

The **only** place Framer Motion is allowed. Two distinct marketing surfaces share these primitives:
- **Company site** — `apps/marketing` (auraedu.com, EP-47): AuraEDU's *own* brand (not tenant-themed) — product features, pricing, "sign up your school" funnel.
- **Per-school public sites** — `apps/web` `(public)/[tenant]` (EP-19/EP-44): each tenant-branded (admissions/news/pages), themed by §3.

Adopt `uposa` marketing's reusable Framer Motion primitives on both:
- `ScrollReveal` (`useInView({ once, margin: '-60px' })` → fade+directional translate), `StaggerChildren` (parent `staggerChildren` + child 3D tilt `rotateX 12→0`), `Reveal3D` (`useScroll`+`useTransform` scroll-linked tilt), `Parallax`/`ParallaxImg`, `AnimatedCounter` (`animate(0,value)` on inView), `PageTransition` (route-level `AnimatePresence mode="wait"`).
- **Every one** early-returns a static element under `useReducedMotion()`.
- Optional mega-menu for the public nav: a wide dropdown grid with a promo panel + icon/title/description child rows (from `uposa` marketing Header) — good for Admissions/Academics/News (school) or Product/Pricing/Company (auraedu.com).

---

## 18b. Mobile app (Expo / React Native) — EP-48

One tenant-aware Expo app for **teacher, parent, student** (no admin — user directive). It re-expresses the same design language natively via `packages/ui-native` (NativeWind) reading the **hex token mirror** from `packages/tokens`.

**What carries over vs. what changes on mobile:**

| Web pattern (§) | Mobile equivalent |
|---|---|
| Per-tenant theming (§3) | Same model — tenant brand loaded at login from Tenant Service, applied as NativeWind theme vars (hex, not OKLCH); persisted in `AsyncStorage` |
| Theme toggle circular reveal (§6) | No View Transitions on RN → **cross-fade** via Reanimated; `useColorScheme` for system default |
| Collapsible sidebar (§7) | **Bottom tab bar** (primary) + a **drawer** (`expo-router` Drawer) for secondary/grouped nav; connector lines drop (not idiomatic on mobile) |
| User menu rich rows (§8) | Same icon+title+description rows in a native sheet/menu |
| PageHeader + help (§9) | Native header + a help sheet; **`expo-speech`** replaces `speechSynthesis` for the TTS guide (§11) |
| "Show me around" tour (§10) | Native coach-marks over `data-tour`-equivalent measured views; same first-login `AsyncStorage` flag + replay |
| Wave-dot button (§12) | Reanimated three-dot wave |
| Skeletons (§13), Empty (§14), Pagination (§15) | RN skeleton (Reanimated pulse); same cursor pagination via shared `packages/api-client` |
| Notifications (§8.2) | **`expo-notifications`** push; tokens registered with Notification Service; deep-link into the relevant screen |

**Rules:** mobile consumes the **same `contracts` + `packages/api-client` + `packages/feature-flags`** as web (no new backend), so a teacher/parent/student feature is only *Done* when it ships on **both** web and mobile (plan §18 mobile-parity rule). Feature flags gate mobile screens exactly as on web (`useFeature`). Respect `AccessibilityInfo.isReduceMotionEnabled()` — the RN analogue of `prefers-reduced-motion` — for every animation. Admin surfaces are intentionally absent from the app.

---

## 18c. Media & images (Cloudinary)

All images/documents/PDFs/video go through **Cloudinary** (File Service, EP-20) — folders prefixed by tenant code, **signed uploads** only.
- **Web:** use `next-cloudinary` (`<CldImage>`/`<CldUploadWidget>`) for responsive images, on-the-fly transforms (`f_auto,q_auto`), and DPR-aware delivery. Tenant logos in the sidebar/PageHeader/auth brand panel resolve to Cloudinary `secure_url`s.
- **Mobile:** signed Cloudinary URLs with width/quality transforms sized to the device; cache via the RN image layer.
- Never expose an unsigned upload preset; the File Service issues signed upload params. PDF report cards (EP-15) are stored as Cloudinary assets and opened via a viewer.

---

## 19. Accessibility & reduced-motion checklist

Applies to every component above (verify with `axe` in CI + a manual keyboard/SR pass on sidebar, user menu/guide, pagination, theme toggle):
- [ ] Keyboard reachable; visible `--ring` focus outline (`:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px }`).
- [ ] Correct roles/labels: `aria-expanded` (groups), `aria-pressed` (collapse/theme), `aria-current="page"` (active link), `aria-busy` (loading), `aria-modal` (drawer/tour), `aria-live` (wave/status).
- [ ] Decorative elements `aria-hidden` (connector SVG, animated icons, watermarks, brand panel).
- [ ] Contrast ≥ 4.5:1 — validate each tenant's brand at onboarding (reject a primary that fails on white/paper; auto-darken or warn).
- [ ] Global `prefers-reduced-motion` kill-switch present; discretionary motion also `motion-safe:`-guarded; TTS feature-detected + cancelled on unmount; tour respects Save-Data + reduced-motion.
- [ ] Touch targets ≥ 44 px; drawer traps focus and restores on close.

---

## 20. Component inventory → plan mapping

Build these in `packages/ui` (EP-46) and consume in `apps/web` (EP-06 + portals). Proposed story additions to `agent_plan.md`:

| Component | Design § | Plan epic | Suggested story |
|---|---|---|---|
| Token layer + per-tenant theming (Layer 1–3) | §3, §4 | EP-46 / EP-06 | `AURA-46.1` (extend: OKLCH tokens + brand slots) · `AURA-6.2` (extend: inject tenant `<style>` + no-FOUC) |
| `ThemeToggle` (circular reveal) | §6 | EP-46 | `AURA-46.3` |
| `AppSidebar` + `NavGroup`/`NavItem`/`NavConnector` | §7 | EP-46 | `AURA-46.4` |
| `AppTopbar` + `UserMenu` + `NotificationsBell` | §8 | EP-46 | `AURA-46.5` |
| `PageHeader` + `PageHelp` popover | §9 | EP-46 | `AURA-46.6` |
| `AppTour` (`data-tour`, event-bus replay) | §10 | EP-06 | `AURA-6.6` (extend feature-aware nav guards) → add `AURA-6.7` |
| `useSpeech` / TTS + `page-guides` registry | §11 | EP-06 | `AURA-6.8` |
| `Button` (wave loader) | §12 | EP-46 | `AURA-46.7` |
| `Skeleton*` set + `loading.tsx` wiring | §13 | EP-46 | `AURA-46.8` |
| `EmptyState`, `DataPagination`, `StatCard`/Overview | §14–16 | EP-46 | `AURA-46.9` |
| Auth split-screen + headers | §17 | EP-06/EP-40 | `AURA-6.9` |
| Public-site Framer Motion primitives | §18 | EP-44 | `AURA-44.x` |

**Ownership:** `packages/ui` is owned by the L4 design-system lead; portal agents (L4 sub-lanes) consume these components and never fork them — new shared components land in `packages/ui` via a separate PR (matches plan §8 collision rules).

---

## Appendix — concrete values (copy these)

| Thing | Value |
|---|---|
| Easing (everything) | `--ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1)` |
| Sidebar width | expanded `w-72` (288px) · collapsed `w-20` (80px) · anim `transition-[width] duration-200 ease-out` |
| Group open/close | CSS `grid-rows-[1fr]↔[0fr]` + opacity, `duration-200 ease-out` |
| Connector SVG | `viewBox 0 0 24 40`, `preserveAspectRatio="none"`, trunk x=7, elbow `Q7 23 13 23`, branch→x=24, stroke 1.5 round |
| Active indicator | `absolute left-0 top-2 h-6 w-1 rounded-r-full bg-[var(--primary)]` |
| Theme reveal | `clip-path: circle(0 → 150vmax at cursor)`, 400–500ms (aura uses 800ms) |
| Button wave | 3 × `size-1.5` dots, `button-wave 1s ease-in-out infinite`, stagger `i*0.15s`, rise `0.28rem` |
| Skeleton | `animate-pulse rounded-md bg-[var(--muted)]` |
| Empty float | `empty-state-float 2.8s infinite`, `translateY(-0.35rem)` @50% |
| Page enter | `page-enter 360ms`, opacity 0→1 + translateY 0.75rem→0, `motion-safe:` |
| Radix surfaces | dropdown/sheet overlay `fade-in 160ms`; popover `slide-up 180ms`; dialog content `slide-up 200ms`; tooltip `fade-in 120ms` |
| Icon-rail target | `size-11` (44px) `rounded-xl`, native `title` tooltip |
| localStorage keys | `auraedu-theme` · `auraedu-sidebar-collapsed` · `auraedu-nav-groups` |
| Tour completion key | `auraedu-tour-complete:<mode>:<tenantId>:<userId>` |
| Replay event | `new CustomEvent('auraedu:replay-tour')` |
| z-index | tour `z-90`, sticky topbar `z-30`, sheet overlay `z-210` / content `z-211`, popovers/menus `z-220` |
| Font | Outfit via `next/font`, `--font-sans`, weights 400/500/600/700, `display: swap` |

---

*Every pattern here is proven in a shipping app and re-expressed for AuraEDU's per-tenant reality. Build it once in `packages/ui`, theme it per tenant via CSS variables, and every school gets a distinct, branded, accessible experience from one codebase.*
