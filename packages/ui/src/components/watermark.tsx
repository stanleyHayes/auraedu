import * as React from "react";
import { cn } from "../lib/cn";

export interface WatermarkProps {
  children: React.ReactNode;
  className?: string;
}

/** Oversized decorative word mark used behind page headers and hero sections. */
export function Watermark({ children, className }: WatermarkProps) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "watermark motion-safe:animate-[float-mark_8s_ease-in-out_infinite]",
        className,
      )}
    >
      {children}
    </span>
  );
}
