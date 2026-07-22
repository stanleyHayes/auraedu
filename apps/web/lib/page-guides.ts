import type { PageGuide } from "@auraedu/ui";

type GuideSeed = Omit<PageGuide, "key" | "title" | "description"> & {
  key?: string;
  title?: string;
  description?: string;
};

const GUIDE_SEEDS: GuideSeed[] = [
  {
    href: "/admin",
    section: "School operations",
    steps: [
      "Review the live school indicators and dependency notices.",
      "Use Quick work to move into the record that needs attention.",
      "Open the navigation rail for the complete operational workspace.",
    ],
  },
  {
    href: "/admin/students",
    section: "People",
    steps: [
      "Review or search the tenant-scoped learner register.",
      "Open a learner to inspect the complete school-owned record.",
      "Use the primary action only when you are ready to add or import verified data.",
    ],
  },
  {
    href: "/admin/staff",
    section: "People",
    steps: [
      "Review the current staff register and role context.",
      "Open a staff record before changing assignments.",
      "Use the primary action to add a verified member of staff.",
    ],
  },
  {
    href: "/admin/attendance",
    section: "Teaching and learning",
    steps: [
      "Choose the date and class context first.",
      "Review submitted marks and any missing register evidence.",
      "Resolve exceptions through the assigned teacher workflow.",
    ],
  },
  {
    href: "/admin/assessments",
    section: "Teaching and learning",
    steps: [
      "Review assessment lifecycle and class context.",
      "Check publication status before relying on results.",
      "Open the scoring workflow only for authorised staff.",
    ],
  },
  {
    href: "/admin/fees",
    section: "Finance",
    steps: [
      "Review fee structures and invoice attention states by currency.",
      "Open the relevant learner or invoice before making changes.",
      "Keep provider and reconciliation failures distinct from genuine zero balances.",
    ],
  },
  {
    href: "/admin/payments",
    section: "Finance",
    steps: [
      "Review provider-backed payment outcomes and reconciliation state.",
      "Use references to trace a transaction without exposing provider secrets.",
      "Escalate unresolved or failed payments through the audited workflow.",
    ],
  },
  {
    href: "/admin/reports",
    section: "Reporting",
    steps: [
      "Review draft, generated and published report-card states.",
      "Open the relevant learner record before publication.",
      "Download only published documents through the authorised proxy.",
    ],
  },
  {
    href: "/admin/settings",
    section: "School configuration",
    steps: [
      "Review the current school identity and operational settings.",
      "Change only fields your role is authorised to manage.",
      "Save and verify tenant branding and locale changes before leaving.",
    ],
  },
  {
    href: "/admin/admissions",
    section: "Growth and admissions",
    steps: [
      "Review applications by lifecycle stage and applicant ownership.",
      "Open the complete evidence before recording a human decision.",
      "Use offer actions only after the independent review is documented.",
    ],
  },
  {
    href: "/admin/leads",
    section: "Growth and admissions",
    steps: [
      "Review consented enquiries in the tenant pipeline.",
      "Assign an owner and record each meaningful follow-up.",
      "Move the stage only when the timeline contains supporting evidence.",
    ],
  },
  {
    href: "/admin/journeys",
    section: "Growth and admissions",
    steps: [
      "Choose the exact admissions event that should start the journey and define quiet-hour and frequency limits.",
      "Build each step from an active approved template, using only the allowlisted deterministic branch fields.",
      "Save the draft and ask a different authorised reviewer to activate it; monitor delivery, cancellation and failure counts after launch.",
    ],
  },
  {
    href: "/admin/content",
    section: "Growth and admissions",
    steps: [
      "Confirm the brand policy before generating a governed draft from verified facts.",
      "Resolve every compliance finding and submit the exact version for independent review.",
      "Approve or reject as a different authorised user; publishing remains a separate human workflow.",
    ],
  },
  {
    href: "/teacher",
    section: "Teaching",
    steps: [
      "Review today’s assigned work and live class context.",
      "Use My classes to stay inside your authorised learner scope.",
      "Open attendance, scores or reports from the relevant task card.",
    ],
  },
  {
    href: "/teacher/attendance",
    section: "Teaching",
    steps: [
      "Select one of your assigned classes.",
      "Confirm the resolved roster before marking attendance.",
      "Submit the complete register and review the saved outcome.",
    ],
  },
  {
    href: "/teacher/scores",
    section: "Teaching",
    steps: [
      "Choose an assessment before selecting a learner.",
      "Confirm the authorised class roster and assessment maximum.",
      "Record the score and verify the updated result state.",
    ],
  },
  {
    href: "/teacher/reports",
    section: "Teaching",
    steps: [
      "Review report cards only for your assigned learners.",
      "Distinguish drafts from published documents.",
      "Download or share only the published, authorised PDF.",
    ],
  },
  {
    href: "/parent",
    section: "Family",
    steps: [
      "Choose the child whose school day you want to review.",
      "Use the attention cards for attendance, fees and reports.",
      "Open notifications for current school updates.",
    ],
  },
  {
    href: "/parent/fees",
    section: "Family",
    steps: [
      "Choose the correct child and review invoices by currency.",
      "Open an invoice to confirm amount and status.",
      "Use the secure provider checkout only for an authorised outstanding invoice.",
    ],
  },
  {
    href: "/parent/reports",
    section: "Family",
    steps: [
      "Choose the child whose report you want to review.",
      "Open only published report cards.",
      "Download the PDF through the authenticated school portal.",
    ],
  },
  {
    href: "/student",
    section: "Learning",
    steps: [
      "Review today’s timetable, assignments and learning signals.",
      "Open the task that needs your attention.",
      "Use the navigation rail to move between school modules.",
    ],
  },
  {
    href: "/student/assignments",
    section: "Learning",
    steps: [
      "Review active and due assignments.",
      "Open the correct subject and class task.",
      "Confirm submission or grading status without assuming an unavailable service means no work.",
    ],
  },
  {
    href: "/student/cbt-exams",
    section: "Learning",
    steps: [
      "Review only active exam sessions assigned to you.",
      "Read the instructions before starting a timed attempt.",
      "Submit once and wait for the confirmed result state.",
    ],
  },
  {
    href: "/student/report-card",
    section: "Learning",
    steps: [
      "Review published report cards by academic period.",
      "Open the correct report before downloading.",
      "Use the authorised PDF action to keep the school record protected.",
    ],
  },
  {
    href: "/superadmin",
    section: "Platform operations",
    steps: [
      "Review platform-wide tenant and service indicators.",
      "Open the affected tenant or system service.",
      "Use audited controls for plan, feature and lifecycle changes.",
    ],
  },
  {
    href: "/applicant",
    section: "Admissions journey",
    steps: [
      "Review the completion checklist for your application.",
      "Add only accurate details and required documents.",
      "Submit when every required section is complete, then follow the recorded decision timeline.",
    ],
  },
];

export const PAGE_GUIDES: PageGuide[] = GUIDE_SEEDS.map((seed) => ({
  key: seed.key ?? (seed.href.replaceAll("/", "-").replace(/^-/, "") || "workspace"),
  section: seed.section,
  href: seed.href,
  title: seed.title ?? titleFromHref(seed.href),
  description:
    seed.description ??
    `A concise walkthrough for the ${titleFromHref(seed.href).toLowerCase()} workspace.`,
  steps: seed.steps,
}));

export function getPageGuide(
  pathname: string,
  page: { title: string; description?: string },
): PageGuide {
  const match = [...PAGE_GUIDES]
    .sort((a, b) => b.href.length - a.href.length)
    .find((guide) => pathname === guide.href || pathname.startsWith(`${guide.href}/`));

  if (match) {
    return { ...match, title: page.title, description: page.description ?? match.description };
  }

  return {
    key: pathname.replaceAll("/", "-").replace(/^-/, "") || "workspace",
    section: "AuraEDU workspace",
    href: pathname,
    title: page.title,
    description: page.description ?? `Guidance for ${page.title}.`,
    steps: [
      "Review the page description and current status before taking action.",
      "Use the primary action or the navigation rail to continue the workflow.",
      "Treat unavailable data as a dependency issue rather than an empty result.",
    ],
  };
}

function titleFromHref(href: string): string {
  const segment = href.split("/").filter(Boolean).at(-1) ?? "Workspace";
  return segment.replaceAll("-", " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}
