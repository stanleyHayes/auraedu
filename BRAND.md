# AuraEDU — Brand & Visual Identity

**Direction:** *Chalkboard & Register.* The single source of truth for AuraEDU's own voice,
colour, typography, and signature. Companion to [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md)
(interaction/components) — this doc is the *character*; that doc is the *mechanics*.
Live reference: the "Chalkboard & Register" landing artifact.

> **Why this and not generic ed-tech.** The default school-software look — friendly rounded
> sans, primary blue, blobby illustrations — reads as a toy. AuraEDU is the **system of record**
> for a school. Its identity is drawn from the real materials of a Ghanaian classroom: the
> **chalkboard**, the **attendance register**, the **red marking pen**, the **timetable grid**.
> Institutional, warm, unmistakable — and it survives per-tenant re-skinning because the identity
> lives in type, structure, and motion, not only colour.

---

## 1. The idea

AuraEDU accounts for every student, every day, across many schools. The **register** is the
daily ritual; the **red tick** is the proof. That ritual is the brand: ruled lines to sit on,
a red mark for *present / done / approved*, and a monospace "ledger" voice for data.

**One line:** *Every student accounted for. Every school, one platform.*

---

## 2. Colour

Neutrals are **green-biased** (the chalkboard), never plain grey — this is the deliberate
signature that unifies every tenant. The **red marking pen** is the AuraEDU default action
colour; per tenant it is replaced by the school's own brand colour (see §5), while the
chalkboard neutrals stay constant.

### Light — chalk paper
| Token | Hex | Use |
|---|---|---|
| `--paper` | `#F4F5F1` | app background (cool chalk-white — **not** warm cream) |
| `--surface` | `#FBFCF9` | cards, register |
| `--ink` | `#16241D` | primary text (chalkboard green-black) |
| `--ink-2` | `#48594F` | secondary text |
| `--ink-3` | `#7C8B82` | muted / captions |
| `--rule` | `#DCE0D8` | ruled lines, borders |
| `--mark` | `#C6402F` | red marking pen — primary action/accent |
| `--mark-deep` | `#9A2E20` | hover/pressed on red |
| `--gold` | `#A9781F` | kente ochre — secondary/status, used sparingly |

### Dark — the chalkboard
| Token | Hex |
|---|---|
| `--paper` | `#16241D` (the board) · `--surface` `#1D2F27` · `--sunk` `#132019` |
| `--ink` | `#EAEDE6` (chalk) · `--ink-2` `#A6B2AA` · `--ink-3` `#7B897F` |
| `--rule` | `#2B3D34` · `--mark` `#E36A54` (brighter for contrast) · `--gold` `#D3A44E` |

**Semantic** (distinct from the accent): `--ok #2F7D53`, `--warn #A9781F`, `--crit #C6402F`.
Contrast: `--mark` on `--paper` and `--ink` on `--paper` both pass WCAG AA.

---

## 3. Typography

Character comes from **treatment** (ruled baselines, tracked mono labels, the red tick,
dramatic scale), so the faces are chosen to be reliable and institutional, not novel.

- **Display / h1–h2 — Fraunces.** A warm, editorial serif for the two largest heading levels.
  Load **Fraunces** (600/700/800) via `next/font` and bind it to `--font-heading`. In
  constrained contexts (email, artifacts with no webfont): fall back to a classic serif stack
  (`Georgia`, `Times New Roman`, serif).
- **Body / UI — Outfit.** Open, legible, and institutional. Used for everything that is not an
  h1 or h2: running text, navigation, labels, buttons, logos, KPI values, and empty-state titles.
  Load **Outfit** (400/500/600/700) via `next/font` and bind it to `--font-sans`.
- **Data / ledger — monospace** (`Spline Sans Mono`, or system `ui-monospace`). Registers,
  marks, IDs, tenant codes, timetables, KPI values. `font-variant-numeric: tabular-nums` wherever
  digits align. Uppercase mono with `letter-spacing:.14em` for eyebrows and labels.

Type scale is deliberate; headings get `text-wrap: balance`; running text ~65ch.

---

## 4. The signature: the tick & the rule

One memorable element, used with discipline:

- **The red tick** (`✓`, a stroked mark) means *present · done · approved · active*. It is the
  active nav marker, the enabled-feature state, the register mark, the form-submit success. Draw
  it as a stroked SVG path (animates via `stroke-dashoffset`); never a generic checkbox.
- **The ruled line** is the baseline everything sits on — the register's horizontal rules, the
  sidebar's connector "threads", section dividers. Structure *is* the register.
- **Signature moment (once per surface):** the **living register** — names/rows tick in on load,
  a present-count settles. Used on the marketing hero; echoed by skeletons "filling the register"
  in-app. Under `prefers-reduced-motion`, everything is pre-ticked and static.

Do **not** add decoration beyond this. Spend boldness on the tick; keep everything else quiet.

---

## 5. Multi-tenant rule (how the identity re-skins)

AuraEDU serves many schools from one codebase. Per tenant:
- **Chalkboard neutrals stay constant** — this is the recognisable "AuraEDU green" across all schools.
- **The school's brand colour replaces `--mark`** (the action/accent) — injected at runtime as a
  CSS variable (see `DESIGN_SYSTEM.md` §3). Ticks, active nav, primary buttons, crest take the
  school's colour; the ruled chalkboard system stays.
- **Type, structure, and motion never change** — they are the persistent AuraEDU identity.

So UPSHS wears maroon, Aboom wears green, the next school wears its own — all unmistakably AuraEDU.

---

## 6. Voice

Clear, calm, institutional; British English; sentence case. Plain verbs for what people control
("Take the register", "Publish results", "Sign up your school"), never system nouns. A control
says what happens and keeps its name through the flow (button "Publish" → toast "Published").
Errors explain what went wrong and how to fix it — no apologies, no vagueness. Empty states invite
the next action. Copy is design material: specific beats clever.

---

## 7. Surfaces

- **auraedu.com (company site):** fixed AuraEDU identity above — chalkboard + red pen. Framer
  Motion allowed (`DESIGN_SYSTEM.md` §18). The hero is the living register (the thesis).
- **Product (portals, per tenant):** chalkboard neutrals + the tenant's accent; the tick & rule
  signatures throughout; motion policy per `DESIGN_SYSTEM.md`.
- **Mobile:** the same identity via `packages/ui-native` + the hex token mirror in `packages/tokens`.

---

## 8. Logo

A wordmark **AuraEDU** (display face, 800) beside a mark: a small dark tile carrying a **red
register tick**. Provide full lockup, mark-only (app icon / favicon = the tick), and a monochrome
variant. The tick is the app icon.
