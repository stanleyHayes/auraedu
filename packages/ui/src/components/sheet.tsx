"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export interface SheetProps {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
  side?: "left" | "right";
  className?: string;
}

/** Lightweight modal drawer using the native <dialog> element (DESIGN_SYSTEM §7.6). */
export function Sheet({ open, onClose, children, side = "left", className }: SheetProps) {
  const ref = React.useRef<HTMLDialogElement>(null);

  React.useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  React.useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    function handleClose() {
      onClose();
    }
    dialog.addEventListener("close", handleClose);
    return () => dialog.removeEventListener("close", handleClose);
  }, [onClose]);

  return (
    <dialog
      ref={ref}
      className={cn(
        "fixed inset-y-0 m-0 h-full max-h-none w-72 max-w-full bg-[var(--surface)] p-0 shadow-2xl backdrop:bg-[var(--color-ink-950)]/45 backdrop:animate-[fade-in_160ms_var(--ease-out-quart)]",
        side === "left" ? "left-0" : "right-0",
        "[&:modal]:max-w-full [&:modal]:max-h-none",
        className,
      )}
    >
      <div className="h-full overflow-y-auto">{children}</div>
    </dialog>
  );
}
