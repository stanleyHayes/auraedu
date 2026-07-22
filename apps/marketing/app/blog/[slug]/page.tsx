import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, ArrowRight, Check } from "lucide-react";
import { Eyebrow } from "@/components/brand-primitives";
import { ScrollReveal, StaggerChildren, StaggerItem } from "@/components/motion-primitives";
import { fieldNotes, getFieldNote } from "../content";

export function generateStaticParams() {
  return fieldNotes.map((note) => ({ slug: note.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const note = getFieldNote((await params).slug);
  if (!note) return {};
  return { title: note.title, description: note.summary };
}

export default async function FieldNotePage({ params }: { params: Promise<{ slug: string }> }) {
  const note = getFieldNote((await params).slug);
  if (!note) notFound();
  const next = fieldNotes[(fieldNotes.indexOf(note) + 1) % fieldNotes.length]!;

  return (
    <article className="overflow-hidden bg-white">
      <header className="article-hero text-white">
        <div className="mx-auto grid max-w-[1440px] lg:grid-cols-[0.9fr_1.1fr]">
          <ScrollReveal className="flex min-h-[620px] flex-col justify-center px-6 py-20 sm:px-10 lg:px-16">
            <Link href="/blog" className="article-back">
              <ArrowLeft className="size-4" /> All field notes
            </Link>
            <p className="resource-meta mt-12 !text-teal-bright">
              {note.area} · Note {note.number} · {note.readTime}
            </p>
            <h1 className="mt-5 max-w-[13ch] text-balance text-[clamp(3.1rem,5.8vw,6.4rem)] font-bold leading-[0.9] tracking-[-0.055em]">
              {note.title}
            </h1>
            <p className="mt-7 max-w-xl text-lg leading-8 text-slate-300">{note.summary}</p>
          </ScrollReveal>
          <div className="relative min-h-[520px] lg:min-h-full">
            <Image
              src={note.image}
              alt={note.imageAlt}
              fill
              priority
              sizes="(max-width: 1024px) 100vw, 55vw"
              className="object-cover"
            />
            <div className="article-photo-shade absolute inset-0" aria-hidden="true" />
          </div>
        </div>
      </header>

      <div className="mx-auto grid max-w-7xl gap-12 px-6 py-20 lg:grid-cols-[0.7fr_1.3fr] lg:py-28">
        <aside>
          <ScrollReveal className="article-thesis">
            <Eyebrow>The position</Eyebrow>
            <p>{note.thesis}</p>
          </ScrollReveal>
          <StaggerChildren className="mt-8 grid gap-3">
            {note.principles.map((principle) => (
              <StaggerItem key={principle} className="article-principle">
                <Check className="size-4" />
                <span>{principle}</span>
              </StaggerItem>
            ))}
          </StaggerChildren>
        </aside>

        <div className="article-body">
          {note.sections.map((section, index) => (
            <ScrollReveal key={section.heading} className="article-section">
              <span>0{index + 1}</span>
              <h2>{section.heading}</h2>
              {section.body.map((paragraph) => (
                <p key={paragraph}>{paragraph}</p>
              ))}
            </ScrollReveal>
          ))}
        </div>
      </div>

      <section className="bg-cool-mist px-6 py-20">
        <ScrollReveal className="article-next mx-auto max-w-7xl">
          <div>
            <p className="resource-meta">Read next · {next.area}</p>
            <h2>{next.title}</h2>
          </div>
          <Link href={`/blog/${next.slug}`} className="cta-solid">
            Continue reading <ArrowRight className="size-4" />
          </Link>
        </ScrollReveal>
      </section>
    </article>
  );
}
