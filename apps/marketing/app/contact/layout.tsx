import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Contact",
  description:
    "Talk with AuraEDU about a school workflow, rollout, pricing question or support need.",
};

export default function ContactLayout({ children }: { children: React.ReactNode }) {
  return children;
}
