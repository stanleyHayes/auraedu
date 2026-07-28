import { ChevronDown } from "lucide-react";

export interface FaqEntry {
  question: string;
  answer: string;
}

export interface FaqGroup {
  title: string;
  eyebrow: string;
  items: FaqEntry[];
}

export function FaqAccordion({ groups }: { groups: FaqGroup[] }) {
  let offset = 0;
  return (
    <div className="grid items-start gap-5 lg:grid-cols-2">
      {groups.map((group) => {
        const start = offset;
        offset += group.items.length;
        return (
          <section
            key={group.title}
            aria-label={group.title}
            className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm"
          >
            <header className="flex items-center justify-between gap-4 bg-navy-deep px-6 py-4 text-white">
              <div>
                <p className="font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-teal-bright">
                  {group.eyebrow}
                </p>
                <h2 className="mt-1 text-lg font-bold">{group.title}</h2>
              </div>
              <span className="text-xs text-slate-400">{group.items.length} questions</span>
            </header>
            <div className="px-6">
              {group.items.map((item, index) => (
                <details
                  key={item.question}
                  className="group border-t border-slate-200 first:border-t-0"
                >
                  <summary className="flex min-h-[68px] cursor-pointer list-none items-center gap-4 py-4 marker:content-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt [&::-webkit-details-marker]:hidden">
                    <span
                      aria-hidden="true"
                      className="grid size-9 shrink-0 place-items-center rounded-lg border border-teal-strong/25 bg-teal-strong/5 font-mono text-[11px] font-bold text-teal-strong transition-colors group-hover:bg-teal-strong group-hover:text-white"
                    >
                      {String(start + index + 1).padStart(2, "0")}
                    </span>
                    <span className="text-[15px] font-bold leading-snug text-navy-deep">
                      {item.question}
                    </span>
                    <ChevronDown
                      aria-hidden="true"
                      className="ml-auto size-4 shrink-0 text-slate-400 transition-transform group-open:rotate-180"
                    />
                  </summary>
                  <p className="max-w-[62ch] pb-5 pl-[52px] pr-2 text-sm leading-7 text-slate-600">
                    {item.answer}
                  </p>
                </details>
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}
