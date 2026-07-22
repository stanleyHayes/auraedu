"use client";

import * as React from "react";
import { CheckCircle2, Copy, Globe2, ShieldCheck } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label } from "@auraedu/ui";
import {
  activateCustomDomainAction,
  deactivateCustomDomainAction,
  requestCustomDomainAction,
  verifyCustomDomainAction,
  type DomainActionResult,
} from "@/lib/tenant-actions";

type CustomDomain = OpenAPI.tenant_v1.components["schemas"]["CustomDomain"];

export function CustomDomainCard({
  tenantCode,
  initial,
  platformAdmin = false,
}: {
  tenantCode: string;
  initial?: CustomDomain;
  platformAdmin?: boolean;
}) {
  const [domain, setDomain] = React.useState<CustomDomain | undefined>(initial);
  const [copied, setCopied] = React.useState(false);
  const [requestState, requestAction, requesting] = React.useActionState<
    DomainActionResult,
    FormData
  >(requestCustomDomainAction.bind(null, tenantCode), {});
  const [verifyState, verifyAction, verifying] = React.useActionState<DomainActionResult, FormData>(
    verifyCustomDomainAction.bind(null, tenantCode),
    {},
  );
  const [activateState, activateAction, activating] = React.useActionState<
    DomainActionResult,
    FormData
  >(activateCustomDomainAction.bind(null, tenantCode), {});
  const [deactivateState, deactivateAction, deactivating] = React.useActionState<
    DomainActionResult,
    FormData
  >(deactivateCustomDomainAction.bind(null, tenantCode), {});

  React.useEffect(() => {
    const latest =
      deactivateState.domain ?? activateState.domain ?? verifyState.domain ?? requestState.domain;
    if (latest) setDomain(latest);
  }, [activateState.domain, deactivateState.domain, requestState.domain, verifyState.domain]);

  const challenge = requestState.domain?.verification_token;
  const message =
    deactivateState.error ?? activateState.error ?? verifyState.error ?? requestState.error;

  return (
    <section className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)]">
      <div className="flex flex-col gap-4 border-b border-[var(--border)] bg-[radial-gradient(circle_at_top_right,color-mix(in_oklch,var(--primary)_12%,transparent),transparent_52%)] p-6 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex gap-3">
          <span className="grid size-11 shrink-0 place-items-center rounded-2xl bg-[var(--primary)] text-white shadow-[0_14px_30px_-18px_var(--primary)]">
            <Globe2 className="size-5" />
          </span>
          <div>
            <h2 className="font-heading text-lg font-bold">Verified school domain</h2>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Prove DNS ownership first. AuraEDU activates traffic only after the hosting provider
              reports valid TLS.
            </p>
          </div>
        </div>
        <span className="w-fit rounded-full border border-[var(--border)] bg-[var(--surface)] px-3 py-1 text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
          {domain?.status?.replace("_", " ") ?? "Not configured"}
        </span>
      </div>

      <div className="grid gap-6 p-6 lg:grid-cols-[minmax(0,1fr)_minmax(18rem,0.72fr)]">
        <form action={requestAction} className="space-y-3">
          <Label htmlFor={`hostname-${tenantCode}`}>School-owned hostname</Label>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Input
              id={`hostname-${tenantCode}`}
              name="hostname"
              defaultValue={domain?.hostname ?? ""}
              placeholder="www.school.edu.gh"
              required
            />
            <Button type="submit" loading={requesting} loadingLabel="Creating challenge">
              {domain ? "Replace challenge" : "Start verification"}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Platform-owned <code>*.auraedu.com</code> addresses do not use this workflow.
          </p>
        </form>

        <div className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--background)] p-4">
          {domain ? (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm font-semibold">
                <ShieldCheck className="size-4 text-[var(--primary)]" />
                DNS ownership record
              </div>
              <dl className="space-y-2 text-sm">
                <div>
                  <dt className="text-xs uppercase tracking-wide text-muted-foreground">
                    TXT name
                  </dt>
                  <dd className="break-all font-mono">{domain.txt_record_name}</dd>
                </div>
                {challenge ? (
                  <div>
                    <dt className="text-xs uppercase tracking-wide text-muted-foreground">
                      TXT value — shown once
                    </dt>
                    <dd className="mt-1 flex items-start gap-2">
                      <code className="min-w-0 flex-1 break-all rounded bg-[var(--surface)] p-2">
                        {challenge}
                      </code>
                      <button
                        type="button"
                        className="rounded-lg border border-[var(--border)] p-2"
                        aria-label="Copy TXT value"
                        onClick={() => {
                          void navigator.clipboard.writeText(challenge).then(() => setCopied(true));
                        }}
                      >
                        <Copy className="size-4" />
                      </button>
                    </dd>
                    {copied ? <p className="mt-1 text-xs text-emerald-600">Copied.</p> : null}
                  </div>
                ) : null}
              </dl>
              {domain.status === "pending_dns" ? (
                <form action={verifyAction}>
                  <Button
                    type="submit"
                    variant="secondary"
                    loading={verifying}
                    loadingLabel="Checking DNS"
                  >
                    Check DNS now
                  </Button>
                </form>
              ) : (
                <p className="flex items-center gap-2 text-sm font-medium text-emerald-600">
                  <CheckCircle2 className="size-4" />
                  Ownership verified
                </p>
              )}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              Create a challenge to receive the exact DNS record.
            </p>
          )}
        </div>
      </div>

      {platformAdmin && domain?.status === "verified" ? (
        <form
          action={activateAction}
          className="grid gap-3 border-t border-[var(--border)] bg-[var(--background)] p-6 sm:grid-cols-[1fr_auto] sm:items-end"
        >
          <div className="space-y-2">
            <Label htmlFor={`provider-${tenantCode}`}>Provider TLS reference</Label>
            <Input
              id={`provider-${tenantCode}`}
              name="provider_reference"
              placeholder="Render custom-domain ID or certificate reference"
              required
              minLength={8}
            />
            <p className="text-xs text-muted-foreground">
              Activate only after the provider shows certificate issuance and HTTPS readiness.
            </p>
          </div>
          <Button type="submit" loading={activating} loadingLabel="Activating">
            Activate domain
          </Button>
        </form>
      ) : null}
      {domain?.status === "active" ? (
        <div className="border-t border-emerald-500/20 bg-emerald-500/8 px-6 py-4">
          <p className="text-sm font-medium text-emerald-700">
            HTTPS traffic is active for {domain.hostname}.
          </p>
          {platformAdmin ? (
            <form
              action={deactivateAction}
              className="mt-4 grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end"
            >
              <div className="space-y-2">
                <Label htmlFor={`provider-remove-${tenantCode}`}>Provider removal reference</Label>
                <Input
                  id={`provider-remove-${tenantCode}`}
                  name="provider_reference"
                  placeholder="Render removal, incident, or certificate reference"
                  required
                  minLength={8}
                />
              </div>
              <Button
                type="submit"
                variant="secondary"
                loading={deactivating}
                loadingLabel="Deactivating"
              >
                Deactivate safely
              </Button>
            </form>
          ) : null}
        </div>
      ) : null}
      {message ? (
        <p
          role="alert"
          className="border-t border-destructive/20 bg-destructive/8 px-6 py-3 text-sm text-destructive"
        >
          {message}
        </p>
      ) : null}
    </section>
  );
}
