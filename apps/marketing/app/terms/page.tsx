import type { Metadata } from "next";
import { TrustPage } from "@/components/trust-page";

export const metadata: Metadata = {
  title: "Terms of service",
  description:
    "Template terms of service for schools using AuraEDU: accounts, data protection under Act 843, acceptable use and termination. Pending legal review.",
};

const sections = [
  {
    title: "Accounts and responsibility",
    copy: "Each school operates under a single institutional account with named administrators. The school is responsible for the accuracy of the records it enters, for assigning roles and permissions carefully, and for the actions taken under its accounts.",
    points: [
      "Tenants are provisioned only after an approved onboarding review—never automatically.",
      "Administrator access must be held by people the school has authorized.",
      "Schools keep their own user lists, roles and enabled modules under control.",
    ],
  },
  {
    title: "Data protection and Act 843",
    copy: "The school remains the data controller for the records it places in AuraEDU; AuraEDU acts as a processor providing the configured services. Processing practices are aligned with Ghana's Data Protection Act, 2012 (Act 843), including purpose limitation, minimisation and respect for data-subject rights routed through the school.",
    points: [
      "School data is isolated per tenant at application and database boundaries.",
      "We do not sell school, learner, family or applicant data.",
      "Schools can request supported export or deletion workflows.",
    ],
  },
  {
    title: "Acceptable use",
    copy: "AuraEDU is provided for running legitimate school operations. Accounts must not be used to store unlawful content, to attempt access to another school's tenant, to probe or disrupt the platform, or to misrepresent identity or authority.",
  },
  {
    title: "Service changes and availability",
    copy: "Modules evolve under published contracts, and schools enable the operating set that fits them. We communicate material changes in advance and design maintenance to respect the school day, but uninterrupted availability is not guaranteed.",
  },
  {
    title: "Termination",
    copy: "Either side may end the relationship under the notice terms agreed at onboarding. On termination the school can request a supported export of its records within a defined window, after which tenant data is removed under the retention schedule.",
  },
  {
    title: "Liability and disputes",
    copy: "Liability is limited to the extent permitted by law and the commercial terms agreed with each school. Schools remain responsible for the decisions they take with the information AuraEDU presents—AI features advise with evidence; people decide.",
  },
] as const;

export default function TermsPage() {
  return (
    <TrustPage
      eyebrow="Terms of service"
      title="The agreement between AuraEDU and your school."
      introduction="These terms set out how schools use AuraEDU: accounts, data protection, acceptable use and how the relationship ends. They are written to be read, not endured."
      updated="28 July 2026"
      notice="Template pending legal review. This page becomes binding only after the legal review gate (AURAEDU_LEGAL_REVIEW_CONFIRMED) is signed off; until then it is indicative, not contractual."
      sections={sections}
    />
  );
}
