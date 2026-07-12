"use client";

import * as React from "react";
import { cn } from "../lib/cn";
import { useTheme } from "../lib/use-theme";

type ViewTransitionDoc = Document & {
  startViewTransition?: (cb: () => void) => { finished: Promise<void> };
};

/** Light/dark toggle with the circular View-Transition reveal (DESIGN_SYSTEM §6). */
export function ThemeToggle({ className }: { className?: string }) {
  const { theme, setTheme } = useTheme();
  const dark = theme === "dark";

  function onClick(event: React.MouseEvent<HTMLButtonElement>) {
    const next = dark ? "light" : "dark";
    const root = document.documentElement;
    root.style.setProperty("--theme-reveal-x", `${event.clientX}px`);
    root.style.setProperty("--theme-reveal-y", `${event.clientY}px`);

    const doc = document as ViewTransitionDoc;
    const reduce =
      typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;

    if (typeof doc.startViewTransition === "function" && !reduce) {
      root.classList.add("theme-reveal");
      void doc
        .startViewTransition(() => setTheme(next))
        .finished.finally(() => root.classList.remove("theme-reveal"));
    } else {
      setTheme(next);
    }
  }

  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={dark}
      aria-label={dark ? "Switch to light theme" : "Switch to dark theme"}
      className={cn(
        "grid size-10 place-items-center rounded-full border border-[var(--border)] bg-[var(--surface)] text-[var(--foreground)]",
        "transition-all hover:bg-[var(--muted)] hover:rotate-12",
        className,
      )}
    >
      {dark ? (
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z" />
        </svg>
      ) : (
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
          strokeLinecap="round"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="4.5" />
          <path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4" />
        </svg>
      )}
    </button>
  );
}
