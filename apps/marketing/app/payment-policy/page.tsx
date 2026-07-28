import type { Metadata } from "next";
import { TrustPage } from "@/components/trust-page";

export const metadata: Metadata = {
  title: "Payment policy",
  description:
    "Template payment policy for school fees through AuraEDU: Paystack processing, settlement, refunds and failed-payment retry. Pending legal review.",
};

const sections = [
  {
    title: "How payments are processed",
    copy: "Online fee payments in AuraEDU are processed by Paystack, including mobile money and card channels available to Ghanaian guardians. AuraEDU records and reconciles payments against each learner; it does not itself hold school funds.",
    points: [
      "Guardians see the payable amount and any transaction charge before confirming.",
      "Every payment receives a reference the school and the parent can both trace.",
      "Payment status flows from the provider's confirmation—not from optimistic guesses.",
    ],
  },
  {
    title: "Settlement",
    copy: "Collected funds settle to the school's configured account on Paystack's settlement schedule for the chosen channel. AuraEDU shows expected and completed settlement states so the bursar can reconcile without chasing statements.",
  },
  {
    title: "Refunds and reversals",
    copy: "Refunds are a school decision, initiated by the school for the payments it controls. Once approved, refunds follow the provider's channel rules and timelines, and both the school and the guardian can see the refund state against the original receipt.",
  },
  {
    title: "Failed-payment retry",
    copy: "When a mobile money or card attempt fails, no fee is marked paid and no partial state is stored. The guardian can retry safely, schools can see failed attempts for follow-up, and duplicate submissions are rejected through idempotent processing.",
  },
  {
    title: "Charges and who pays them",
    copy: "Transaction charges are set by the payment channel, not by AuraEDU. Whether the school absorbs the charge or passes it to the guardian is a configuration choice the school makes, and it is disclosed before payment is confirmed.",
  },
  {
    title: "Disputes and records",
    copy: "Every invoice, payment, receipt and refund keeps an auditable history inside the school's tenant. Questions about a specific payment should start with the school's accounts office, which holds the full record.",
  },
] as const;

export default function PaymentPolicyPage() {
  return (
    <TrustPage
      eyebrow="Payment policy"
      title="School fees, handled in the open."
      introduction="How money moves when a school collects fees through AuraEDU: processing, settlement, refunds and what happens when a payment fails."
      updated="28 July 2026"
      notice="Template pending legal review. This page becomes binding only after the legal review gate (AURAEDU_LEGAL_REVIEW_CONFIRMED) is signed off; until then it is indicative, not contractual."
      sections={sections}
    />
  );
}
