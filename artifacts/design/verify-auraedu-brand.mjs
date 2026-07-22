import { chromium } from "/Users/shayford/.npm/_npx/0cf6ff1fad43f633/node_modules/playwright/index.mjs";

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 1,
});
const errors = [];
page.on("console", (message) => {
  if (message.type() === "error") errors.push(message.text());
});
page.on("pageerror", (error) => errors.push(error.message));
await page.goto("http://127.0.0.1:3101/", { waitUntil: "networkidle" });
await page.locator("header").screenshot({ path: "artifacts/design/auraedu-brand-header.png" });
await page.locator("footer").scrollIntoViewIfNeeded();
await page.waitForTimeout(200);
await page.locator("footer").screenshot({ path: "artifacts/design/auraedu-brand-footer.png" });
const brandImages = await page.locator('img[alt="AuraEDU"]').count();
const iconLinks = await page
  .locator('link[rel~="icon"], link[rel="apple-touch-icon"]')
  .evaluateAll((links) =>
    links.map((link) => ({ rel: link.rel, href: link.getAttribute("href") })),
  );
console.log(JSON.stringify({ brandImages, iconLinks, errors }, null, 2));
await browser.close();
