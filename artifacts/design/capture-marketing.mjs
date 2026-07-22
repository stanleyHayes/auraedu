import { chromium } from "/Users/shayford/.npm/_npx/0cf6ff1fad43f633/node_modules/playwright/index.mjs";

const browser = await chromium.launch({ headless: true });

async function capture({ viewport, output, mobile = false }) {
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 1,
    isMobile: mobile,
    hasTouch: mobile,
  });
  const page = await context.newPage();
  await page.goto("http://127.0.0.1:3101/", { waitUntil: "networkidle" });

  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  for (let y = 0; y < height; y += Math.max(320, Math.floor(viewport.height * 0.7))) {
    await page.evaluate((nextY) => window.scrollTo({ top: nextY, behavior: "instant" }), y);
    await page.waitForTimeout(220);
  }

  await page.evaluate(() => window.scrollTo({ top: 0, behavior: "instant" }));
  await page.waitForTimeout(500);
  await page.screenshot({ path: output, fullPage: true });
  await context.close();
}

await capture({
  viewport: { width: 1440, height: 1100 },
  output: "artifacts/design/marketing-desktop-revealed.png",
});

await capture({
  viewport: { width: 390, height: 844 },
  output: "artifacts/design/marketing-mobile-revealed.png",
  mobile: true,
});

await browser.close();
