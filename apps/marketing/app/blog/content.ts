export interface FieldNote {
  slug: string;
  area: string;
  number: string;
  title: string;
  summary: string;
  image: string;
  imageAlt: string;
  tone: "blue" | "teal" | "orange";
  readTime: string;
  thesis: string;
  principles: string[];
  sections: { heading: string; body: string[] }[];
}

export const fieldNotes: FieldNote[] = [
  {
    slug: "ai-should-reduce-teacher-workload",
    area: "Teaching",
    number: "01",
    title: "AI should reduce teacher workload—not move it around",
    summary:
      "Recommendations need evidence, confidence and a review path. The final academic decision remains human.",
    image: "/images/auraedu/role-teacher-source.png",
    imageAlt: "A teacher supporting a student during a lesson",
    tone: "blue",
    readTime: "6 min read",
    thesis:
      "The useful test for school AI is not whether it can produce an answer. It is whether a teacher can understand, verify and act on that answer with less effort than before.",
    principles: [
      "Evidence travels with every recommendation.",
      "Teachers approve consequential academic decisions.",
      "Silence is better than a low-confidence interruption.",
    ],
    sections: [
      {
        heading: "Start with the teacher’s day",
        body: [
          "A teacher already carries lesson preparation, observation, marking, pastoral care and communication. An AI feature that adds another inbox, another dashboard or another set of fields has not reduced workload; it has only moved it.",
          "AuraEDU begins with the decision the teacher needs to make. The system gathers the relevant attendance, assessment and engagement context, then offers a small, reviewable next step inside the workflow where that decision already happens.",
        ],
      },
      {
        heading: "Explanation is part of the interface",
        body: [
          "A risk label without its evidence is not guidance. Teachers need to see which observations contributed, how recent they are and where uncertainty remains. That context should be readable without opening a technical report.",
          "Confidence is not decoration either. When evidence is incomplete or contradictory, the product should say so plainly and avoid creating false urgency.",
        ],
      },
      {
        heading: "Keep responsibility human",
        body: [
          "AI can surface patterns that are difficult to notice across many records. It should not make disciplinary, placement or progression decisions on behalf of educators. Those decisions require professional judgement and knowledge of the learner that data cannot fully capture.",
          "A complete review trail records what was suggested, what evidence was shown, who reviewed it and what action was taken. Accountability becomes a product behavior, not a policy document stored elsewhere.",
        ],
      },
    ],
  },
  {
    slug: "tenant-isolation-is-product-experience",
    area: "Platform",
    number: "02",
    title: "Tenant isolation is part of the product experience",
    summary:
      "A school should feel fully independent even while sharing one platform. That promise shapes every request, table and event.",
    image: "/images/auraedu/role-leader-source.png",
    imageAlt: "School leaders reviewing information together",
    tone: "teal",
    readTime: "5 min read",
    thesis:
      "Multi-tenancy is successful when each school experiences a coherent, private institution—not a partition inside somebody else’s database.",
    principles: [
      "School context is resolved before data access.",
      "Authorization is enforced in services, not implied by screens.",
      "Brand, modules and policies remain school-owned.",
    ],
    sections: [
      {
        heading: "A boundary people can feel",
        body: [
          "Isolation begins below the interface, but its quality is visible everywhere. A parent should never encounter another school’s language, programme or learner. A staff member should never see a module their institution has not enabled. A school leader should understand which policies apply without decoding global defaults.",
          "That consistency comes from carrying verified tenant context through the gateway, service boundary, database session and event envelope—not from adding a school filter to a page query.",
        ],
      },
      {
        heading: "Fail closed at relationship boundaries",
        body: [
          "A role name is not enough to authorize a record. Teachers need assigned-class scope; guardians need explicit learner links; students need their current enrolment. If the service that owns that relationship cannot answer, the safer response is unavailable—not tenant-wide data.",
          "This rule can feel strict during development, but it prevents a degraded dependency from quietly becoming a privacy incident.",
        ],
      },
      {
        heading: "Independence without duplication",
        body: [
          "Schools benefit from shared infrastructure, security improvements and product updates. They should not have to accept a generic identity in return. Runtime branding, feature policy, academic structures and communication choices preserve institutional character on a common core.",
          "The result is one maintainable platform that still behaves like each school’s own operating environment.",
        ],
      },
    ],
  },
  {
    slug: "a-living-system-should-feel-calm",
    area: "Design",
    number: "03",
    title: "A living system should still feel calm",
    summary:
      "The product connects people, daily rhythms and reliable foundations without turning education into a generic dashboard.",
    image: "/images/auraedu/role-family-source.png",
    imageAlt: "A parent and learner reviewing school information together",
    tone: "orange",
    readTime: "5 min read",
    thesis:
      "A school platform can hold enormous operational complexity while presenting each person with one clear next action and enough context to trust it.",
    principles: [
      "Show the next decision before the whole database.",
      "Use motion to explain change, not demand attention.",
      "Distinguish empty, unavailable and incomplete states.",
    ],
    sections: [
      {
        heading: "Complexity belongs in the system",
        body: [
          "Schools coordinate admissions, learning, finance, communication, people and public responsibility. The platform must respect that complexity without transferring it to every screen. A guardian does not need an operations console, and a teacher should not navigate an administrative information architecture to record today’s work.",
          "Role-focused navigation and progressive disclosure let the platform remain deep while the immediate experience stays legible.",
        ],
      },
      {
        heading: "Calm does not mean static",
        body: [
          "Motion can show that a set of modules belongs together, that information has changed or that a new level of detail is available. It becomes noise when every element competes to announce itself.",
          "AuraEDU uses short entrances, restrained depth and clear state transitions. Reduced-motion preferences are honored, and essential content remains available before animation runs.",
        ],
      },
      {
        heading: "Honesty is a visual quality",
        body: [
          "Zero learners and unavailable learner data are different situations. No lesson today and a failed timetable dependency are different situations. A polished interface should never blur those distinctions to keep a card looking complete.",
          "Explicit empty, loading, partial and error states make the system calmer because people can understand what they are seeing—and what they are not.",
        ],
      },
    ],
  },
];

export function getFieldNote(slug: string) {
  return fieldNotes.find((note) => note.slug === slug);
}
