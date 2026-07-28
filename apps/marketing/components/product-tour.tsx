"use client";

import { useRef, useState } from "react";
import Image from "next/image";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  LayoutDashboard,
  MonitorSmartphone,
  ReceiptText,
  Smartphone,
  Wallet,
  type LucideIcon,
} from "lucide-react";

export interface TourStop {
  id: string;
  title: string;
  caption: string;
  /** Desktop framing shot, served from /tour. */
  image: string;
  /** Optional narrow-viewport variant slot, served from /tour. */
  mobileImage?: string;
  alt: string;
  /**
   * Static manifest flag. Screenshots are dropped into public/tour later; until a
   * stop is flagged ready the frame renders the designed placeholder instead of
   * a broken image. Never a runtime filesystem check.
   */
  imageReady: boolean;
  icon: LucideIcon;
}

export const tourStops: TourStop[] = [
  {
    id: "admin-dashboard",
    title: "Admin dashboard",
    caption:
      "The morning in one view—attendance as it lands, balances that need attention and the notices waiting for approval.",
    image: "/tour/admin-dashboard.png",
    mobileImage: "/tour/admin-dashboard-mobile.png",
    alt: "AuraEDU admin dashboard showing attendance, fee balances and pending approvals",
    imageReady: false,
    icon: LayoutDashboard,
  },
  {
    id: "fees-momo",
    title: "Fees & MoMo payments",
    caption:
      "Invoices go out, guardians pay by mobile money and every cedi reconciles against the right learner automatically.",
    image: "/tour/fees-momo.png",
    mobileImage: "/tour/fees-momo-mobile.png",
    alt: "AuraEDU fees screen showing an invoice paid by mobile money and reconciled",
    imageReady: false,
    icon: Wallet,
  },
  {
    id: "report-cards",
    title: "Report-card publishing",
    caption:
      "Teachers finalize marks, heads review and the school publishes once—parents see results the moment they are released.",
    image: "/tour/report-cards.png",
    mobileImage: "/tour/report-cards-mobile.png",
    alt: "AuraEDU report-card publishing flow from teacher marks to published results",
    imageReady: false,
    icon: ReceiptText,
  },
  {
    id: "parent-portal",
    title: "Parent portal",
    caption:
      "Fees, receipts, report cards, attendance and announcements in the one place a guardian actually checks.",
    image: "/tour/parent-portal.png",
    mobileImage: "/tour/parent-portal-mobile.png",
    alt: "AuraEDU parent portal showing fees, receipts, report cards and announcements",
    imageReady: false,
    icon: Smartphone,
  },
];

function ScreenshotFrame({ stop }: { stop: TourStop }) {
  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_26px_70px_-42px_rgba(12,36,75,0.4)]">
      <div
        aria-hidden="true"
        className="flex h-9 items-center gap-1.5 border-b border-slate-200 bg-slate-50 px-3.5"
      >
        <span className="size-2 rounded-full bg-cobalt" />
        <span className="size-2 rounded-full bg-teal-bright" />
        <span className="size-2 rounded-full bg-lime-signal" />
        <span className="ml-2 truncate font-mono text-[10px] font-semibold tracking-[0.08em] text-slate-500">
          app.auraedu.com · {stop.title.toLowerCase()}
        </span>
      </div>
      {stop.imageReady ? (
        <picture className="block">
          {stop.mobileImage ? (
            <source media="(max-width: 640px)" srcSet={stop.mobileImage} />
          ) : null}
          <Image
            src={stop.image}
            alt={stop.alt}
            width={1600}
            height={1000}
            className="aspect-[4/5] w-full object-cover object-top sm:aspect-[16/10]"
          />
        </picture>
      ) : (
        <div
          role="img"
          aria-label={stop.alt}
          className="relative flex aspect-[4/5] flex-col justify-between overflow-hidden bg-navy-deep p-6 sm:aspect-[16/10] sm:p-8"
        >
          <div
            aria-hidden="true"
            className="absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,rgba(21,87,255,0.45),transparent_55%),radial-gradient(ellipse_at_bottom_left,rgba(8,127,140,0.5),transparent_55%)]"
          />
          <div
            aria-hidden="true"
            className="absolute -right-16 -top-16 size-64 rounded-full border-[26px] border-white/5"
          />
          <div className="relative flex items-center justify-between">
            <span className="font-mono text-[10px] font-bold uppercase tracking-[0.18em] text-teal-bright">
              AuraEDU · {stop.title}
            </span>
            <span className="inline-flex items-center gap-1.5 rounded-full border border-white/15 px-2.5 py-1 text-[10px] font-semibold text-slate-300">
              <MonitorSmartphone className="size-3" aria-hidden="true" />
              Desktop + mobile
            </span>
          </div>
          <div className="relative">
            <span className="grid size-12 place-items-center rounded-xl bg-white/10 text-lime-signal">
              <stop.icon className="size-6" aria-hidden="true" />
            </span>
            <p className="mt-4 max-w-[38ch] text-sm leading-6 text-slate-300">{stop.caption}</p>
            <p className="mt-3 font-mono text-[10px] uppercase tracking-[0.16em] text-slate-500">
              Live screenshot ships with the next content drop
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

export function ProductTour({ stops = tourStops }: { stops?: TourStop[] }) {
  const [active, setActive] = useState(0);
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const reduced = useReducedMotion();
  const stop = stops[active] ?? stops[0];
  if (!stop) return null;

  function select(index: number) {
    setActive(index);
    tabRefs.current[index]?.focus();
  }

  function handleKeyDown(event: React.KeyboardEvent, index: number) {
    const last = stops.length - 1;
    if (event.key === "ArrowRight") {
      event.preventDefault();
      select(index === last ? 0 : index + 1);
    } else if (event.key === "ArrowLeft") {
      event.preventDefault();
      select(index === 0 ? last : index - 1);
    } else if (event.key === "Home") {
      event.preventDefault();
      select(0);
    } else if (event.key === "End") {
      event.preventDefault();
      select(last);
    }
  }

  return (
    <div>
      <div role="tablist" aria-label="Product tour" className="flex flex-wrap gap-2">
        {stops.map((item, index) => {
          const selected = index === active;
          return (
            <button
              key={item.id}
              ref={(node) => {
                tabRefs.current[index] = node;
              }}
              role="tab"
              id={`tour-tab-${item.id}`}
              aria-selected={selected}
              aria-controls={`tour-panel-${item.id}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => setActive(index)}
              onKeyDown={(event) => handleKeyDown(event, index)}
              className={`inline-flex min-h-11 items-center gap-2 rounded-full border px-4 py-2 text-xs font-bold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt ${
                selected
                  ? "border-navy-deep bg-navy-deep text-white"
                  : "border-slate-300 bg-white text-slate-600 hover:border-cobalt hover:text-cobalt"
              }`}
            >
              <span className="font-mono text-[10px] opacity-60">
                {String(index + 1).padStart(2, "0")}
              </span>
              {item.title}
            </button>
          );
        })}
      </div>

      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={stop.id}
          role="tabpanel"
          id={`tour-panel-${stop.id}`}
          aria-labelledby={`tour-tab-${stop.id}`}
          initial={reduced ? false : { opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          exit={reduced ? undefined : { opacity: 0, y: -8 }}
          transition={{ duration: reduced ? 0 : 0.32, ease: [0.25, 1, 0.5, 1] }}
          className="mt-6 grid items-center gap-8 lg:grid-cols-[0.62fr_1.38fr]"
        >
          <div>
            <p className="font-mono text-xs font-bold uppercase tracking-[0.16em] text-teal-strong">
              {String(active + 1).padStart(2, "0")} / {String(stops.length).padStart(2, "0")}
            </p>
            <h3 className="mt-3 text-3xl font-bold tracking-[-0.035em] text-navy-deep">
              {stop.title}
            </h3>
            <p className="mt-4 max-w-md leading-7 text-slate-600">{stop.caption}</p>
          </div>
          <ScreenshotFrame stop={stop} />
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
