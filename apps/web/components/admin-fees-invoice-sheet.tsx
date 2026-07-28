"use client";

import * as React from "react";
import { ReceiptText } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import {
  AdminFeesInvoiceForm,
  type InvoiceClassOption,
  type InvoiceStudentOption,
} from "./admin-fees-invoice-form";

type FeeStructure = OpenAPI.fees_v1.components["schemas"]["FeeStructure"];

interface AdminFeesInvoiceSheetProps {
  structures: FeeStructure[];
  students: InvoiceStudentOption[];
  classes: InvoiceClassOption[];
}

export function AdminFeesInvoiceSheet({
  structures,
  students,
  classes,
}: AdminFeesInvoiceSheetProps) {
  const [open, setOpen] = React.useState(false);
  const billable = structures.some((structure) => structure.status === "active");

  return (
    <>
      <Button
        type="button"
        variant="secondary"
        onClick={() => setOpen(true)}
        disabled={!billable || students.length === 0}
      >
        <ReceiptText className="mr-2 size-4" />
        Issue invoice
      </Button>
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-2xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">Issue invoice</h2>
            <p className="text-sm text-muted-foreground">
              Bill an active fee structure to one student or a whole class, with an optional due
              date and amount override.
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminFeesInvoiceForm
              structures={structures}
              students={students}
              classes={classes}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
