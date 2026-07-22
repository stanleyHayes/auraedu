import { NextResponse } from "next/server";

const gateway =
  process.env.API_GATEWAY_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export async function GET() {
  try {
    const response = await fetch(`${gateway.replace(/\/$/, "")}/api/v1/public/billing/plans`, {
      headers: { Accept: "application/json" },
      next: { revalidate: 300 },
    });

    if (!response.ok) return NextResponse.json({ data: [] });
    return NextResponse.json(await response.json());
  } catch {
    return NextResponse.json({ data: [] });
  }
}
