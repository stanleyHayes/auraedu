import { NextResponse } from "next/server";
import { isValidIdempotencyKey, publicOnboardingFailure } from "./policy";

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

  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return NextResponse.json(
      { code: "validation_error", message: "The form could not be read." },
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
        body: JSON.stringify(body),
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
        message: "Onboarding is temporarily unavailable. Please try again shortly.",
      },
      { status: 503 },
    );
  } finally {
    clearTimeout(timeout);
  }
}
