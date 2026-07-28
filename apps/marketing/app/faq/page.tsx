import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Eyebrow } from "@/components/brand-primitives";
import { FaqAccordion, type FaqGroup } from "@/components/faq-accordion";
import { ScrollReveal } from "@/components/motion-primitives";

export const metadata: Metadata = {
  title: "FAQ",
  description:
    "Straight answers for schools: data migration, offline resilience, BECE/WASSCE report formats, mobile money fees, training, data ownership and children's data security.",
};

const groups: FaqGroup[] = [
  {
    title: "Settling in",
    eyebrow: "Migration & onboarding",
    items: [
      {
        question: "We run on spreadsheets and paper registers. How does migration actually work?",
        answer:
          "Carefully, in stages. We take your existing spreadsheets and register data, map them into AuraEDU's structure and validate every import with you before it becomes the record of truth. Nothing is switched on until your team has checked the migrated learners, guardians, classes and balances against your own books. Paper-only history can be captured progressively—you do not need a perfect digital archive to start.",
      },
      {
        question: "How are teachers and staff trained?",
        answer:
          "Onboarding is role-based, not a single marathon session. Administrators learn setup and permissions, teachers learn attendance and marks in the flow of a real school day, and the front office learns fees and communication. Training is included in the rollout plan we agree before anything is provisioned, and each role opens with supported workflows rather than a manual.",
      },
      {
        question: "Who owns our data, and can we export it?",
        answer:
          "Your school does. Each institution controls the records it places in AuraEDU; we process them only to provide the services you have enabled. Schools can request supported export and deletion workflows, and leaving the platform does not strand your records with us.",
      },
    ],
  },
  {
    title: "Daily reality",
    eyebrow: "Power, network & formats",
    items: [
      {
        question: "What happens when the power or internet goes down mid-day?",
        answer:
          "The school day should not wait for the network. Core daily actions are designed to tolerate interruption: work entered during an outage is queued and synchronised when connectivity returns, and records are never half-saved. Reporting and communication catch up automatically rather than demanding re-entry.",
      },
      {
        question: "Do report cards match BECE and WASSCE expectations?",
        answer:
          "AuraEDU's assessment and report-card workflows are built for Ghanaian school structures—terms, subjects, continuous assessment and exam components—and are configurable to the format your school presents to parents and to candidates preparing for BECE and WASSCE. Your grading scheme, remarks structure and school branding are yours to set.",
      },
    ],
  },
  {
    title: "Money & trust",
    eyebrow: "Fees, security & responsibility",
    items: [
      {
        question: "Who pays the mobile money transaction fees?",
        answer:
          "Payment processing is handled through Paystack, and transaction charges are a real cost either the school or the parent must bear. AuraEDU makes the charge visible before a guardian confirms payment, and who absorbs it is a school policy decision set during configuration. Our payment policy explains settlement and refunds in plain terms.",
      },
      {
        question: "How is our children's data protected?",
        answer:
          "Children's records sit inside your school's isolated tenant boundary—never pooled with other schools. Access is permission-based down to each role, sensitive use is auditable, and AI features explain their evidence and never make admission or teaching decisions alone. Our security and privacy pages describe the controls, and our practices are aligned with Ghana's Data Protection Act, 2012 (Act 843).",
      },
    ],
  },
];

export default function FaqPage() {
  return (
    <div className="overflow-hidden">
      <section className="trust-stage text-white">
        <ScrollReveal className="mx-auto max-w-7xl px-6 py-24 sm:py-28">
          <Eyebrow inverse>Questions schools actually ask</Eyebrow>
          <h1 className="mt-5 max-w-[13ch] text-balance text-[clamp(3.2rem,6vw,6.5rem)] font-bold leading-[0.9] tracking-[-0.055em]">
            Straight answers, <span className="text-teal-bright">before the demo.</span>
          </h1>
          <p className="mt-7 max-w-3xl text-lg leading-8 text-slate-300">
            Migration, outages, report formats, MoMo charges and who is responsible for children’s
            data. If your question is not here, ask it directly—we reply with substance, not a sales
            sequence.
          </p>
        </ScrollReveal>
      </section>

      <section className="bg-cool-mist">
        <ScrollReveal className="mx-auto max-w-7xl px-6 py-20">
          <FaqAccordion groups={groups} />
        </ScrollReveal>
      </section>

      <section className="bg-white">
        <div className="mx-auto flex max-w-7xl flex-col gap-6 px-6 py-20 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <Eyebrow>Still deciding?</Eyebrow>
            <h2 className="mt-3 max-w-[22ch] text-4xl font-bold tracking-[-0.04em] text-navy-deep">
              Bring the hardest question your school has.
            </h2>
          </div>
          <div className="flex flex-wrap gap-3">
            <Link href="/contact" className="cta-solid">
              Ask us directly <ArrowRight className="size-4" aria-hidden="true" />
            </Link>
            <Link href="/for-parents" className="cta-outline">
              Information for parents
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
