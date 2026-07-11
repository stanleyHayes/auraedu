/**
 * AuraEDU design tokens — hex mirror of tokens.css for React Native / NativeWind,
 * which cannot evaluate `oklch()` or `color-mix()` (BRAND.md §3, DESIGN_SYSTEM §3).
 * Web reads tokens.css; mobile reads these values. Keep the two in sync.
 */
export const tokens = {
  /** Default brand = red marking pen. Per tenant, replace `brand.DEFAULT` at runtime. */
  brand: { DEFAULT: "#C6402F", deep: "#9A2E20", tint: "#F3DED9", contrast: "#FFFFFF" },
  ink: {
    950: "#16241D",
    900: "#1D2F27",
    800: "#24382F",
    600: "#48594F",
    400: "#7C8B82",
    200: "#A6B2AA",
  },
  paper: { 50: "#F4F5F1", 100: "#FBFCF9", 200: "#EFF1EB", 300: "#DCE0D8", 400: "#CBD1C6" },
  status: { ok: "#2F7D53", warn: "#A9781F", crit: "#C6402F", gold: "#A9781F" },
  radius: { sm: 5, md: 8, lg: 11, xl: 16 },
  ease: "cubic-bezier(0.25,1,0.5,1)",
} as const;

/** Semantic theme tokens (mobile). Dark brightens the tenant brand for contrast on the board. */
export const theme = {
  light: {
    background: "#F4F5F1",
    surface: "#FBFCF9",
    foreground: "#16241D",
    muted: "#EFF1EB",
    mutedForeground: "#7C8B82",
    border: "#DCE0D8",
    primary: "#C6402F",
    primaryForeground: "#FFFFFF",
    ring: "#C6402F",
  },
  dark: {
    background: "#16241D",
    surface: "#1D2F27",
    foreground: "#EAEDE6",
    muted: "#132019",
    mutedForeground: "#A6B2AA",
    border: "#2B3D34",
    primary: "#E36A54",
    primaryForeground: "#16241D",
    ring: "#E36A54",
  },
} as const;

export type Tokens = typeof tokens;
export type ThemeName = keyof typeof theme;
export type ThemeTokens = (typeof theme)[ThemeName];
