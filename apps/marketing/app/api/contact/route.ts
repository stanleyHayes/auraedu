import { NextResponse } from "next/server";
import { isValidIdempotencyKey, publicOnboardingFailure } from "../onboarding/policy";
import { buildContactOnboardingRequest, type ContactSubmission } from "./policy";

const API_BASE =
  process.env.AURAEDU_API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export async function POST(request: Request) {
  const idempotencyKey = request.headers.get("idempotency-key");
  if (!isValidIdempotencyKey(idempotencyKey)) {
    return NextResponse.json(
      { code: "validation_error", message: "A valid request key is required." },
      { status: 422 },
    );
  }

  let body: ContactSubmission;
  try {
    body = (await request.json()) as ContactSubmission;
  } catch {
    return NextResponse.json(
      { code: "validation_error", message: "The form could not be read." },
      { status: 422 },
    );
  }

  // No generic contact endpoint exists; contact messages are composed into the
  // reviewed onboarding intake with a `[contact:<interest>]` source tag rather
  // than inventing a new backend.
  const onboardingRequest = buildContactOnboardingRequest(body);
  if (!onboardingRequest) {
    return NextResponse.json(
      {
        code: "validation_error",
        message: "We could not accept the message. Please check the form and try again.",
      },
      { status: 422 },
    );
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10_000);
  try {
    const response = await fetch(
      `${API_BASE.replace(/\/$/, "")}/api/v1/public/onboarding-requests`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": idempotencyKey,
          "X-Forwarded-For": request.headers.get("x-forwarded-for") ?? "",
          "X-Request-Id": request.headers.get("x-request-id") ?? crypto.randomUUID(),
        },
        body: JSON.stringify(onboardingRequest),
        cache: "no-store",
        signal: controller.signal,
      },
    );
    const payload = (await response.json().catch(() => null)) as unknown;
    if (!response.ok) {
      const failure = publicOnboardingFailure(response.status);
      return NextResponse.json(failure.body, { status: failure.status });
    }
    return NextResponse.json(payload, { status: 202 });
  } catch {
    return NextResponse.json(
      {
        code: "service_unavailable",
        message: "Contact is temporarily unavailable. Please try again shortly.",
      },
      { status: 503 },
    );
  } finally {
    clearTimeout(timeout);
  }
}
