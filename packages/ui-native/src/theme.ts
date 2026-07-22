import React, { createContext, useContext, useMemo } from "react";
import { theme as designTheme, tokens } from "@auraedu/tokens";

export const colors = {
  ink: designTheme.light.foreground,
  muted: designTheme.light.mutedForeground,
  paper: designTheme.light.background,
  surface: designTheme.light.surface,
  border: designTheme.light.border,
  brand: tokens.brand.DEFAULT,
  brandSoft: tokens.brand.tint,
  midnight: "#061631",
  midnightSoft: "#0C2857",
  ink200: "#B6C1D1",
  cobalt: "#1557FF",
  teal: "#087F8C",
  tealBright: "#63D5DA",
  signal: tokens.signal,
  coral: tokens.coral,
  sky: "#E8EFFF",
  mist: "#EEF4FA",
  success: "#07876F",
  warning: "#C47800",
  danger: tokens.status.crit,
} as const;

export type Theme = Omit<typeof colors, "brand"> & { brand: string; onBrand: string };
const ThemeContext = createContext<Theme>({ ...colors, onBrand: "#FFFFFF" });

function readableText(hex: string) {
  const value = hex.slice(1);
  const [r = 0, g = 0, b = 0] = [value.slice(0, 2), value.slice(2, 4), value.slice(4, 6)].map(
    (part) => Number.parseInt(part, 16),
  );
  return (r * 299 + g * 587 + b * 114) / 1000 > 150 ? colors.ink : "#FFFFFF";
}

export function ThemeProvider({
  brand: brandOverride,
  children,
}: {
  brand?: string;
  children: React.ReactNode;
}) {
  const theme = useMemo(() => {
    const brand = brandOverride ?? colors.brand;
    return { ...colors, brand, onBrand: readableText(brand) };
  }, [brandOverride]);
  return React.createElement(ThemeContext.Provider, { value: theme }, children);
}

export function useTheme() {
  return useContext(ThemeContext);
}
