import type { MetadataRoute } from "next";
import { fieldNotes } from "./blog/content";

export default function sitemap(): MetadataRoute.Sitemap {
  const updated = new Date();
  const staticPages: MetadataRoute.Sitemap = [
    "",
    "/features",
    "/pricing",
    "/about",
    "/blog",
    "/contact",
    "/signup",
    "/privacy",
    "/security",
    "/accessibility",
  ].map((path) => ({
    url: `https://auraedu.com${path}`,
    lastModified: updated,
    changeFrequency: path === "" ? "weekly" : "monthly",
    priority: path === "" ? 1 : path === "/signup" ? 0.9 : 0.7,
  }));
  const articles: MetadataRoute.Sitemap = fieldNotes.map((note) => ({
    url: `https://auraedu.com/blog/${note.slug}`,
    lastModified: updated,
    changeFrequency: "monthly",
    priority: 0.65,
  }));
  return [...staticPages, ...articles];
}
