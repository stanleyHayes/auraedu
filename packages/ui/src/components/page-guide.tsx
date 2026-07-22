"use client";

import * as React from "react";
import { HelpCircle, Square, Volume2, X } from "lucide-react";
import { cn } from "../lib/cn";

export interface PageGuide {
  key: string;
  section: string;
  href: string;
  title: string;
  description: string;
  steps: string[];
}

type PageGuideResolver = (title: string, description?: string) => PageGuide | undefined;

const PageGuideContext = React.createContext<PageGuideResolver | null>(null);

export function PageGuideProvider({
  resolve,
  children,
}: {
  resolve: PageGuideResolver;
  children: React.ReactNode;
}) {
  return <PageGuideContext.Provider value={resolve}>{children}</PageGuideContext.Provider>;
}

export function useResolvedPageGuide(
  title: string,
  description?: string,
  explicit?: PageGuide,
): PageGuide | undefined {
  const resolve = React.useContext(PageGuideContext);
  return explicit ?? resolve?.(title, description);
}

export function PageHelp({ guide, className }: { guide: PageGuide; className?: string }) {
  const [open, setOpen] = React.useState(false);
  const [speaking, setSpeaking] = React.useState(false);
  const rootRef = React.useRef<HTMLDivElement>(null);
  const transcriptId = React.useId();
  const titleId = React.useId();

  React.useEffect(() => {
    if (!open) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    function onPointerDown(event: PointerEvent) {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, [open]);

  React.useEffect(
    () => () => {
      if ("speechSynthesis" in window) window.speechSynthesis.cancel();
    },
    [],
  );

  function toggleSpeech() {
    if (!("speechSynthesis" in window)) return;
    if (speaking) {
      window.speechSynthesis.cancel();
      setSpeaking(false);
      return;
    }
    const utterance = new SpeechSynthesisUtterance(
      [guide.title, guide.description, ...guide.steps].join(". "),
    );
    utterance.lang = "en-GB";
    utterance.onend = utterance.onerror = () => setSpeaking(false);
    window.speechSynthesis.cancel();
    setSpeaking(true);
    window.speechSynthesis.speak(utterance);
  }

  return (
    <div ref={rootRef} className={cn("relative", className)}>
      <button
        type="button"
        onClick={() => setOpen((current) => !current)}
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-describedby={transcriptId}
        aria-label={`How to use ${guide.title}`}
        className="grid size-9 place-items-center rounded-full border border-[var(--border)] bg-[var(--surface)] text-[var(--muted-foreground)] transition hover:border-[var(--primary)]/35 hover:bg-[var(--accent)] hover:text-[var(--primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
      >
        <HelpCircle className="size-[18px]" aria-hidden="true" />
      </button>

      <div id={transcriptId} data-page-guide hidden>
        <h2>{guide.title}</h2>
        <p>{guide.description}</p>
        <ol>
          {guide.steps.map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
      </div>

      {open ? (
        <section
          role="dialog"
          aria-modal="false"
          aria-labelledby={titleId}
          className="absolute right-0 top-full z-[230] mt-3 w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] shadow-2xl motion-safe:animate-[slide-up_180ms_var(--ease-out-quart)]"
        >
          <div className="border-b border-[var(--border)] bg-[linear-gradient(135deg,color-mix(in_oklab,var(--primary)_11%,var(--surface)),var(--surface))] p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="font-mono text-[10px] font-black uppercase tracking-[0.18em] text-[var(--primary)]">
                  Page guide
                </p>
                <h2
                  id={titleId}
                  className="mt-1 font-heading text-lg font-extrabold text-[var(--foreground)]"
                >
                  {guide.title}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close page guide"
                className="grid size-8 place-items-center rounded-full text-[var(--muted-foreground)] hover:bg-[var(--muted)] hover:text-[var(--foreground)]"
              >
                <X className="size-4" aria-hidden="true" />
              </button>
            </div>
            <p className="mt-2 text-sm leading-6 text-[var(--muted-foreground)]">
              {guide.description}
            </p>
          </div>
          <ol className="space-y-3 p-5">
            {guide.steps.map((step, index) => (
              <li
                key={step}
                className="grid grid-cols-[1.75rem_1fr] gap-3 text-sm leading-6 text-[var(--foreground)]"
              >
                <span className="grid size-7 place-items-center rounded-full bg-[var(--accent)] font-mono text-[11px] font-black text-[var(--primary)]">
                  {index + 1}
                </span>
                <span>{step}</span>
              </li>
            ))}
          </ol>
          <div className="flex items-center justify-between border-t border-[var(--border)] bg-[var(--muted)]/35 px-5 py-4">
            <span className="text-xs text-[var(--muted-foreground)]">
              British English narration
            </span>
            <button
              type="button"
              onClick={toggleSpeech}
              aria-pressed={speaking}
              className="inline-flex h-9 items-center gap-2 rounded-full bg-[var(--primary)] px-4 text-xs font-bold text-[var(--primary-foreground)] transition hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            >
              {speaking ? (
                <Square className="size-3.5" aria-hidden="true" />
              ) : (
                <Volume2 className="size-4" aria-hidden="true" />
              )}
              {speaking ? "Stop" : "Listen"}
            </button>
          </div>
        </section>
      ) : null}
    </div>
  );
}
