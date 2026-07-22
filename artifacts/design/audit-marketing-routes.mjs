import { chromium } from "/Users/shayford/.npm/_npx/0cf6ff1fad43f633/node_modules/playwright/index.mjs";

const routes = ["", "features", "pricing", "about", "blog", "contact", "signup"];
const browser = await chromium.launch({ headless: true });

async function capture(route, viewport, suffix, mobile = false) {
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 1,
    isMobile: mobile,
    hasTouch: mobile,
  });
  const page = await context.newPage();
  const errors = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("pageerror", (error) => errors.push(error.message));
  const path = route ? `/${route}` : "/";
  const response = await page.goto(`http://127.0.0.1:3101${path}`, { waitUntil: "networkidle" });
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  for (let y = 0; y < height; y += Math.max(360, Math.floor(viewport.height * 0.72))) {
    await page.evaluate((nextY) => window.scrollTo({ top: nextY, behavior: "instant" }), y);
    await page.waitForTimeout(130);
  }
  await page.evaluate(() => window.scrollTo({ top: 0, behavior: "instant" }));
  await page.waitForTimeout(250);
  const name = route || "home";
  await page.screenshot({
    path: `artifacts/design/audit-current/${name}-${suffix}.png`,
    fullPage: true,
  });
  console.log(JSON.stringify({ path, status: response?.status(), errors }));
  await context.close();
}

for (const route of routes) {
  await capture(route, { width: 1440, height: 1000 }, "desktop");
  await capture(route, { width: 390, height: 844 }, "mobile", true);
}

await browser.close();
