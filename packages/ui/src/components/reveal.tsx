"use client";

import * as React from "react";
import { cn } from "../lib/cn";

export type RevealVariant = "up" | "left" | "right" | "scale";

export interface RevealProps {
  children: React.ReactNode;
  as?: React.ElementType;
  variant?: RevealVariant;
  delay?: number;
  threshold?: number;
  rootMargin?: string;
  className?: string;
  once?: boolean;
}

const variantClass: Record<RevealVariant, string> = {
  up: "reveal",
  left: "reveal-left",
  right: "reveal-right",
  scale: "reveal-scale",
};

/** Scroll-triggered reveal wrapper using IntersectionObserver. */
export function Reveal({
  children,
  as: Component = "div",
  variant = "up",
  delay = 0,
  threshold = 0.1,
  rootMargin = "0px 0px -40px 0px",
  className,
  once = true,
}: RevealProps) {
  const ref = React.useRef<HTMLElement>(null);
  const [visible, setVisible] = React.useState(false);

  React.useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          setVisible(true);
          if (once) observer.disconnect();
        } else if (!once) {
          setVisible(false);
        }
      },
      { threshold, rootMargin },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [once, threshold, rootMargin]);

  return (
    <Component
      ref={ref}
      className={cn(variantClass[variant], visible && "reveal-visible", className)}
      style={{ transitionDelay: `${delay}ms` }}
    >
      {children}
    </Component>
  );
}
