"use client";

import * as React from "react";
import { Check, KeyRound, Palette, ShieldCheck, UserRound } from "lucide-react";
import {
  requestPlatformPasswordResetAction,
  updatePlatformProfileAction,
  type AccountActionResult,
} from "@/lib/superadmin-account-actions";

const initialState: AccountActionResult = {};
const designs = [
  {
    id: "aura",
    name: "Aura",
    description:
      "Balanced depth, crisp surfaces, and the original education command-centre language.",
  },
  {
    id: "neumorphic",
    name: "Neumorphism",
    description: "Soft extruded controls and low-contrast dimensional surfaces.",
  },
  {
    id: "clay",
    name: "Clay",
    description: "Rounded, tactile forms with stronger colour and friendly depth.",
  },
  {
    id: "liquid-glass",
    name: "Liquid glass",
    description: "Translucent layers, luminous borders, and fluid glass-like depth.",
  },
] as const;

type DesignId = (typeof designs)[number]["id"];

export function PlatformAccountSettings({
  user,
}: {
  user: { id: string; name: string; email: string; role: string };
}) {
  const [profileState, profileAction, profilePending] = React.useActionState(
    updatePlatformProfileAction,
    initialState,
  );
  const [passwordState, passwordAction, passwordPending] = React.useActionState(
    requestPlatformPasswordResetAction,
    initialState,
  );
  const [design, setDesign] = React.useState<DesignId>("aura");

  React.useEffect(() => {
    const stored = localStorage.getItem("auraedu-design-system") as DesignId | null;
    if (stored && designs.some((option) => option.id === stored)) setDesign(stored);
  }, []);

  function chooseDesign(value: DesignId) {
    setDesign(value);
    localStorage.setItem("auraedu-design-system", value);
    document.documentElement.dataset.designSystem = value;
  }

  return (
    <div className="space-y-6">
      <section className="settings-panel grid gap-6 p-5 lg:grid-cols-[minmax(0,0.7fr)_minmax(0,1.3fr)]">
        <div>
          <div className="settings-icon">
            <UserRound className="size-5" />
          </div>
          <h2 className="mt-4 text-xl font-bold">Profile</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            The name used in the console and attributed to platform audit activity.
          </p>
          <dl className="mt-5 space-y-3 text-sm">
            <div>
              <dt className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                Scope
              </dt>
              <dd className="mt-1 font-semibold">AuraEDU platform</dd>
            </div>
            <div>
              <dt className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                Role
              </dt>
              <dd className="mt-1 font-semibold capitalize">{user.role.replaceAll("_", " ")}</dd>
            </div>
          </dl>
        </div>
        <form action={profileAction} className="space-y-4">
          <input type="hidden" name="user_id" value={user.id} />
          <label className="block text-sm font-semibold">
            Display name
            <input
              className="journey-input"
              name="name"
              defaultValue={user.name}
              minLength={2}
              required
            />
          </label>
          <label className="block text-sm font-semibold">
            Sign-in email
            <input className="journey-input opacity-70" value={user.email} disabled />
            <span className="mt-1 block text-xs font-normal text-muted-foreground">
              Email changes require another platform administrator.
            </span>
          </label>
          <ActionMessage state={profileState} />
          <button className="settings-primary-button" disabled={profilePending}>
            {profilePending ? "Saving…" : "Save profile"}
          </button>
        </form>
      </section>

      <section className="settings-panel p-5">
        <div className="flex items-start gap-3">
          <div className="settings-icon">
            <Palette className="size-5" />
          </div>
          <div>
            <h2 className="text-xl font-bold">Design system</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Choose the surface language you prefer. Navigation, accessibility, and information
              architecture stay consistent.
            </p>
          </div>
        </div>
        <div className="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {designs.map((option) => (
            <button
              key={option.id}
              type="button"
              onClick={() => chooseDesign(option.id)}
              aria-pressed={design === option.id}
              className={`design-choice design-choice--${option.id} ${design === option.id ? "is-selected" : ""}`}
            >
              <span className="design-choice-preview" aria-hidden="true">
                <span />
                <span />
                <span />
              </span>
              <span className="flex items-center justify-between gap-2 font-bold">
                {option.name}
                {design === option.id ? <Check className="size-4" /> : null}
              </span>
              <span className="mt-1 block text-left text-xs leading-relaxed text-muted-foreground">
                {option.description}
              </span>
            </button>
          ))}
        </div>
      </section>

      <div className="grid gap-6 lg:grid-cols-2">
        <section className="settings-panel p-5">
          <div className="settings-icon">
            <KeyRound className="size-5" />
          </div>
          <h2 className="mt-4 text-xl font-bold">Password</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Request a one-time, expiring reset link through the existing secure recovery flow.
          </p>
          <form action={passwordAction} className="mt-5">
            <input type="hidden" name="email" value={user.email} />
            <ActionMessage state={passwordState} />
            <button className="settings-primary-button mt-3" disabled={passwordPending}>
              {passwordPending ? "Requesting…" : "Request password reset"}
            </button>
          </form>
        </section>

        <section className="settings-panel p-5">
          <div className="settings-icon">
            <ShieldCheck className="size-5" />
          </div>
          <h2 className="mt-4 text-xl font-bold">Multi-factor authentication</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            MFA is mandatory for platform administrators in production. AuraEDU prompts for
            authenticator setup during the first privileged production sign-in and verifies every
            later sign-in.
          </p>
          <div className="mt-5 inline-flex items-center gap-2 rounded-full bg-emerald-500/10 px-3 py-1.5 text-sm font-bold text-emerald-700 dark:text-emerald-300">
            <ShieldCheck className="size-4" />
            Production-enforced role
          </div>
        </section>
      </div>
    </div>
  );
}

function ActionMessage({ state }: { state: AccountActionResult }) {
  if (state.error)
    return (
      <p role="alert" className="text-sm font-medium text-red-600">
        {state.error}
      </p>
    );
  if (state.success)
    return (
      <p role="status" className="text-sm font-medium text-emerald-700 dark:text-emerald-300">
        {state.success}
      </p>
    );
  return null;
}
