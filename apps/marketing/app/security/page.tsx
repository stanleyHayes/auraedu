import type { Metadata } from "next";
import { TrustPage } from "@/components/trust-page";

export const metadata: Metadata = {
  title: "Security",
  description:
    "AuraEDU security principles covering tenant isolation, identity, encryption, auditability and resilient operations.",
};

const sections = [
  {
    title: "Tenant isolation",
    copy: "Every request carries an authenticated tenant context. Tenant-owned PostgreSQL tables enforce row-level security, services use scoped contracts rather than another service's database, and cross-school access is tested as a release gate.",
  },
  {
    title: "Identity and least privilege",
    copy: "Signed sessions, explicit permissions, role-aware interfaces and internal service authentication protect each boundary. Privileged roles require stronger controls, and client-supplied identity headers are never trusted at ingress.",
  },
  {
    title: "Encryption and secrets",
    copy: "Production traffic uses encrypted transport. Credentials and provider tokens come from managed secret stores, are excluded from source control and are redacted from application logs.",
  },
  {
    title: "Secure engineering",
    copy: "Contracts, migrations and generated clients are reviewed together. Automated gates cover authentication, authorization, injection, uploads, tenant boundaries, dependency risk and production configuration.",
  },
  {
    title: "Monitoring and response",
    copy: "Structured telemetry, owned alerts and incident runbooks cover service health, security signals, payments, communications, AI and background work. Audit trails preserve consequential human and automated actions.",
  },
  {
    title: "Responsible disclosure",
    copy: "If you believe you have found a vulnerability, email security@auraedu.com with the affected surface, reproduction steps and potential impact. Do not access data that is not yours or disrupt a school service while testing.",
  },
] as const;

export default function SecurityPage() {
  return (
    <TrustPage
      eyebrow="Security at AuraEDU"
      title="Trust is built into every boundary."
      introduction="Schools carry consequential information. AuraEDU treats isolation, least privilege, auditability and recovery as product capabilities—not infrastructure footnotes."
      updated="20 July 2026"
      sections={sections}
    />
  );
}
