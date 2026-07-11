"use client";

import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";

const STORAGE_KEY = "auraedu-theme";

/**
 * Light/dark theme state, persisted to localStorage and seeded from the system
 * preference. Not next-themes — the app also injects a no-FOUC inline script
 * (DESIGN_SYSTEM §3.3) so first paint is correct before hydration.
 */
export function useTheme(): { theme: Theme; setTheme: (t: Theme) => void } {
  const [theme, setThemeState] = useState<Theme>("light");

  useEffect(() => {
    const saved: Theme | null = (() => {
      try {
        const raw = localStorage.getItem(STORAGE_KEY);
        return raw === "light" || raw === "dark" ? raw : null;
      } catch {
        return null;
      }
    })();
    const prefersDark =
      typeof matchMedia !== "undefined" && matchMedia("(prefers-color-scheme: dark)").matches;
    setThemeState(saved ?? (prefersDark ? "dark" : "light"));
  }, []);

  const setTheme = useCallback((next: Theme) => {
    const root = document.documentElement;
    root.classList.toggle("dark", next === "dark");
    root.style.colorScheme = next;
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* storage unavailable — theme still applies for this session */
    }
    setThemeState(next);
  }, []);

  return { theme, setTheme };
}
