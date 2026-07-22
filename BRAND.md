# AuraEDU — Brand & Visual Identity

**Direction:** *Learning Orbit.* This is the source of truth for AuraEDU's own logo, colour,
typography and visual signature. [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md) defines interaction and
components; this document defines character.

> AuraEDU is an education operating system, not a digitised register. Its identity must feel
> capable enough for an institution, clear enough for a school day and alive enough to represent
> continuous learning. The system is modern and connected without looking like generic enterprise
> software or a children's learning app.

---

## 1. The idea

The mark combines three ideas:

- **The open A** is AuraEDU's initial and a stable architectural frame.
- **The learning orbit** represents the continuous flow between learners, teachers, families,
  operations and evidence.
- **The signal point** marks the moment when information becomes a clear, human-owned decision.

The lower arc keeps the symbol open rather than enclosed: schools retain their identity and
professional judgement inside the platform.

**One line:** *One school day. One dependable rhythm.*

---

## 2. Colour

AuraEDU's company identity uses **Cobalt, Midnight & Signal**. Cobalt communicates systems and
clarity; midnight gives the product institutional weight; teal connects the operational layers;
lime is reserved for a small number of decisive signals.

| Token | Hex | Use |
|---|---|---|
| Systems cobalt | `#1557FF` | Primary company action, orbit start |
| Cobalt deep | `#0B3FC7` | Hover, pressed and light-surface `EDU` |
| Midnight | `#061631` | Core ink, dark fields and mark tile |
| Teal strong | `#087F8C` | Orbit transition, supporting data |
| Teal bright | `#63D5DA` | Open lower orbit on dark surfaces |
| Signal lime | `#B7F500` | Decision point and scarce high-salience action |
| Cool paper | `#F7F9FC` | Light background |
| White | `#FFFFFF` | Surfaces and mark construction |

Signal lime is not a general-purpose fill. Use it for one decisive call to action, a live state or
the logo's signal point. Status colours remain semantic and must not be replaced with brand colour.

---

## 3. Typography

- **Marketing and product headings:** Outfit or the current product heading face. Use strong,
  compact weights and balanced line lengths.
- **Body and UI:** Outfit with system-sans fallbacks. It must remain highly legible at dense portal
  sizes.
- **Data and labels:** Spline Sans Mono or system monospace. Use tracked uppercase sparingly for
  operating labels, identifiers and the `EDU` capsule.
- **Logo:** the word `Aura` is an extra-bold geometric sans; `EDU` is a tracked mono label inside a
  restrained capsule. Never recreate the lockup with page text.

---

## 4. The signature: orbit, signal and connected rhythm

- **Orbit:** curved paths can connect related modules or show a flow continuing between roles.
- **Signal:** a small lime point marks the current decision or active hand-off. One signal is
  stronger than a field of decorative dots.
- **Connected rhythm:** entrances may stagger related items in sequence, showing information
  arriving in a usable order.
- **Motion:** the logo orbit may settle in and the signal point may scale once. It never spins
  continuously. All motion must have a complete static first frame and honour
  `prefers-reduced-motion`.

Avoid generic graduation-cap marks, shield marks, checkmarks, literal books, rainbow palettes and
unbounded glowing effects.

---

## 5. Multi-tenant rule

AuraEDU and each school are distinct brands:

- Company marketing, authentication and platform-level chrome use the AuraEDU lockup.
- A tenant's public website leads with the school's own crest, name and colours.
- Inside the portal, the AuraEDU product lockup may sit beside the tenant's short name; it must not
  replace the school's identity.
- Tenant colour overrides apply to semantic product actions through the documented token layer.
  They do not recolour the AuraEDU master logo.
- When a tenant logo is unavailable, show a deliberate school initial fallback rather than using
  the AuraEDU mark as the school's crest.

---

## 6. Voice

Clear, calm and institutional; British English; sentence case. Prefer plain verbs that describe
what people control. Keep control names stable through a flow. Errors state what is unavailable and
what the person can do next. Do not invent activity, outcomes, customers or confidence.

---

## 7. Logo system

The master assets are:

- `auraedu-logo-dark.svg` — dark wordmark for light surfaces.
- `auraedu-logo-light.svg` — light wordmark for midnight or dark surfaces.
- `auraedu-mark.svg` — mark-only use in compact product chrome.
- `app/icon.svg` — browser icon source.

Usage rules:

- Preserve the lockup's aspect ratio and clear space of at least half the mark width.
- Do not type `AuraEDU` as a substitute for the logo in a header, authentication panel or footer.
- Use the full lockup at 32–48 px high. Use the mark alone below that range.
- Do not recolour, rotate, redraw or separate the orbit, A and signal point.
- SVG animation is optional; the complete mark must remain visible when animation is disabled.
- Accessible images use the alt text `AuraEDU`; decorative repetitions use an empty alt.

`tools/generate-brand-assets.mjs` deterministically produces the multi-resolution `.ico`, Apple
touch icons, mobile app icon, Android adaptive foreground and mobile lockup from the master SVGs.
Run it after any approved master-mark change and commit the generated assets.

---

## 8. Surfaces

- **auraedu.com:** fixed AuraEDU identity with midnight fields, cobalt systems structure, teal
  continuity and sparse lime signals.
- **Product:** cool mineral neutrals with tenant-aware semantic actions. AuraEDU retains its master
  colours in the product lockup.
- **Mobile:** the same mark, adaptive icon and lockup generated from the web masters; no mobile-only
  redraw.
