import Image from "next/image";
import Link from "next/link";
import { ArrowRight, BrainCircuit, Database, ShieldCheck } from "lucide-react";
import { Eyebrow } from "@/components/brand-primitives";
import { ScrollReveal, StaggerChildren, StaggerItem } from "@/components/motion-primitives";
import { fieldNotes } from "./content";

export const metadata = {
  title: "Field notes",
  description: "Ideas from the team building a more dependable education operating system.",
};

const icons = { Teaching: BrainCircuit, Platform: Database, Design: ShieldCheck };

export default function BlogPage() {
  const featured = fieldNotes[0]!;
  return (
    <div className="overflow-hidden bg-white">
      <section className="resource-hero">
        <ScrollReveal className="mx-auto max-w-7xl px-6 pb-16 pt-24">
          <Eyebrow>Field notes from AuraEDU</Eyebrow>
          <div className="mt-5 grid gap-8 lg:grid-cols-[1.1fr_0.9fr] lg:items-end">
            <h1 className="max-w-[12ch] text-balance text-[clamp(3.5rem,7vw,7.5rem)] font-bold leading-[0.87] tracking-[-0.06em] text-navy-deep">
              Ideas for a school system that can{" "}
              <span className="text-teal-strong">remember, respond and improve.</span>
            </h1>
            <p className="max-w-xl pb-2 text-lg leading-8 text-slate-600">
              Notes on education, product craft, trusted data and responsible AI—from the principles
              shaping the platform.
            </p>
          </div>
        </ScrollReveal>
      </section>

      <section className="mx-auto max-w-7xl px-6 pb-24">
        <ScrollReveal>
          <Link href={`/blog/${featured.slug}`} className="featured-note group">
            <div className="relative min-h-[420px] overflow-hidden">
              <Image
                src={featured.image}
                alt={featured.imageAlt}
                fill
                priority
                sizes="(max-width: 1024px) 100vw, 58vw"
                className="object-cover transition-transform duration-700 group-hover:scale-[1.025]"
              />
            </div>
            <div className="flex flex-col justify-center p-8 sm:p-12">
              <p className="resource-meta">Featured · {featured.area}</p>
              <h2>{featured.title}</h2>
              <p className="mt-5 text-lg leading-8 text-slate-300">{featured.summary}</p>
              <span className="resource-read">
                {featured.readTime}{" "}
                <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" />
              </span>
            </div>
          </Link>
        </ScrollReveal>

        <StaggerChildren className="mt-6 grid gap-6 lg:grid-cols-2">
          {fieldNotes.slice(1).map((note) => {
            const Icon = icons[note.area as keyof typeof icons] ?? ShieldCheck;
            return (
              <StaggerItem key={note.number} className="h-full">
                <Link href={`/blog/${note.slug}`} className="resource-card-link group">
                  <article className={`resource-card resource-card-${note.tone}`}>
                    <div className="relative aspect-[16/8] overflow-hidden">
                      <Image
                        src={note.image}
                        alt={note.imageAlt}
                        fill
                        sizes="(max-width: 1024px) 100vw, 50vw"
                        className="object-cover transition-transform duration-700 group-hover:scale-[1.035]"
                      />
                    </div>
                    <div className="p-7 sm:p-9">
                      <div className="flex items-center justify-between">
                        <p className="resource-meta">{note.area}</p>
                        <Icon className="size-5" />
                      </div>
                      <h2>{note.title}</h2>
                      <p>{note.summary}</p>
                      <span className="resource-read">
                        {note.readTime}{" "}
                        <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" />
                      </span>
                    </div>
                  </article>
                </Link>
              </StaggerItem>
            );
          })}
        </StaggerChildren>
      </section>

      <section className="trust-stage text-white">
        <ScrollReveal className="mx-auto grid max-w-7xl gap-8 px-6 py-20 lg:grid-cols-[1fr_auto] lg:items-center">
          <div>
            <Eyebrow inverse>Go deeper</Eyebrow>
            <h2 className="mt-4 max-w-[18ch] text-4xl font-bold tracking-[-0.04em] sm:text-5xl">
              The thinking behind the platform lives in the engineering handbook.
            </h2>
            <p className="mt-4 max-w-2xl leading-7 text-slate-300">
              Architecture, product rules and operating principles are documented so humans and AI
              agents work from the same source of truth.
            </p>
          </div>
          <Link href="/contact" className="cta-primary">
            Discuss the platform <ArrowRight className="size-4" />
          </Link>
        </ScrollReveal>
      </section>
    </div>
  );
}
