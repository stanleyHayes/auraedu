// Feature-flag catalog for the superadmin flags grid.
//
// SOURCE OF TRUTH: contracts/features/features.yaml (agent_plan Appendix A) — feature keys,
// plan tiers, and descriptions. Codegen (`make contracts`) currently emits only the Go
// catalog (apps/tenant-service/internal/domain/catalog.go); there is no generated TS copy
// in packages/shared-types or packages/feature-flags yet, so this mirror is maintained by
// hand until codegen emits one. The tenant-service snapshot already returns every catalog
// key with plan_required; this list adds the descriptions and a stable display order.

export interface FeatureCatalogEntry {
  key: string;
  planRequired: string;
  description: string;
}

export const FEATURE_CATALOG: FeatureCatalogEntry[] = [
  { key: "public_website", planRequired: "starter", description: "Public school website" },
  { key: "admissions", planRequired: "growth", description: "Online admissions" },
  {
    key: "student_management",
    planRequired: "starter",
    description: "Students, guardians, enrollment",
  },
  {
    key: "staff_management",
    planRequired: "starter",
    description: "Teaching & non-teaching staff",
  },
  {
    key: "academic_management",
    planRequired: "core",
    description: "Academic years, terms, classes, subjects, curriculum",
  },
  {
    key: "parent_portal",
    planRequired: "starter",
    description: "Parent portal (web + mobile)",
  },
  {
    key: "student_portal",
    planRequired: "growth",
    description: "Student portal (web + mobile)",
  },
  {
    key: "teacher_portal",
    planRequired: "starter",
    description: "Teacher portal (web + mobile)",
  },
  { key: "attendance", planRequired: "starter", description: "Daily & subject attendance" },
  { key: "assignments", planRequired: "growth", description: "Assignments" },
  { key: "assessments", planRequired: "growth", description: "Tests, exams, scores" },
  {
    key: "cbt_exams",
    planRequired: "professional",
    description: "Computer-based / online exams",
  },
  {
    key: "report_cards",
    planRequired: "starter",
    description: "Report cards & transcripts (PDF)",
  },
  { key: "fees", planRequired: "growth", description: "Fees, invoices, balances" },
  {
    key: "online_payments",
    planRequired: "professional",
    description: "Payment gateway integrations",
  },
  { key: "timetable", planRequired: "growth", description: "Timetabling" },
  { key: "library", planRequired: "professional", description: "Library management" },
  { key: "hostel", planRequired: "professional", description: "Hostel management" },
  { key: "transport", planRequired: "professional", description: "Transport management" },
  { key: "announcements", planRequired: "starter", description: "Announcements" },
  {
    key: "notifications",
    planRequired: "starter",
    description: "Notification service (messages, templates, subscriptions)",
  },
  {
    key: "email_notifications",
    planRequired: "starter",
    description: "Email notifications",
  },
  { key: "sms_notifications", planRequired: "growth", description: "SMS notifications" },
  {
    key: "whatsapp_notifications",
    planRequired: "professional",
    description: "WhatsApp notifications",
  },
  { key: "analytics", planRequired: "professional", description: "Dashboards & KPIs" },
  {
    key: "ai_recommendations",
    planRequired: "ai_plus",
    description: "AI learning recommendations",
  },
  {
    key: "ai_predictions",
    planRequired: "ai_plus",
    description: "AI risk & performance predictions",
  },
  {
    key: "career_guidance",
    planRequired: "ai_plus",
    description: "AI career/course guidance",
  },
  { key: "billing", planRequired: "core", description: "SaaS subscription & billing" },
  {
    key: "custom_domain",
    planRequired: "professional",
    description: "Custom domain for the school",
  },
  {
    key: "file_management",
    planRequired: "core",
    description: "File uploads and storage",
  },
];
