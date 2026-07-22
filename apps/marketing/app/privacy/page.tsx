import type { Metadata } from "next";
import { TrustPage } from "@/components/trust-page";

export const metadata: Metadata = {
  title: "Privacy",
  description:
    "How AuraEDU approaches data minimisation, school ownership, consent, retention and privacy rights.",
};

const sections = [
  {
    title: "Who controls school data",
    copy: "Each institution controls the records it places in AuraEDU. AuraEDU processes those records only to provide the configured services, while public website enquiries are handled for the purpose stated at collection.",
    points: [
      "School data remains separated by tenant at application and database boundaries.",
      "People receive access through explicit roles and permissions.",
      "We do not sell school, learner, family or applicant data.",
    ],
  },
  {
    title: "What we collect",
    copy: "We collect only the information needed for an enabled workflow. That can include identity and contact details, school records, application information, payment references, support correspondence and security telemetry.",
    points: [
      "Sensitive fields are limited to the workflow that requires them.",
      "Public onboarding requests require an explicit consent record.",
      "Operational logs are designed to exclude credentials and direct personal identifiers.",
    ],
  },
  {
    title: "Why information is used",
    copy: "Information supports school operations, learning, admissions, communication, security, support and institution-approved analytics. AI features use scoped, approved information and do not make admission or teaching decisions by themselves.",
  },
  {
    title: "Retention and deletion",
    copy: "Retention follows the institution's policy, legal duties and the purpose for which data was collected. Expired assistant conversations and other temporary records are removed through scheduled retention jobs. Schools can request supported export or deletion workflows.",
  },
  {
    title: "Service providers and transfers",
    copy: "Where infrastructure, communication, payment or AI providers are used, access is limited to the service being delivered. Provider configuration is tenant-aware, secret-backed and subject to contractual and security review.",
  },
  {
    title: "Your choices and questions",
    copy: "People can ask their institution to correct or access school-held records. Questions about AuraEDU's own website enquiries, privacy practices or a suspected incident can be sent to privacy@auraedu.com.",
  },
] as const;

export default function PrivacyPage() {
  return (
    <TrustPage
      eyebrow="Privacy at AuraEDU"
      title="Data serves the school community—not the other way around."
      introduction="Privacy is an operating rule across AuraEDU. We minimise collection, preserve institutional control and make high-impact use visible to the people responsible for it."
      updated="20 July 2026"
      sections={sections}
    />
  );
}
