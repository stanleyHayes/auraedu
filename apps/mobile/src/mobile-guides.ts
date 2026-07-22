export interface MobileGuide {
  title: string;
  description: string;
  steps: string[];
}

const STEPS: Record<string, string[]> = {
  Today: [
    "Review the role-aware school brief.",
    "Open the card that needs your attention.",
    "Use the floating tabs to move between daily work, notices and your profile.",
  ],
  "My work": [
    "Review only the modules enabled for your school and role.",
    "Open the relevant class, learner or task.",
    "Return here when you need to switch workflows.",
  ],
  Notices: [
    "Review current tenant-scoped school updates.",
    "Enable push only when you want device alerts.",
    "Open a notice to follow its relevant school workflow.",
  ],
  Profile: [
    "Confirm the active school, role and account.",
    "Replay the guided tour whenever you need it.",
    "Sign out to revoke the server refresh session and remove this device registration.",
  ],
  "My children": [
    "Choose the child whose records you want to view.",
    "Confirm the school and class context.",
    "Open attendance, fees, results or reports from the family workspace.",
  ],
  "My classes": [
    "Review only classes assigned to your staff identity.",
    "Open a class to work with its authoritative roster.",
    "Continue to attendance or scores without entering a learner identifier manually.",
  ],
  Attendance: [
    "Choose an assigned class and date.",
    "Confirm the resolved roster before marking learners.",
    "Submit the complete register and review the saved outcome.",
  ],
  Results: [
    "Choose the correct learner and academic period.",
    "Review published results and unavailable states honestly.",
    "Use report cards for the official generated document.",
  ],
  "Record scores": [
    "Choose an assessment first.",
    "Select a learner only from its authorised roster.",
    "Enter a score within the assessment maximum and confirm the saved result.",
  ],
  Assignments: [
    "Review active and due assignments.",
    "Open the correct subject task.",
    "Confirm submission and grading status from the server response.",
  ],
  Fees: [
    "Choose the correct learner and review invoices by currency.",
    "Confirm the outstanding amount and provider state.",
    "Use secure checkout only for an authorised invoice.",
  ],
  "Report cards": [
    "Choose the learner and academic period.",
    "Open only published report cards.",
    "Share the authenticated PDF; AuraEDU removes the temporary device copy afterwards.",
  ],
  Timetable: [
    "Review lessons grouped by weekday.",
    "Check the subject, class and time context.",
    "Treat service failure separately from a genuinely empty day.",
  ],
  Recommendations: [
    "Review only approved learning recommendations.",
    "Open the explanation and confidence context.",
    "Discuss guidance with a teacher rather than treating AI as a final decision.",
  ],
  "Review guidance": [
    "Choose one of your assigned learners.",
    "Read the evidence and explanation before deciding.",
    "Approve, reject or override with a clear professional reason.",
  ],
  "CBT exams": [
    "Open only an active exam assigned to you.",
    "Read the instructions before starting the timed attempt.",
    "Submit once and wait for the confirmed result.",
  ],
  "Career guidance": [
    "Review only approved guidance for the learner.",
    "Read the reasoning and supporting context.",
    "Use it as a conversation aid with teachers and family, not an automatic decision.",
  ],
};

export function getMobileGuide(title: string, description?: string): MobileGuide {
  return {
    title,
    description: description ?? `A short walkthrough for ${title.toLowerCase()}.`,
    steps: STEPS[title] ?? [
      "Review the current school and learner context.",
      "Use only actions available for your role.",
      "Confirm the saved result before leaving the screen.",
    ],
  };
}
