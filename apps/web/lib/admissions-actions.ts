"use server";
import { revalidatePath } from "next/cache";
import { createServerClient } from "./api";
export async function attachApplicationDocument(
  applicationId: string,
  fileId: string,
  documentType: string,
  fileName: string,
) {
  const client = await createServerClient();
  await client.post(`/api/v1/applications/${applicationId}/documents`, {
    file_id: fileId,
    document_type: documentType,
    file_name: fileName,
  });
  revalidatePath("/applicant");
}
