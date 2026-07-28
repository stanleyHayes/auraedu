"use client";

import * as React from "react";
import { Plus, Trash2 } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createSectionAction,
  updateSectionAction,
  type AdminWebsiteActionResult,
} from "@/lib/admin-website-actions";

type WebsiteSection = OpenAPI.website_v1.components["schemas"]["Section"];

const SECTION_TYPES = ["hero", "text", "features", "gallery", "cta", "contact"] as const;
type SectionType = (typeof SECTION_TYPES)[number];

const TYPE_LABELS: Record<SectionType, string> = {
  hero: "Hero banner",
  text: "Text block",
  features: "Feature grid",
  gallery: "Gallery grid",
  cta: "Call to action",
  contact: "Contact cards",
};

interface FeatureRow {
  title: string;
  description: string;
  icon: string;
}

function contentRecord(section?: WebsiteSection): Record<string, unknown> {
  const content = section?.content;
  return content && typeof content === "object" ? content : {};
}

function textField(section: WebsiteSection | undefined, key: string): string {
  const value = contentRecord(section)[key];
  return typeof value === "string" ? value : "";
}

function itemRows(section?: WebsiteSection): FeatureRow[] {
  const items = contentRecord(section).items;
  if (!Array.isArray(items)) return [];
  return items
    .filter((item): item is Record<string, unknown> => item !== null && typeof item === "object")
    .map((item) => ({
      title: typeof item.title === "string" ? item.title : "",
      description: typeof item.description === "string" ? item.description : "",
      icon: typeof item.icon === "string" ? item.icon : "",
    }));
}

const inputClass = "w-full rounded-xl border border-border bg-background p-3 text-sm";

interface AdminWebsiteSectionFormProps {
  mode: "create" | "edit";
  pageId: string;
  sectionId?: string;
  initial?: WebsiteSection;
  nextSortOrder: number;
  onSuccess?: () => void;
}

export function AdminWebsiteSectionForm({
  mode,
  pageId,
  sectionId,
  initial,
  nextSortOrder,
  onSuccess,
}: AdminWebsiteSectionFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateSectionAction.bind(null, pageId, sectionId!)
    : createSectionAction.bind(null, pageId);

  const [state, formAction, pending] = React.useActionState<AdminWebsiteActionResult, FormData>(
    action,
    {},
  );
  const [type, setType] = React.useState<SectionType>(
    (SECTION_TYPES as readonly string[]).includes(initial?.type ?? "") ? initial!.type : "text",
  );
  const [rows, setRows] = React.useState<FeatureRow[]>(() => itemRows(initial));

  React.useEffect(() => {
    if (state.success && onSuccess) {
      onSuccess();
    }
  }, [state, onSuccess]);

  const showsItems = type === "features" || type === "gallery";
  const showsBody = type !== "contact";
  const showsCta = type === "hero" || type === "cta";

  function updateRow(index: number, key: keyof FeatureRow, value: string) {
    setRows((current) =>
      current.map((row, rowIndex) => (rowIndex === index ? { ...row, [key]: value } : row)),
    );
  }

  return (
    <form action={formAction} className="space-y-5">
      <input type="hidden" name="sort_order" value={initial?.sort_order ?? nextSortOrder} />

      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="type">Section type</Label>
          <Select
            id="type"
            name="type"
            value={type}
            onChange={(event) => setType(event.target.value as SectionType)}
          >
            {SECTION_TYPES.map((sectionType) => (
              <option key={sectionType} value={sectionType}>
                {TYPE_LABELS[sectionType]}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="status">Visibility</Label>
          <Select id="status" name="status" defaultValue={initial?.status ?? "draft"}>
            <option value="draft">Draft (hidden)</option>
            <option value="published">Published (live)</option>
          </Select>
        </div>

        {type !== "hero" ? (
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="title">{type === "contact" ? "Heading" : "Title"}</Label>
            <Input
              id="title"
              name="title"
              defaultValue={textField(initial, "title")}
              placeholder={
                type === "cta" ? "Ready to join us?" : "A short heading for this section"
              }
            />
          </div>
        ) : (
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="headline">Headline</Label>
            <Input
              id="headline"
              name="headline"
              defaultValue={textField(initial, "headline")}
              placeholder="A warm welcome to our school"
            />
          </div>
        )}

        {showsBody ? (
          <div className="space-y-1.5 sm:col-span-2">
            <Label htmlFor="body">{type === "text" ? "Body copy" : "Supporting text"}</Label>
            <textarea
              id="body"
              name="body"
              rows={type === "text" ? 6 : 3}
              defaultValue={textField(initial, "body")}
              className={inputClass}
              placeholder={
                type === "text"
                  ? "The main paragraph visitors read."
                  : "One or two sentences under the heading."
              }
            />
          </div>
        ) : null}

        {showsCta ? (
          <>
            <div className="space-y-1.5">
              <Label htmlFor="cta_label">Button label</Label>
              <Input
                id="cta_label"
                name="cta_label"
                defaultValue={textField(initial, "cta_label")}
                placeholder="Apply now"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="cta_url">Button link</Label>
              <Input
                id="cta_url"
                name="cta_url"
                defaultValue={textField(initial, "cta_url")}
                placeholder="/admissions"
              />
            </div>
          </>
        ) : null}

        {type === "contact" ? (
          <>
            <div className="space-y-1.5">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                name="email"
                type="email"
                defaultValue={textField(initial, "email")}
                placeholder="office@school.edu.gh"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="phone">Phone</Label>
              <Input
                id="phone"
                name="phone"
                defaultValue={textField(initial, "phone")}
                placeholder="+233 30 000 0000"
              />
            </div>
            <div className="space-y-1.5 sm:col-span-2">
              <Label htmlFor="address">Address</Label>
              <Input
                id="address"
                name="address"
                defaultValue={textField(initial, "address")}
                placeholder="12 Independence Avenue, Accra"
              />
            </div>
          </>
        ) : null}
      </div>

      {showsItems ? (
        <fieldset className="space-y-3 rounded-xl border border-border p-4">
          <legend className="px-1 text-sm font-semibold">
            {type === "gallery" ? "Gallery cards" : "Feature cards"}
          </legend>
          {rows.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No cards yet — add one to build the grid.
            </p>
          ) : (
            <ul className="space-y-3">
              {rows.map((row, index) => (
                <li
                  key={index}
                  className="grid gap-3 rounded-lg border border-border bg-background/60 p-3 sm:grid-cols-[1fr_1fr_auto]"
                >
                  <Input
                    aria-label={`Card ${index + 1} title`}
                    name="item_title"
                    value={row.title}
                    onChange={(event) => updateRow(index, "title", event.target.value)}
                    placeholder="Card title"
                  />
                  <Input
                    aria-label={`Card ${index + 1} description`}
                    name="item_description"
                    value={row.description}
                    onChange={(event) => updateRow(index, "description", event.target.value)}
                    placeholder="Short description"
                  />
                  <div className="flex items-center gap-2">
                    <Input
                      aria-label={`Card ${index + 1} icon`}
                      name="item_icon"
                      value={row.icon}
                      onChange={(event) => updateRow(index, "icon", event.target.value)}
                      placeholder="Icon (mail, phone, map)"
                      className="w-36"
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      className="h-8 px-2 text-destructive hover:bg-destructive/10"
                      onClick={() =>
                        setRows((current) => current.filter((_, rowIndex) => rowIndex !== index))
                      }
                    >
                      <Trash2 className="size-4" />
                      <span className="sr-only">Remove card {index + 1}</span>
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
          <Button
            type="button"
            variant="secondary"
            onClick={() =>
              setRows((current) => [...current, { title: "", description: "", icon: "" }])
            }
          >
            <Plus className="mr-2 size-4" />
            Add card
          </Button>
        </fieldset>
      ) : null}

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Section saved." : "Section added."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Adding"}>
          {isEdit ? "Save changes" : "Add section"}
        </Button>
      </div>
    </form>
  );
}
