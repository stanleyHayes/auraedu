"use client";

import * as React from "react";
import { X } from "lucide-react";
import { cn } from "../lib/cn";

export interface SheetProps {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
  side?: "left" | "right";
  className?: string;
  closeButtonClassName?: string;
}

/** Modal drawer using the native <dialog> element with slide-in motion. */
export function Sheet({
  open,
  onClose,
  children,
  side = "left",
  className,
  closeButtonClassName,
}: SheetProps) {
  const ref = React.useRef<HTMLDialogElement>(null);
  const [mounted, setMounted] = React.useState(false);

  React.useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      setMounted(true);
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [open]);

  React.useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    function handleClose() {
      setMounted(false);
      onClose();
    }
    dialog.addEventListener("close", handleClose);
    return () => dialog.removeEventListener("close", handleClose);
  }, [onClose]);

  return (
    <dialog
      ref={ref}
      className={cn(
        "fixed inset-y-0 m-0 h-full max-h-none max-w-full bg-[var(--surface)] p-0 shadow-2xl backdrop:bg-[var(--color-ink-950)]/45",
        side === "left" ? "left-0" : "right-0",
        "[&:modal]:max-w-full [&:modal]:max-h-none",
        "motion-safe:transition-[transform,opacity] motion-safe:duration-300 motion-safe:[--ease-spring:cubic-bezier(0.22,1,0.36,1)]",
        mounted
          ? "translate-x-0 opacity-100"
          : side === "left"
            ? "-translate-x-8 opacity-0"
            : "translate-x-8 opacity-0",
        className,
      )}
    >
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-end px-3 py-2">
          <button
            type="button"
            onClick={onClose}
            aria-label="Close panel"
            className={cn(
              "grid size-9 place-items-center rounded-full text-[var(--foreground)] transition-colors hover:bg-[var(--muted)]",
              closeButtonClassName,
            )}
          >
            <X className="size-5" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto">{children}</div>
      </div>
    </dialog>
  );
}
