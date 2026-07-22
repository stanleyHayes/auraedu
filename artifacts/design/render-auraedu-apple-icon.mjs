import { chromium } from "/Users/shayford/.npm/_npx/0cf6ff1fad43f633/node_modules/playwright/index.mjs";
import { readFile } from "node:fs/promises";

const svg = await readFile("apps/marketing/app/icon.svg", "utf8");
const dataUrl = `data:image/svg+xml;base64,${Buffer.from(svg).toString("base64")}`;

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 180, height: 180 }, deviceScaleFactor: 1 });
await page.setContent(`
  <style>*{box-sizing:border-box}html,body{margin:0;width:180px;height:180px;overflow:hidden;background:#04122b}img{display:block;width:180px;height:180px}</style>
  <img src="${dataUrl}" alt="">
`);
await page.waitForTimeout(100);
await page.screenshot({ path: "apps/marketing/app/apple-icon.png", omitBackground: false });
await browser.close();
