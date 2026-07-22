"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { Bot, ClipboardList, Mail, MessageCircle, PhoneCall, Send, X } from "lucide-react";
import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import type { Programme } from "@/lib/programmes";

interface Citation {
  source_id: string;
  title: string;
  version: number;
}

interface AssistantResponse {
  session_id: string;
  message_id: string;
  answer: string;
  confidence: number;
  citations: Citation[];
  needs_human: boolean;
  escalation_message: string | null;
  locale: AssistantLocale;
}

interface Message {
  id: string;
  role: "visitor" | "assistant";
  text: string;
  citations?: Citation[];
  needsHuman?: boolean;
}

const assistantId = "admissions-assistant-panel";
type AssistantLocale = "en-GH" | "fr-GH";

const assistantCopy = {
  "en-GH": {
    language: "Language",
    title: "Ask admissions",
    sourceNote: "Answers use approved sources from",
    source: "Source",
    intro:
      "Ask about programmes, entry requirements, fees, deadlines, campus life, or accommodation. If an approved source does not answer it, I'll help you reach a person.",
    suggestions: [
      "Which programmes are open?",
      "What are the entry requirements?",
      "When is the application deadline?",
    ],
    close: "Close admissions assistant",
    open: "Open admissions assistant",
    send: "Send question",
    placeholder: "Ask a question…",
    checking: "Checking approved sources…",
    rateLimited: "Too many questions. Please wait a moment and try again.",
    unavailable: "The admissions assistant is temporarily unavailable.",
    contact: "Ask admissions to contact me",
    emailContact: "Email me",
    callbackContact: "Request a call time",
    followUpTitle: "Request a human follow-up",
    followUpDescription: "Your request will enter the school's admissions workspace.",
    name: "Name",
    email: "Email",
    consent: "may email me about this admissions enquiry.",
    sendFollowUp: "Send follow-up request",
    sending: "Sending…",
    leadError: "We could not send your request. Please try again.",
    leadSuccess:
      "Your request is with the admissions team. They can now follow up using the email you provided.",
    followUpMessage: "Requested an admissions follow-up from the website assistant.",
    callbackTitle: "Request a preferred call time",
    callbackDescription:
      "Choose a convenient time. This is a request—the admissions team will confirm the appointment separately.",
    phone: "Phone number",
    preferredTime: "Preferred date and time",
    callbackConsent: "may call me about this admissions enquiry.",
    sendCallback: "Request this call time",
    callbackSuccess:
      "Your preferred call time has been requested. The admissions team will confirm the appointment separately.",
    callbackError: "We could not request that call time. Please check the details and try again.",
    callbackMessage: "Requested an admissions callback from the website assistant.",
    applicationAction: "Continue an application",
    applicationDescription:
      "Choose an intake from the school's verified catalogue, then sign in with your applicant account.",
    browseProgrammes: "Browse every open programme",
    privacy: "Do not share passwords, payment details, or identity documents here.",
  },
  "fr-GH": {
    language: "Langue",
    title: "Demander aux admissions",
    sourceNote: "Les réponses utilisent les sources approuvées de",
    source: "Source",
    intro:
      "Posez vos questions sur les programmes, les conditions d'admission, les frais, les délais, la vie sur le campus ou le logement. Si aucune source approuvée ne répond, je vous aiderai à contacter une personne.",
    suggestions: [
      "Quels programmes sont ouverts ?",
      "Quelles sont les conditions d'admission ?",
      "Quelle est la date limite de candidature ?",
    ],
    close: "Fermer l'assistant d'admission",
    open: "Ouvrir l'assistant d'admission",
    send: "Envoyer la question",
    placeholder: "Posez une question…",
    checking: "Recherche dans les sources approuvées…",
    rateLimited: "Trop de questions. Veuillez patienter avant de réessayer.",
    unavailable: "L'assistant d'admission est temporairement indisponible.",
    contact: "Demander à l'équipe de me contacter",
    emailContact: "M'envoyer un e-mail",
    callbackContact: "Demander un horaire d'appel",
    followUpTitle: "Demander un suivi humain",
    followUpDescription:
      "Votre demande sera transmise à l'espace de travail de l'équipe des admissions.",
    name: "Nom",
    email: "E-mail",
    consent: "peut m'envoyer un e-mail concernant cette demande d'admission.",
    sendFollowUp: "Envoyer la demande",
    sending: "Envoi…",
    leadError: "Nous n'avons pas pu envoyer votre demande. Veuillez réessayer.",
    leadSuccess:
      "Votre demande a été transmise à l'équipe des admissions. Elle peut maintenant vous répondre à l'adresse fournie.",
    followUpMessage: "Demande de suivi auprès des admissions depuis l'assistant du site web.",
    callbackTitle: "Demander un horaire d'appel préféré",
    callbackDescription:
      "Choisissez un horaire qui vous convient. Il s'agit d'une demande : l'équipe des admissions confirmera le rendez-vous séparément.",
    phone: "Numéro de téléphone",
    preferredTime: "Date et heure souhaitées",
    callbackConsent: "peut m'appeler au sujet de cette demande d'admission.",
    sendCallback: "Demander cet horaire",
    callbackSuccess:
      "Votre horaire d'appel préféré a été demandé. L'équipe des admissions confirmera le rendez-vous séparément.",
    callbackError:
      "Nous n'avons pas pu demander cet horaire. Vérifiez les informations et réessayez.",
    callbackMessage: "Demande de rappel des admissions depuis l'assistant du site web.",
    applicationAction: "Continuer une candidature",
    applicationDescription:
      "Choisissez une session dans le catalogue vérifié de l'établissement, puis connectez-vous avec votre compte candidat.",
    browseProgrammes: "Voir tous les programmes ouverts",
    privacy:
      "Ne communiquez pas de mots de passe, de données de paiement ou de pièces d'identité ici.",
  },
} as const;

function formValue(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value.trim() : "";
}

export function AdmissionsAssistant({
  tenantCode,
  schoolName,
}: {
  tenantCode: string;
  schoolName: string;
}) {
  const [open, setOpen] = useState(false);
  const [locale, setLocale] = useState<AssistantLocale>("en-GH");
  const [question, setQuestion] = useState("");
  const [sessionId, setSessionId] = useState<string>();
  const [messages, setMessages] = useState<Message[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [leadFormOpen, setLeadFormOpen] = useState(false);
  const [leadSubmitted, setLeadSubmitted] = useState(false);
  const [leadBusy, setLeadBusy] = useState(false);
  const [leadError, setLeadError] = useState<string>();
  const [callbackFormOpen, setCallbackFormOpen] = useState(false);
  const [callbackSubmitted, setCallbackSubmitted] = useState(false);
  const [callbackBusy, setCallbackBusy] = useState(false);
  const [callbackError, setCallbackError] = useState<string>();
  const [programmes, setProgrammes] = useState<Programme[]>([]);
  const [timezone, setTimezone] = useState("Africa/Accra");
  const [callbackMin, setCallbackMin] = useState("");
  const [callbackMax, setCallbackMax] = useState("");
  const callbackKeyRef = useRef<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const launcherRef = useRef<HTMLButtonElement>(null);
  const transcriptRef = useRef<HTMLDivElement>(null);
  const copy = assistantCopy[locale];

  function changeLocale(nextLocale: AssistantLocale) {
    if (nextLocale === locale) return;
    setLocale(nextLocale);
    setSessionId(undefined);
    setMessages([]);
    setQuestion("");
    setError(undefined);
    setLeadFormOpen(false);
    setLeadSubmitted(false);
    setCallbackFormOpen(false);
    setCallbackSubmitted(false);
    setCallbackError(undefined);
    callbackKeyRef.current = null;
  }

  function openAssistant() {
    setOpen(true);
    requestAnimationFrame(() => inputRef.current?.focus());
  }

  function closeAssistant() {
    setOpen(false);
    requestAnimationFrame(() => launcherRef.current?.focus());
  }

  useEffect(() => {
    if (!open) return;
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
        requestAnimationFrame(() => launcherRef.current?.focus());
      }
    }
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [open]);

  useEffect(() => {
    const detected = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (detected) setTimezone(detected);
    const earliest = new Date(Date.now() + 15 * 60 * 1000);
    earliest.setSeconds(0, 0);
    const offset = earliest.getTimezoneOffset() * 60_000;
    setCallbackMin(new Date(earliest.getTime() - offset).toISOString().slice(0, 16));
    const latest = new Date(Date.now() + 90 * 24 * 60 * 60 * 1000);
    latest.setSeconds(0, 0);
    const latestOffset = latest.getTimezoneOffset() * 60_000;
    setCallbackMax(new Date(latest.getTime() - latestOffset).toISOString().slice(0, 16));
  }, []);

  useEffect(() => {
    if (!open || programmes.length > 0) return;
    const controller = new AbortController();
    void fetch(`${publicApiUrl}/api/v1/public/programmes?limit=20`, {
      headers: { [tenantHeaderName]: tenantCode },
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) return;
        const result = (await response.json()) as { data?: Programme[] };
        setProgrammes(result.data ?? []);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [open, programmes.length, tenantCode]);

  useEffect(() => {
    if (!open || !transcriptRef.current) return;
    const frame = requestAnimationFrame(() => {
      const transcript = transcriptRef.current;
      if (!transcript) return;
      const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      transcript.scrollTo({
        top: transcript.scrollHeight,
        behavior: reducedMotion ? "auto" : "smooth",
      });
    });
    return () => cancelAnimationFrame(frame);
  }, [
    callbackFormOpen,
    callbackSubmitted,
    error,
    leadFormOpen,
    leadSubmitted,
    messages,
    open,
    programmes.length,
  ]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    const cleanQuestion = question.trim();
    if (cleanQuestion.length < 2 || busy) return;

    const visitorMessage: Message = {
      id: crypto.randomUUID(),
      role: "visitor",
      text: cleanQuestion,
    };
    setMessages((current) => [...current, visitorMessage]);
    setQuestion("");
    setError(undefined);
    setBusy(true);
    try {
      const response = await fetch(`${publicApiUrl}/api/v1/public/assistant/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
          [tenantHeaderName]: tenantCode,
        },
        body: JSON.stringify({ question: cleanQuestion, session_id: sessionId ?? null, locale }),
      });
      if (!response.ok) {
        throw new Error(response.status === 429 ? copy.rateLimited : copy.unavailable);
      }
      const result = (await response.json()) as AssistantResponse;
      if (result.locale !== locale) throw new Error(copy.unavailable);
      setSessionId(result.session_id);
      setMessages((current) => [
        ...current,
        {
          id: result.message_id,
          role: "assistant",
          text: result.answer,
          citations: result.citations,
          needsHuman: result.needs_human,
        },
      ]);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : copy.unavailable);
    } finally {
      setBusy(false);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }

  async function submitLead(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (leadBusy) return;
    const data = new FormData(event.currentTarget);
    const fullName = formValue(data, "name").split(/\s+/);
    const firstName = fullName.shift() ?? "";
    const lastName = fullName.join(" ") || "Prospect";
    const email = formValue(data, "email");
    if (!firstName || !email || data.get("consent") !== "on") return;
    setLeadBusy(true);
    setLeadError(undefined);
    try {
      const latestQuestion = [...messages]
        .reverse()
        .find((message) => message.role === "visitor")?.text;
      const response = await fetch(`${publicApiUrl}/api/v1/public/leads`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
          [tenantHeaderName]: tenantCode,
        },
        body: JSON.stringify({
          first_name: firstName,
          last_name: lastName,
          email,
          phone: null,
          preferred_programme_ids: [],
          preferred_intake_id: null,
          source: "website_assistant",
          campaign_id: null,
          message: latestQuestion ?? copy.followUpMessage,
          consent: {
            privacy_notice_version: "2026-07-18",
            email: true,
            sms: false,
            whatsapp: false,
            voice: false,
          },
        }),
      });
      if (!response.ok) throw new Error(copy.leadError);
      setLeadSubmitted(true);
    } catch (cause) {
      setLeadError(cause instanceof Error ? cause.message : copy.leadError);
    } finally {
      setLeadBusy(false);
    }
  }

  async function submitCallback(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (callbackBusy) return;
    const data = new FormData(event.currentTarget);
    const fullName = formValue(data, "name").split(/\s+/);
    const firstName = fullName.shift() ?? "";
    const lastName = fullName.join(" ") || "Prospect";
    const phone = formValue(data, "phone");
    const email = formValue(data, "email");
    const preferredValue = formValue(data, "preferred_at");
    const preferredAt = new Date(preferredValue);
    if (
      !firstName ||
      !phone ||
      !preferredValue ||
      Number.isNaN(preferredAt.getTime()) ||
      data.get("voice_consent") !== "on"
    )
      return;

    setCallbackBusy(true);
    setCallbackError(undefined);
    callbackKeyRef.current ??= crypto.randomUUID();
    try {
      const latestQuestion = [...messages]
        .reverse()
        .find((message) => message.role === "visitor")?.text;
      const response = await fetch(`${publicApiUrl}/api/v1/public/callback-requests`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": callbackKeyRef.current,
          [tenantHeaderName]: tenantCode,
        },
        body: JSON.stringify({
          first_name: firstName,
          last_name: lastName,
          email: email || null,
          phone,
          preferred_at: preferredAt.toISOString(),
          timezone,
          locale,
          message: latestQuestion ?? copy.callbackMessage,
          consent: {
            privacy_notice_version: "2026-07-19",
            email: false,
            sms: false,
            whatsapp: false,
            voice: true,
          },
        }),
      });
      if (!response.ok) throw new Error(copy.callbackError);
      setCallbackSubmitted(true);
    } catch (cause) {
      setCallbackError(cause instanceof Error ? cause.message : copy.callbackError);
    } finally {
      setCallbackBusy(false);
    }
  }

  return (
    <div className="fixed bottom-5 right-5 z-50 sm:bottom-7 sm:right-7">
      <section
        id={assistantId}
        role="dialog"
        lang={locale.startsWith("fr") ? "fr" : "en"}
        aria-label={`${schoolName} admissions assistant`}
        aria-hidden={!open}
        inert={!open}
        className={`fixed inset-x-3 bottom-3 flex h-[calc(100dvh-1.5rem)] w-auto origin-bottom-right flex-col overflow-hidden rounded-[1.5rem] border border-[var(--border)] bg-[var(--background)] shadow-2xl transition duration-300 sm:absolute sm:inset-x-auto sm:bottom-16 sm:right-0 sm:h-[min(36rem,calc(100vh-7rem))] sm:w-[min(25rem,calc(100vw-2.5rem))] sm:rounded-2xl motion-reduce:transition-none ${open ? "pointer-events-auto translate-y-0 scale-100 opacity-100" : "pointer-events-none translate-y-6 scale-[0.98] opacity-0 sm:translate-y-3 sm:scale-95"}`}
      >
        <header className="flex shrink-0 items-start justify-between gap-4 bg-[var(--primary)] px-5 py-4 text-[var(--primary-foreground)]">
          <div className="flex gap-3">
            <span className="grid size-10 shrink-0 place-items-center rounded-full bg-white/15">
              <Bot aria-hidden="true" size={20} />
            </span>
            <div className="min-w-0">
              <h2 className="font-sans font-bold">{copy.title}</h2>
              <p className="mt-0.5 flex items-center gap-1.5 text-xs text-white/80">
                <span
                  className="size-1.5 shrink-0 animate-pulse rounded-full bg-emerald-300 motion-reduce:animate-none"
                  aria-hidden="true"
                />
                {copy.sourceNote} {schoolName}.
              </p>
              <label className="mt-2 inline-flex items-center gap-2 text-[11px] font-semibold text-white/90">
                <span>{copy.language}</span>
                <select
                  value={locale}
                  onChange={(event) => changeLocale(event.target.value as AssistantLocale)}
                  className="rounded-md border border-white/25 bg-white/10 px-2 py-1 text-xs text-white outline-none focus:ring-2 focus:ring-white/70"
                >
                  <option value="en-GH" className="text-slate-950">
                    English
                  </option>
                  <option value="fr-GH" className="text-slate-950">
                    Français
                  </option>
                </select>
              </label>
            </div>
          </div>
          <button
            type="button"
            onClick={closeAssistant}
            className="grid size-10 shrink-0 place-items-center rounded-full transition-colors hover:bg-white/15 motion-reduce:transition-none"
            aria-label={copy.close}
          >
            <X size={18} />
          </button>
        </header>

        <div
          ref={transcriptRef}
          className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-5"
          aria-live="polite"
        >
          {messages.length === 0 ? (
            <div className="space-y-4 rounded-xl bg-[var(--muted)] p-4 text-sm text-[var(--muted-foreground)]">
              <p>{copy.intro}</p>
              <div className="flex flex-wrap gap-2" aria-label={copy.title}>
                {copy.suggestions.map((suggestion) => (
                  <button
                    key={suggestion}
                    type="button"
                    onClick={() => {
                      setQuestion(suggestion);
                      requestAnimationFrame(() => inputRef.current?.focus());
                    }}
                    className="rounded-full border border-[var(--border)] bg-[var(--background)] px-3 py-1.5 text-left text-xs font-semibold text-[var(--foreground)] transition-colors hover:border-[var(--primary)] hover:bg-[var(--accent)] hover:text-[var(--foreground)] motion-reduce:transition-none"
                  >
                    {suggestion}
                  </button>
                ))}
              </div>
            </div>
          ) : null}
          {messages.map((message) => (
            <div key={message.id} className={message.role === "visitor" ? "ml-8" : "mr-5"}>
              <div
                className={`rounded-2xl px-4 py-3 text-sm leading-6 ${message.role === "visitor" ? "rounded-br-sm bg-[var(--primary)] text-[var(--primary-foreground)]" : "rounded-bl-sm bg-[var(--muted)] text-[var(--foreground)]"}`}
              >
                {message.text}
              </div>
              {message.citations && message.citations.length > 0 ? (
                <div className="mt-2 px-2 text-xs text-[var(--muted-foreground)]">
                  {copy.source}:{" "}
                  {message.citations
                    .map((citation) => `${citation.title} (v${citation.version})`)
                    .join(", ")}
                </div>
              ) : null}
              {message.needsHuman && !leadSubmitted && !callbackSubmitted ? (
                <div className="mt-2 flex flex-wrap gap-2" aria-label={copy.contact}>
                  <button
                    type="button"
                    onClick={() => {
                      setLeadFormOpen(true);
                      setCallbackFormOpen(false);
                    }}
                    className="inline-flex min-h-9 items-center gap-1.5 rounded-full border border-[var(--border)] bg-[var(--surface)] px-3 text-xs font-semibold text-[var(--foreground)] transition-colors hover:bg-[var(--muted)] motion-reduce:transition-none"
                  >
                    <Mail size={13} aria-hidden="true" /> {copy.emailContact}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setCallbackFormOpen(true);
                      setLeadFormOpen(false);
                    }}
                    className="inline-flex items-center gap-1.5 rounded-full bg-[var(--primary)] px-3 py-1.5 text-xs font-semibold text-[var(--primary-foreground)]"
                  >
                    <PhoneCall size={13} aria-hidden="true" /> {copy.callbackContact}
                  </button>
                </div>
              ) : null}
            </div>
          ))}
          {messages.some((message) => message.role === "assistant") &&
          !leadFormOpen &&
          !callbackFormOpen &&
          !leadSubmitted &&
          !callbackSubmitted ? (
            <aside className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
              <div className="flex items-start gap-3">
                <span className="grid size-9 shrink-0 place-items-center rounded-full bg-[var(--muted)] text-[var(--primary)]">
                  <ClipboardList size={17} aria-hidden="true" />
                </span>
                <div>
                  <h3 className="text-sm font-bold">{copy.applicationAction}</h3>
                  <p className="mt-1 text-xs leading-5 text-[var(--muted-foreground)]">
                    {copy.applicationDescription}
                  </p>
                </div>
              </div>
              {programmes.slice(0, 3).flatMap((programme) =>
                programme.intakes.slice(0, 1).map((intake) => (
                  <a
                    key={intake.id}
                    href={`/applicant?programme=${programme.id}&intake=${intake.id}`}
                    className="mt-3 block rounded-lg border border-[var(--border)] p-3 hover:bg-[var(--muted)]"
                  >
                    <span className="block text-sm font-semibold">{programme.name}</span>
                    <span className="mt-1 block text-xs text-[var(--muted-foreground)]">
                      {intake.name} ·{" "}
                      {new Date(intake.application_closes_at).toLocaleDateString(locale)}
                    </span>
                  </a>
                )),
              )}
              <a
                href={`/${tenantCode}/programmes`}
                className="mt-3 inline-flex w-full items-center justify-center rounded-lg bg-[var(--primary)] px-3 py-2.5 text-sm font-semibold text-[var(--primary-foreground)] transition-transform hover:-translate-y-0.5 motion-reduce:transition-none"
              >
                {programmes.length > 0 ? copy.browseProgrammes : copy.applicationAction}
              </a>
            </aside>
          ) : null}
          {leadFormOpen && !leadSubmitted ? (
            <form
              onSubmit={(event) => {
                void submitLead(event);
              }}
              className="space-y-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4"
            >
              <div>
                <h3 className="text-sm font-bold">{copy.followUpTitle}</h3>
                <p className="mt-1 text-xs leading-5 text-[var(--muted-foreground)]">
                  {copy.followUpDescription}
                </p>
              </div>
              <label className="block text-xs font-semibold">
                {copy.name}
                <input
                  name="name"
                  required
                  autoComplete="name"
                  className="mt-1.5 h-10 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-normal"
                />
              </label>
              <label className="block text-xs font-semibold">
                {copy.email}
                <input
                  name="email"
                  type="email"
                  required
                  autoComplete="email"
                  className="mt-1.5 h-10 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-normal"
                />
              </label>
              <label className="flex items-start gap-2 text-[11px] leading-4 text-[var(--muted-foreground)]">
                <input
                  name="consent"
                  type="checkbox"
                  required
                  className="mt-0.5 size-4 accent-[var(--primary)]"
                />
                <span>
                  {locale.startsWith("fr") ? "J'accepte que" : "I agree that"} {schoolName}{" "}
                  {copy.consent}
                </span>
              </label>
              {leadError ? (
                <p role="alert" className="text-xs text-red-700">
                  {leadError}
                </p>
              ) : null}
              <button
                type="submit"
                disabled={leadBusy}
                className="w-full rounded-lg bg-[var(--primary)] px-3 py-2.5 text-sm font-semibold text-[var(--primary-foreground)] disabled:opacity-50"
              >
                {leadBusy ? copy.sending : copy.sendFollowUp}
              </button>
            </form>
          ) : null}
          {callbackFormOpen && !callbackSubmitted ? (
            <form
              onSubmit={(event) => {
                void submitCallback(event);
              }}
              className="space-y-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4"
            >
              <div>
                <h3 className="text-sm font-bold">{copy.callbackTitle}</h3>
                <p className="mt-1 text-xs leading-5 text-[var(--muted-foreground)]">
                  {copy.callbackDescription}
                </p>
              </div>
              <label className="block text-xs font-semibold">
                {copy.name}
                <input
                  name="name"
                  required
                  autoComplete="name"
                  className="mt-1.5 h-10 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-normal"
                />
              </label>
              <label className="block text-xs font-semibold">
                {copy.phone}
                <input
                  name="phone"
                  type="tel"
                  required
                  minLength={7}
                  maxLength={32}
                  autoComplete="tel"
                  className="mt-1.5 h-10 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-normal"
                />
              </label>
              <label className="block text-xs font-semibold">
                {copy.email}{" "}
                <span className="font-normal text-[var(--muted-foreground)]">
                  ({locale.startsWith("fr") ? "facultatif" : "optional"})
                </span>
                <input
                  name="email"
                  type="email"
                  autoComplete="email"
                  className="mt-1.5 h-10 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-normal"
                />
              </label>
              <label className="block text-xs font-semibold">
                {copy.preferredTime}
                <input
                  name="preferred_at"
                  type="datetime-local"
                  required
                  min={callbackMin || undefined}
                  max={callbackMax || undefined}
                  className="mt-1.5 h-10 w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-normal"
                />
              </label>
              <p className="font-mono text-[10px] text-[var(--muted-foreground)]">{timezone}</p>
              <label className="flex items-start gap-2 text-[11px] leading-4 text-[var(--muted-foreground)]">
                <input
                  name="voice_consent"
                  type="checkbox"
                  required
                  className="mt-0.5 size-4 accent-[var(--primary)]"
                />
                <span>
                  {locale.startsWith("fr") ? "J'accepte que" : "I agree that"} {schoolName}{" "}
                  {copy.callbackConsent}
                </span>
              </label>
              {callbackError ? (
                <p role="alert" className="text-xs text-red-700">
                  {callbackError}
                </p>
              ) : null}
              <button
                type="submit"
                disabled={callbackBusy}
                className="w-full rounded-lg bg-[var(--primary)] px-3 py-2.5 text-sm font-semibold text-[var(--primary-foreground)] disabled:opacity-50"
              >
                {callbackBusy ? copy.sending : copy.sendCallback}
              </button>
            </form>
          ) : null}
          {leadSubmitted ? (
            <p role="status" className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-900">
              {copy.leadSuccess}
            </p>
          ) : null}
          {callbackSubmitted ? (
            <p role="status" className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-900">
              {copy.callbackSuccess}
            </p>
          ) : null}
          {busy ? (
            <p role="status" className="text-sm text-[var(--muted-foreground)]">
              {copy.checking}
            </p>
          ) : null}
          {error ? (
            <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-800">
              {error}
            </p>
          ) : null}
        </div>

        <form
          onSubmit={(event) => {
            void submit(event);
          }}
          className="shrink-0 border-t border-[var(--border)] bg-[var(--surface)] p-3"
        >
          <label htmlFor="admissions-question" className="sr-only">
            {copy.title}
          </label>
          <div className="flex items-center gap-2 rounded-xl border border-[var(--border)] bg-[var(--background)] p-1.5 focus-within:ring-2 focus-within:ring-[var(--primary)]">
            <input
              ref={inputRef}
              id="admissions-question"
              value={question}
              onChange={(event) => setQuestion(event.target.value)}
              maxLength={500}
              disabled={busy}
              placeholder={copy.placeholder}
              className="min-w-0 flex-1 bg-transparent px-2 py-2 text-sm outline-none"
            />
            <button
              type="submit"
              disabled={busy || question.trim().length < 2}
              className="grid size-9 place-items-center rounded-lg bg-[var(--primary)] text-[var(--primary-foreground)] disabled:opacity-40"
              aria-label={copy.send}
            >
              <Send size={16} />
            </button>
          </div>
          <p className="mt-2 px-1 text-[10px] leading-4 text-[var(--muted-foreground)]">
            {copy.privacy}
          </p>
        </form>
      </section>

      <button
        ref={launcherRef}
        type="button"
        onClick={openAssistant}
        aria-expanded={open}
        aria-controls={assistantId}
        aria-label={copy.open}
        aria-hidden={open}
        tabIndex={open ? -1 : undefined}
        className={`ml-auto grid size-14 place-items-center rounded-full bg-[var(--primary)] text-[var(--primary-foreground)] shadow-lg transition duration-200 hover:-translate-y-0.5 hover:shadow-xl motion-reduce:transition-none ${open ? "pointer-events-none scale-75 opacity-0" : "pointer-events-auto scale-100 opacity-100"}`}
      >
        <MessageCircle aria-hidden="true" />
      </button>
    </div>
  );
}
