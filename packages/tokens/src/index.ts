/**
 * AuraEDU design tokens — hex mirror of tokens.css for React Native / NativeWind.
 * Keep the two files in sync.
 */
export const tokens = {
  /** Default brand = deep maroon. Per tenant, replace `brand.DEFAULT` at runtime. */
  brand: {
    DEFAULT: "#7B1113",
    deep: "#4A0A0C",
    tint: "#F3DED9",
    contrast: "#FFFFFF",
    secondary: "#001B50",
  },
  navy: { DEFAULT: "#001B50", soft: "#002870" },
  gold: { DEFAULT: "#C58B2C", soft: "#D4AF37" },
  cream: "#FDFCF7",
  parchment: "#F7F5EE",
  wine: "#800020",
  ink: {
    950: "#15111A",
    900: "#1F1C18",
    800: "#2A2722",
    600: "#56514A",
    400: "#8A837A",
    200: "#BFB8AD",
  },
  paper: {
    50: "#FDFCF7",
    100: "#FFFFFF",
    200: "#F5F3EC",
    300: "#E8E4DA",
    400: "#D6D1C4",
  },
  status: { ok: "#1E7D52", warn: "#B5740B", crit: "#B42318", gold: "#C58B2C" },
  radius: { sm: 5, md: 8, lg: 12, xl: 16, "2xl": 20, full: 9999 },
  ease: "cubic-bezier(0.25,1,0.5,1)",
} as const;

/** Semantic theme tokens (mobile). */
export const theme = {
  light: {
    background: "#FDFCF7",
    surface: "#FFFFFF",
    foreground: "#15111A",
    muted: "#F5F3EC",
    mutedForeground: "#8A837A",
    border: "#E8E4DA",
    primary: "#7B1113",
    primaryForeground: "#FFFFFF",
    secondary: "#001B50",
    secondaryForeground: "#FFFFFF",
    accent: "#F3DED9",
    ring: "#7B1113",
  },
  dark: {
    background: "#15111A",
    surface: "#1F1C18",
    foreground: "#FDFCF7",
    muted: "#1A1714",
    mutedForeground: "#BFB8AD",
    border: "#2A2722",
    primary: "#D45A52",
    primaryForeground: "#15111A",
    secondary: "#4A6FA5",
    secondaryForeground: "#FDFCF7",
    accent: "#3A201E",
    ring: "#D45A52",
  },
} as const;

export type Tokens = typeof tokens;
export type ThemeName = keyof typeof theme;
export type ThemeTokens = (typeof theme)[ThemeName];
