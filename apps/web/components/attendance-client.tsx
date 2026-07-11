"use client";

import * as React from "react";
import { Button, PageHeader, RegisterCard, StatCard, type Pupil } from "@auraedu/ui";

const CLASS_SIZE = 36;

const initialRoster: Pupil[] = [
  { id: "1", name: "Ama Owusu", present: true },
  { id: "2", name: "Kwame Mensah", present: true },
  { id: "3", name: "Efua Sarpong", present: true },
  { id: "4", name: "Yaw Boateng", present: true },
  { id: "5", name: "Adjoa Nyarko", present: true },
  { id: "6", name: "Kojo Amaning", present: false },
  { id: "7", name: "Abena Darko", present: true },
  { id: "8", name: "Kofi Asante", present: true },
  { id: "9", name: "Akua Frimpong", present: true },
  { id: "10", name: "Nana Addo", present: false },
  { id: "11", name: "Esi Bonsu", present: true },
  { id: "12", name: "Yaa Asantewaa", present: true },
];

const clipboardIcon = (
  <svg
    width="22"
    height="22"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth={2}
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    <path d="M9 11l3 3L22 4" />
    <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
  </svg>
);

export function AttendanceClient() {
  const [roster, setRoster] = React.useState<Pupil[]>(initialRoster);
  const [publishing, setPublishing] = React.useState(false);
  const [toast, setToast] = React.useState<string | null>(null);

  const absent = roster.filter((p) => !p.present).length;
  const present = CLASS_SIZE - absent;

  const toggle = React.useCallback((id: string) => {
    setRoster((rs) => rs.map((p) => (p.id === id ? { ...p, present: !p.present } : p)));
  }, []);

  const publish = React.useCallback(() => {
    setPublishing(true);
    window.setTimeout(() => {
      setPublishing(false);
      setToast(`Published — ${present} present, ${absent} absent · parents notified`);
      window.setTimeout(() => setToast(null), 2800);
    }, 1100);
  }, [present, absent]);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        icon={clipboardIcon}
        title="Take the register"
        description="Mark today's attendance for Form 2 Science, then publish it to parents."
        action={
          <Button loading={publishing} loadingLabel="Publishing" onClick={publish}>
            Publish to parents
          </Button>
        }
      />

      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard label="Present now" value={present} unit={`/ ${CLASS_SIZE}`} />
        <StatCard label="Term average" value="96.4" unit="%" />
        <StatCard label="Flagged" value={absent} unit="absences" tone="warn" />
      </div>

      <RegisterCard
        title="Form 2 Science · Register"
        meta="Mon 10 Jul · 08:05"
        pupils={roster}
        total={CLASS_SIZE}
        onToggle={toggle}
      />

      {toast ? (
        <div
          role="status"
          className="fixed bottom-6 left-1/2 z-50 -translate-x-1/2 rounded-[var(--radius-md)] bg-[var(--color-ink-950)] px-4 py-3 text-sm text-[var(--color-paper-50)] shadow-lg"
        >
          {toast}
        </div>
      ) : null}
    </div>
  );
}
