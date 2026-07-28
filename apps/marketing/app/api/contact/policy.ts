export interface ContactSubmission {
  name?: unknown;
  school?: unknown;
  email?: unknown;
  phone?: unknown;
  country?: unknown;
  interest?: unknown;
  message?: unknown;
  accepted_terms?: unknown;
  website?: unknown;
}

export interface ContactOnboardingRequest {
  school_name: string;
  administrator_name: string;
  email: string;
  phone: string | null;
  country_code: string;
  plan: "starter";
  priorities: string;
  privacy_notice_version: string;
  accepted_terms: true;
  website: "";
}

export const CONTACT_INTERESTS = ["demo", "pricing", "support", "other"] as const;

const PRIVACY_NOTICE_VERSION = "2026-07-18";

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/**
 * Shape a public contact message into the existing onboarding intake schema.
 * There is no generic contact endpoint; rather than invent one, contact
 * messages ride the same reviewed intake with a `[contact:<interest>]` source
 * tag in `priorities` so platform staff can route them. `plan` is required by
 * the schema, so the lowest tier is sent and the tag makes intent explicit.
 */
export function buildContactOnboardingRequest(
  input: ContactSubmission,
): ContactOnboardingRequest | null {
  const name = text(input.name);
  const school = text(input.school);
  const email = text(input.email);
  const phone = text(input.phone);
  const country = text(input.country).toUpperCase();
  const interest = text(input.interest);
  const message = text(input.message);

  // Honeypot: bots fill the hidden "website" field; humans never see it.
  if (text(input.website) !== "") return null;
  if (name.length < 2 || name.length > 160) return null;
  if (school.length < 2 || school.length > 200) return null;
  if (email.length > 320 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return null;
  if (phone.length > 40) return null;
  if (!/^[A-Z]{2}$/.test(country)) return null;
  if (!(CONTACT_INTERESTS as readonly string[]).includes(interest)) return null;
  if (message.length < 10 || message.length > 1800) return null;
  if (input.accepted_terms !== true) return null;

  return {
    school_name: school,
    administrator_name: name,
    email,
    phone: phone || null,
    country_code: country,
    plan: "starter",
    priorities: `[contact:${interest}] ${message}`.slice(0, 2000),
    privacy_notice_version: PRIVACY_NOTICE_VERSION,
    accepted_terms: true,
    website: "",
  };
}
