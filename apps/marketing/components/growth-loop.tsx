import { ClipboardList, FileCheck2, GraduationCap, Megaphone } from "lucide-react";

const loop = [
  {
    title: "Enquiry",
    copy: "A parent asks for a place. The school captures the interest once—no notebook, no lost promise to call back.",
    icon: Megaphone,
  },
  {
    title: "Application",
    copy: "Families complete a structured application; the school reviews, decides and communicates from one list.",
    icon: ClipboardList,
  },
  {
    title: "Enrolment",
    copy: "An accepted applicant becomes a learner with records, class placement and guardian links already in place.",
    icon: FileCheck2,
  },
  {
    title: "Parent referrals",
    copy: "Settled families talk. Receipts, report cards and calm communication become the reason the next enquiry arrives.",
    icon: GraduationCap,
  },
];

export function GrowthLoop() {
  return (
    <div className="grid overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_26px_70px_-46px_rgba(12,36,75,0.5)] md:grid-cols-2 lg:grid-cols-4">
      {loop.map((step, index) => {
        const Icon = step.icon;
        return (
          <article
            key={step.title}
            className="relative min-h-[240px] overflow-hidden border-slate-200 p-6 max-lg:[&:not(:last-child)]:border-b lg:[&:not(:last-child)]:border-r"
          >
            <span
              aria-hidden="true"
              className="pointer-events-none absolute -top-3 right-4 font-heading text-[88px] font-bold leading-none text-cobalt/[0.07]"
            >
              {String(index + 1).padStart(2, "0")}
            </span>
            <span className="relative grid size-11 place-items-center rounded-xl bg-accent text-cobalt">
              <Icon className="size-5" aria-hidden="true" />
            </span>
            <h3 className="relative mt-5 text-xl font-bold tracking-[-0.03em] text-navy-deep">
              {step.title}
            </h3>
            <p className="relative mt-2.5 text-sm leading-6 text-slate-600">{step.copy}</p>
          </article>
        );
      })}
    </div>
  );
}
