import type { Metadata } from "next";
import { TrustPage } from "@/components/trust-page";

export const metadata: Metadata = {
  title: "Accessibility",
  description:
    "AuraEDU's commitment to WCAG 2.2 AA, inclusive school workflows and accessible support.",
};

const sections = [
  {
    title: "Our target",
    copy: "AuraEDU targets WCAG 2.2 Level AA across public and authenticated web experiences. Accessibility is part of design review, implementation and release verification rather than a one-time certification exercise.",
  },
  {
    title: "Keyboard and focus",
    copy: "Core journeys are designed to work without a pointer. Pages provide visible focus, logical order, semantic landmarks and a skip link so repeated navigation can be bypassed.",
  },
  {
    title: "Screen readers and structure",
    copy: "Controls use accessible names and states, pages preserve a meaningful heading hierarchy, status changes are announced and decorative imagery is excluded from the accessibility tree.",
  },
  {
    title: "Vision, motion and contrast",
    copy: "The design system specifies readable contrast, scalable text and non-colour cues. Motion respects reduced-motion preferences and must never be required to understand or complete a workflow.",
  },
  {
    title: "Content and localisation",
    copy: "We prefer direct language, descriptive links, labelled inputs and error messages that explain recovery. Layouts are built to tolerate text expansion as AuraEDU's localisation coverage grows.",
  },
  {
    title: "Report a barrier",
    copy: "If something prevents you from using AuraEDU, email accessibility@auraedu.com with the page, device, browser or assistive technology and what you were trying to do. We will acknowledge the report and work toward a practical resolution.",
  },
] as const;

export default function AccessibilityPage() {
  return (
    <TrustPage
      eyebrow="Accessibility at AuraEDU"
      title="Every school workflow should remain open to every person."
      introduction="Education software succeeds only when the people who depend on it can perceive, understand and operate it. We design for that range from the beginning."
      updated="20 July 2026"
      sections={sections}
    />
  );
}
