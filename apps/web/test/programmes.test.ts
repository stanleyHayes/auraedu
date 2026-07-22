import assert from "node:assert/strict";
import test from "node:test";
import { findCatalogueSelection, type Programme } from "../lib/programmes.ts";

const catalogue: Programme[] = [
  {
    id: "11111111-1111-4111-8111-111111111111",
    tenant_id: "school-one",
    code: "SCI",
    name: "General Science",
    slug: "general-science",
    summary: "Science programme",
    description: "Verified programme",
    status: "published",
    version: 2,
    created_at: "2026-07-19T12:00:00Z",
    updated_at: "2026-07-19T12:00:00Z",
    intakes: [
      {
        id: "22222222-2222-4222-8222-222222222222",
        tenant_id: "school-one",
        programme_id: "11111111-1111-4111-8111-111111111111",
        name: "September 2026",
        starts_at: "2026-09-01T08:00:00Z",
        application_opens_at: "2026-07-01T08:00:00Z",
        application_closes_at: "2026-08-15T23:59:59Z",
        status: "open",
        version: 2,
        created_at: "2026-07-19T12:00:00Z",
        updated_at: "2026-07-19T12:00:00Z",
      },
    ],
  },
];

void test("accepts only an intake belonging to the selected catalogue programme", () => {
  assert.equal(
    findCatalogueSelection(catalogue, catalogue[0]?.id, catalogue[0]?.intakes[0]?.id)?.intake.name,
    "September 2026",
  );
  assert.equal(
    findCatalogueSelection(catalogue, catalogue[0]?.id, "33333333-3333-4333-8333-333333333333"),
    null,
  );
  assert.equal(
    findCatalogueSelection(
      catalogue,
      "44444444-4444-4444-8444-444444444444",
      catalogue[0]?.intakes[0]?.id,
    ),
    null,
  );
});
