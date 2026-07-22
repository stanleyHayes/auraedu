"use client";
import * as React from "react";
import { Loader2, UploadCloud } from "lucide-react";
import { Button } from "@auraedu/ui";
import { completeUploadAction, requestSignedUploadAction } from "@/lib/tenant-actions";
import { attachApplicationDocument } from "@/lib/admissions-actions";
export function ApplicationDocumentUploader({ applicationId }: { applicationId: string }) {
  const input = React.useRef<HTMLInputElement>(null);
  const [type, setType] = React.useState("transcript");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  async function upload(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    if (file.size > 10 * 1024 * 1024) {
      setError("Document must be 10 MB or smaller.");
      return;
    }
    if (!["application/pdf", "image/jpeg", "image/png"].includes(file.type)) {
      setError("Use a PDF, JPG, or PNG document.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const signed = await requestSignedUploadAction(
        file.name,
        `applications/${applicationId}`,
        file.type.startsWith("image/") ? "image" : "raw",
      );
      if (signed.error) throw new Error(signed.error);
      const data = new FormData();
      data.append("file", file);
      data.append("api_key", signed.api_key);
      data.append("timestamp", String(signed.timestamp));
      data.append("signature", signed.signature);
      data.append("folder", signed.folder);
      data.append("public_id", signed.file_id);
      const response = await fetch(
        signed.upload_url ??
          `https://api.cloudinary.com/v1_1/${signed.cloud_name}/${file.type.startsWith("image/") ? "image" : "raw"}/upload`,
        { method: "POST", body: data },
      );
      if (!response.ok) throw new Error("Secure document upload failed.");
      const uploaded = (await response.json()) as {
        secure_url: string;
        public_id: string;
        bytes: number;
      };
      const completed = await completeUploadAction(
        signed.file_id,
        uploaded.secure_url,
        uploaded.public_id,
        uploaded.bytes,
        file.type,
      );
      if (completed.error) throw new Error(completed.error);
      await attachApplicationDocument(applicationId, signed.file_id, type, file.name);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Upload failed");
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  }
  return (
    <div className="space-y-2 rounded-lg border border-dashed border-border p-4">
      <div className="flex flex-col gap-2 sm:flex-row">
        <select
          aria-label="Document type"
          value={type}
          onChange={(e) => setType(e.target.value)}
          className="h-10 rounded-md border border-border bg-background px-3 text-sm"
        >
          <option value="transcript">Transcript</option>
          <option value="certificate">Certificate</option>
          <option value="identity">Identity document</option>
          <option value="passport_photo">Passport photo</option>
          <option value="recommendation">Recommendation</option>
          <option value="other">Other</option>
        </select>
        <input
          ref={input}
          type="file"
          accept="application/pdf,image/jpeg,image/png"
          onChange={(event) => void upload(event)}
          className="sr-only"
          id={`document-${applicationId}`}
        />
        <Button
          type="button"
          variant="secondary"
          disabled={busy}
          onClick={() => input.current?.click()}
        >
          {busy ? (
            <Loader2 className="mr-2 size-4 animate-spin" />
          ) : (
            <UploadCloud className="mr-2 size-4" />
          )}
          {busy ? "Uploading securely…" : "Attach document"}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        PDF, JPG or PNG, up to 10 MB. Files are stored by the secure File Service; Admissions keeps
        only the file reference.
      </p>
      {error ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}
