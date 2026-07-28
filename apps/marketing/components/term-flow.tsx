import { AlertTriangle, Check, Clock3, ReceiptText } from "lucide-react";

type StageTone = "done" | "now" | "next";

interface TermStage {
  title: string;
  detail: string;
  tone: StageTone;
}

const stages: TermStage[] = [
  {
    title: "Invoiced",
    detail: "Term 2 bills sent to 412 guardians — GH₵ 850 per learner.",
    tone: "done",
  },
  {
    title: "MoMo paid",
    detail: "387 mobile-money payments confirmed and reconciled to each learner.",
    tone: "now",
  },
  {
    title: "Receipt to parent",
    detail: "Receipts queued for SMS, WhatsApp and the parent portal.",
    tone: "next",
  },
];

const dotStyles: Record<StageTone, string> = {
  done: "border-emerald-600 bg-emerald-600 text-white",
  now: "border-amber-500 bg-amber-500 text-white shadow-[0_0_0_5px_rgba(245,158,11,0.14)]",
  next: "border-slate-300 bg-white text-slate-400",
};

const connectorStyles: Record<StageTone, string> = {
  done: "bg-emerald-500/60",
  now: "bg-slate-200",
  next: "bg-slate-200",
};

export function TermFlow() {
  return (
    <article
      aria-label="Example fee lifecycle for one school term"
      className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_32px_80px_-42px_rgba(4,18,43,0.45)]"
    >
      <header className="relative overflow-hidden bg-navy-deep px-6 py-5 text-white sm:px-8">
        <div
          aria-hidden="true"
          className="absolute -right-16 -top-24 size-48 rounded-full border-[26px] border-white/5"
        />
        <div className="relative flex items-start justify-between gap-4">
          <div>
            <p className="font-mono text-[10px] font-bold uppercase tracking-[0.18em] text-teal-bright">
              Term 2 fees · INV-2041
            </p>
            <h3 className="mt-1.5 text-xl font-bold leading-tight sm:text-2xl">A term in motion</h3>
          </div>
          <span className="shrink-0 rounded-full border border-white/15 bg-white/10 px-3 py-1 text-[11px] font-semibold">
            In progress
          </span>
        </div>
      </header>

      <div className="px-6 py-6 sm:px-8">
        <div className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-4 rounded-xl border border-amber-200 bg-amber-50 p-4">
          <span
            aria-hidden="true"
            className="grid size-11 place-items-center rounded-lg bg-amber-500 text-white shadow-[0_12px_28px_-14px_rgba(245,158,11,0.9)]"
          >
            <ReceiptText className="size-5" />
          </span>
          <div className="min-w-0">
            <p className="text-sm font-bold leading-snug text-navy-deep">
              Payments are landing and reconciling themselves
            </p>
            <p className="mt-1 flex items-center gap-1.5 text-xs text-slate-600">
              <Clock3 className="size-3.5" aria-hidden="true" />
              Receipts usually reach parents within the hour
            </p>
          </div>
        </div>

        <ol className="mt-7">
          {stages.map((stage, index) => (
            <li key={stage.title} className="relative grid grid-cols-[34px_minmax(0,1fr)] gap-3.5">
              {index < stages.length - 1 ? (
                <span
                  aria-hidden="true"
                  className={`absolute left-[16px] top-[38px] bottom-[-8px] w-0.5 ${connectorStyles[stage.tone]}`}
                />
              ) : null}
              <span
                aria-hidden="true"
                className={`relative z-10 grid size-[34px] place-items-center rounded-full border text-xs font-extrabold ${dotStyles[stage.tone]}`}
              >
                {stage.tone === "done" ? <Check className="size-4" /> : index + 1}
              </span>
              <div className="flex min-h-[52px] items-start justify-between gap-3 pb-4">
                <div className="min-w-0">
                  <p
                    className={`text-sm leading-snug ${
                      stage.tone === "next"
                        ? "font-medium text-slate-500"
                        : "font-bold text-navy-deep"
                    }`}
                  >
                    {stage.title}
                  </p>
                  <p className="mt-1 text-xs leading-5 text-slate-500">{stage.detail}</p>
                </div>
                {stage.tone === "now" ? (
                  <span className="mt-0.5 shrink-0 rounded-full bg-amber-100 px-2.5 py-1 text-[10px] font-extrabold uppercase tracking-[0.1em] text-amber-800">
                    Now
                  </span>
                ) : null}
              </div>
            </li>
          ))}
        </ol>

        <p className="mt-2 flex items-start gap-2 rounded-lg bg-red-50 px-3.5 py-2.5 text-xs leading-5 text-red-800">
          <AlertTriangle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />2 payments failed
          — an automatic retry is scheduled and the school can see exactly who to call.
        </p>
      </div>
    </article>
  );
}
