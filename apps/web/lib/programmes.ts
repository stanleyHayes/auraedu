import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";
import type { OpenAPI } from "@auraedu/shared-types";

export type Programme = OpenAPI.admissions_v1.components["schemas"]["Programme"];
export type Intake = OpenAPI.admissions_v1.components["schemas"]["Intake"];

interface ProgrammeList {
  data: Programme[];
}

export async function fetchPublicProgrammes(tenantCode: string): Promise<Programme[]> {
  if (!tenantCode) return [];
  const url = new URL("/api/v1/public/programmes", gatewayInternalUrl);
  url.searchParams.set("limit", "100");
  try {
    const response = await fetch(url, {
      headers: { [tenantHeaderName]: tenantCode },
      next: { revalidate: 60, tags: [`programmes:${tenantCode}`] },
    });
    if (!response.ok) return [];
    return ((await response.json()) as ProgrammeList).data ?? [];
  } catch {
    return [];
  }
}

export function findCatalogueSelection(
  programmes: Programme[],
  programmeID?: string,
  intakeID?: string,
) {
  const programme = programmes.find((item) => item.id === programmeID);
  const intake = programme?.intakes.find((item) => item.id === intakeID);
  return programme && intake ? { programme, intake } : null;
}
