"use client";

import * as React from "react";
import { UploadCloud, Loader2, X } from "lucide-react";
import { Button } from "@auraedu/ui";
import { completeUploadAction, requestSignedUploadAction } from "@/lib/tenant-actions";

export interface LogoUploaderProps {
  tenantCode: string;
  value?: string | null;
  onChange: (url: string | null) => void;
  disabled?: boolean;
}

export function LogoUploader({ tenantCode, value, onChange, disabled }: LogoUploaderProps) {
  const inputRef = React.useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  async function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      setError("Please select an image file.");
      return;
    }

    setUploading(true);
    setError(null);

    try {
      const signed = await requestSignedUploadAction(file.name, `${tenantCode}/logos`, "image");
      if ("error" in signed && signed.error) {
        setError(signed.error);
        return;
      }

      const formData = new FormData();
      formData.append("file", file);
      formData.append("api_key", signed.api_key);
      formData.append("timestamp", String(signed.timestamp));
      formData.append("signature", signed.signature);
      formData.append("folder", signed.folder);
      formData.append("public_id", signed.file_id);

      const uploadRes = await fetch(
        signed.upload_url ?? `https://api.cloudinary.com/v1_1/${signed.cloud_name}/image/upload`,
        {
          method: "POST",
          body: formData,
        },
      );

      if (!uploadRes.ok) {
        const body = await uploadRes.text();
        setError(`Cloudinary upload failed: ${uploadRes.status} ${body.slice(0, 200)}`);
        return;
      }

      const uploadJson = (await uploadRes.json()) as {
        secure_url: string;
        public_id: string;
        bytes: number;
        resource_type: string;
      };

      const completed = await completeUploadAction(
        signed.file_id,
        uploadJson.secure_url,
        uploadJson.public_id,
        uploadJson.bytes,
        file.type,
      );

      if (completed.error) {
        setError(completed.error);
        return;
      }

      onChange(uploadJson.secure_url);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Upload failed.");
    } finally {
      setUploading(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  return (
    <div className="space-y-2">
      <label className="block text-sm font-medium">School logo</label>
      <div className="flex items-center gap-4">
        {value ? (
          <div className="relative">
            <img
              src={value}
              alt="School logo preview"
              className="size-20 rounded-[var(--radius-sm)] border border-border object-contain bg-surface"
            />
            <button
              type="button"
              onClick={() => onChange(null)}
              disabled={disabled ?? uploading}
              className="absolute -right-2 -top-2 grid size-5 place-items-center rounded-full bg-destructive text-white hover:opacity-90 disabled:opacity-50"
              aria-label="Remove logo"
            >
              <X className="size-3" />
            </button>
          </div>
        ) : (
          <div className="grid size-20 place-items-center rounded-[var(--radius-sm)] border border-dashed border-border bg-surface text-muted-foreground">
            <UploadCloud className="size-6" />
          </div>
        )}
        <div>
          <input
            ref={inputRef}
            type="file"
            accept="image/*"
            className="sr-only"
            onChange={(e) => {
              void handleFileSelect(e);
            }}
            disabled={disabled ?? uploading}
          />
          <Button
            type="button"
            variant="secondary"
            onClick={() => inputRef.current?.click()}
            disabled={disabled ?? uploading}
            loading={uploading}
            loadingLabel="Uploading"
          >
            {uploading ? (
              <Loader2 className="mr-2 size-4 animate-spin" />
            ) : (
              <UploadCloud className="mr-2 size-4" />
            )}
            {uploading ? "Uploading..." : value ? "Change logo" : "Upload logo"}
          </Button>
        </div>
      </div>
      {error ? <p className="text-sm text-destructive">{error}</p> : null}
    </div>
  );
}
