import React from "react";
import {
  colors,
  ThemeProvider as NativeThemeProvider,
  useTheme,
  type Theme,
} from "@auraedu/ui-native";
import { useAuth } from "./auth";

export { colors, useTheme, type Theme };

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const { branding } = useAuth();
  return React.createElement(NativeThemeProvider, { brand: branding?.primary, children });
}
