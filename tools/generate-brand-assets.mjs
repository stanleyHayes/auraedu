import { createRequire } from "node:module";
import { mkdir, readdir, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const pnpmStore = join(root, "node_modules", ".pnpm");
const sharpPackage = (await readdir(pnpmStore)).find((entry) => entry.startsWith("sharp@"));

if (!sharpPackage) {
  throw new Error("Sharp is unavailable. Run pnpm install before generating brand assets.");
}

const require = createRequire(import.meta.url);
const sharp = require(join(pnpmStore, sharpPackage, "node_modules", "sharp"));
const mark = join(root, "apps", "marketing", "app", "icon.svg");
const lockup = join(root, "apps", "marketing", "public", "brand", "auraedu-logo-dark.svg");
const lightLockup = join(root, "apps", "marketing", "public", "brand", "auraedu-logo-light.svg");
const webApps = ["marketing", "web"];
const faviconSizes = [16, 32, 48, 64, 128, 256];

async function png(source, width, height, options = {}) {
  let pipeline = sharp(source, { density: 384 }).resize(width, height, { fit: "contain" });
  if (options.flatten) pipeline = pipeline.flatten({ background: options.flatten });
  return pipeline.png().toBuffer();
}

function ico(images) {
  const directorySize = 6 + images.length * 16;
  const header = Buffer.alloc(directorySize);
  header.writeUInt16LE(0, 0);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(images.length, 4);
  let offset = directorySize;

  images.forEach(({ size, bytes }, index) => {
    const entry = 6 + index * 16;
    header.writeUInt8(size === 256 ? 0 : size, entry);
    header.writeUInt8(size === 256 ? 0 : size, entry + 1);
    header.writeUInt8(0, entry + 2);
    header.writeUInt8(0, entry + 3);
    header.writeUInt16LE(1, entry + 4);
    header.writeUInt16LE(32, entry + 6);
    header.writeUInt32LE(bytes.length, entry + 8);
    header.writeUInt32LE(offset, entry + 12);
    offset += bytes.length;
  });

  return Buffer.concat([header, ...images.map(({ bytes }) => bytes)]);
}

const faviconImages = await Promise.all(
  faviconSizes.map(async (size) => ({ size, bytes: await png(mark, size, size) })),
);
const favicon = ico(faviconImages);
const appleIcon = await png(mark, 180, 180, { flatten: "#061631" });

for (const app of webApps) {
  const appDir = join(root, "apps", app, "app");
  await writeFile(join(appDir, "favicon.ico"), favicon);
  await writeFile(join(appDir, "apple-icon.png"), appleIcon);
}

const mobileAssets = join(root, "apps", "mobile", "assets");
await mkdir(mobileAssets, { recursive: true });
await writeFile(
  join(mobileAssets, "icon.png"),
  await png(mark, 1024, 1024, { flatten: "#061631" }),
);
const adaptiveCore = await png(mark, 704, 704);
const adaptiveIcon = await sharp(adaptiveCore)
  .extend({
    top: 160,
    right: 160,
    bottom: 160,
    left: 160,
    background: { r: 0, g: 0, b: 0, alpha: 0 },
  })
  .png()
  .toBuffer();
await writeFile(join(mobileAssets, "adaptive-icon.png"), adaptiveIcon);
await writeFile(join(mobileAssets, "auraedu-logo.png"), await png(lockup, 416, 96));
await writeFile(join(mobileAssets, "auraedu-logo-light.png"), await png(lightLockup, 416, 96));

console.log("Generated AuraEDU web favicons, Apple touch icons, and mobile identity assets.");
