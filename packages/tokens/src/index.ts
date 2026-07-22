/**
 * AuraEDU design tokens — hex mirror of tokens.css for React Native / NativeWind.
 * Keep the two files in sync.
 */
export const tokens = {
  /** Default brand = systems cobalt. Per tenant, replace `brand.DEFAULT` at runtime. */
  brand: {
    DEFAULT: "#1557FF",
    deep: "#0B3FC7",
    tint: "#E8EFFF",
    contrast: "#FFFFFF",
    secondary: "#061631",
  },
  navy: { DEFAULT: "#061631", soft: "#0C2857" },
  gold: { DEFAULT: "#F7B62C", soft: "#FFD36B" },
  signal: "#B7F500",
  tealBright: "#63D5DA",
  coral: "#F46C2F",
  cream: "#F7F9FC",
  parchment: "#F2F6FC",
  wine: "#1557FF",
  ink: {
    950: "#061631",
    900: "#0B1D3A",
    800: "#162B4C",
    600: "#405473",
    400: "#71819A",
    200: "#B6C1D1",
  },
  paper: {
    50: "#F7F9FC",
    100: "#FFFFFF",
    200: "#F2F6FC",
    300: "#DBE3ED",
    400: "#C6D1DF",
  },
  status: { ok: "#07876F", warn: "#C47800", crit: "#B42318", gold: "#F7B62C" },
  radius: { sm: 5, md: 8, lg: 12, xl: 16, "2xl": 20, full: 9999 },
  ease: "cubic-bezier(0.25,1,0.5,1)",
} as const;

/** Semantic theme tokens (mobile). */
export const theme = {
  light: {
    background: "#F7F9FC",
    surface: "#FFFFFF",
    foreground: "#061631",
    muted: "#F2F6FC",
    mutedForeground: "#71819A",
    border: "#DBE3ED",
    primary: "#1557FF",
    primaryForeground: "#FFFFFF",
    secondary: "#061631",
    secondaryForeground: "#FFFFFF",
    accent: "#E8EFFF",
    ring: "#1557FF",
  },
  dark: {
    background: "#061631",
    surface: "#0B1D3A",
    foreground: "#F7F9FC",
    muted: "#0F2444",
    mutedForeground: "#B6C1D1",
    border: "#263B5C",
    primary: "#5E8CFF",
    primaryForeground: "#061631",
    secondary: "#B6C1D1",
    secondaryForeground: "#061631",
    accent: "#163D86",
    ring: "#6D97FF",
  },
} as const;

export type Tokens = typeof tokens;
export type ThemeName = keyof typeof theme;
export type ThemeTokens = (typeof theme)[ThemeName];
