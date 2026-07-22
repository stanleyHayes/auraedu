"use client";

import {
  animate,
  motion,
  useInView,
  useReducedMotion,
  useScroll,
  useTransform,
} from "framer-motion";
import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";

const ease = [0.25, 1, 0.5, 1] as const;

function useHydrated() {
  const [hydrated, setHydrated] = useState(false);
  useEffect(() => setHydrated(true), []);
  return hydrated;
}

export function ScrollReveal({
  children,
  className,
  delay = 0,
  direction = "up",
}: {
  children: ReactNode;
  className?: string;
  delay?: number;
  direction?: "up" | "left" | "right";
}) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "240px 0px 240px 0px" });
  const reduced = useReducedMotion();
  const hydrated = useHydrated();
  const offset = direction === "left" ? { x: -24 } : direction === "right" ? { x: 24 } : { y: 24 };
  const visible = { opacity: 1, x: 0, y: 0 };

  return (
    <motion.div
      ref={ref}
      className={className}
      // `initial={false}` keeps SSR/no-JS content readable. After hydration,
      // off-screen content settles into its reveal origin and animates on entry.
      initial={false}
      animate={!hydrated || reduced || inView ? visible : { opacity: 0, ...offset }}
      transition={{ duration: reduced ? 0 : 0.55, delay: reduced ? 0 : delay, ease }}
    >
      {children}
    </motion.div>
  );
}

export function StaggerChildren({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "240px 0px 240px 0px" });
  const reduced = useReducedMotion();
  const hydrated = useHydrated();

  return (
    <motion.div
      ref={ref}
      className={className}
      initial={false}
      animate={!hydrated || reduced || inView ? "visible" : "hidden"}
      variants={{ visible: { transition: { staggerChildren: reduced ? 0 : 0.09 } } }}
    >
      {children}
    </motion.div>
  );
}

export function StaggerItem({ children, className }: { children: ReactNode; className?: string }) {
  const reduced = useReducedMotion();
  return (
    <motion.div
      className={className}
      variants={{
        hidden: { opacity: 0, y: 18, rotateX: 10 },
        visible: {
          opacity: 1,
          y: 0,
          rotateX: 0,
          transition: { duration: reduced ? 0 : 0.5, ease },
        },
      }}
      style={{ transformPerspective: 900 }}
    >
      {children}
    </motion.div>
  );
}

export function Reveal3D({ children, className }: { children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const reduced = useReducedMotion();
  const hydrated = useHydrated();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const rotateX = useTransform(scrollYProgress, [0, 0.45, 1], [7, 0, -3]);
  const y = useTransform(scrollYProgress, [0, 1], [20, -12]);

  return (
    <motion.div
      ref={ref}
      className={className}
      style={hydrated && !reduced ? { rotateX, y, transformPerspective: 1200 } : undefined}
    >
      {children}
    </motion.div>
  );
}

export function AnimatedCounter({ value, suffix = "" }: { value: number; suffix?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true });
  const reduced = useReducedMotion();
  const hydrated = useHydrated();
  const [display, setDisplay] = useState(value);

  useEffect(() => {
    if (!hydrated || reduced) {
      setDisplay(value);
      return;
    }
    if (!inView) {
      setDisplay(0);
      return;
    }
    setDisplay(0);
    const controls = animate(0, value, {
      duration: 0.9,
      ease,
      onUpdate: (latest) => setDisplay(Math.round(latest)),
    });
    return () => controls.stop();
  }, [hydrated, inView, reduced, value]);

  return (
    <span ref={ref} className="tabular-nums">
      {display}
      {suffix}
    </span>
  );
}

export function PageTransition({ children }: { children: ReactNode }) {
  const reduced = useReducedMotion();
  return (
    <motion.div
      initial={false}
      animate={{ opacity: 1 }}
      transition={{ duration: reduced ? 0 : 0.32, ease }}
    >
      {children}
    </motion.div>
  );
}
