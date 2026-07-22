import { chromium } from "/Users/shayford/.npm/_npx/0cf6ff1fad43f633/node_modules/playwright/index.mjs";

const browser = await chromium.launch({ headless: true });
const routes = ["/", "/features", "/pricing", "/about", "/blog", "/contact", "/signup"];
const results = [];

for (const route of routes) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  const errors = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(`console: ${message.text()}`);
  });
  page.on("pageerror", (error) => errors.push(`page: ${error.message}`));
  const response = await page.goto(`http://127.0.0.1:3101${route}`, { waitUntil: "networkidle" });
  results.push({ route, status: response?.status() ?? 0, title: await page.title(), errors });
  await context.close();
}

const mobile = await browser.newContext({
  viewport: { width: 390, height: 844 },
  isMobile: true,
  hasTouch: true,
});
const mobilePage = await mobile.newPage();
await mobilePage.goto("http://127.0.0.1:3101/", { waitUntil: "networkidle" });
await mobilePage.locator("summary").click();
const mobileMenuVisible = await mobilePage
  .getByRole("navigation", { name: "Mobile primary" })
  .isVisible();
await mobilePage.getByRole("link", { name: "Platform", exact: true }).click();
await mobilePage.waitForURL("**/features");
results.push({
  interaction: "mobile navigation",
  menuVisible: mobileMenuVisible,
  destination: new URL(mobilePage.url()).pathname,
});
await mobile.close();

const formContext = await browser.newContext({ viewport: { width: 1280, height: 900 } });
const contactPage = await formContext.newPage();
await contactPage.goto("http://127.0.0.1:3101/contact", { waitUntil: "networkidle" });
await contactPage.getByLabel("Name").fill("Ama Mensah");
await contactPage.getByLabel("School").fill("Cape Coast Learning Academy");
await contactPage.getByLabel("Work email").fill("ama@example.edu.gh");
await contactPage.getByLabel("I am interested in").selectOption("demo");
await contactPage
  .getByLabel("What is happening today?")
  .fill("We want one dependable view of attendance and family follow-up.");
results.push({
  interaction: "contact form",
  fieldsReady: await contactPage.getByRole("button", { name: "Prepare my message" }).isEnabled(),
});

const signupPage = await formContext.newPage();
await signupPage.goto("http://127.0.0.1:3101/signup?plan=growth", { waitUntil: "networkidle" });
await signupPage.getByLabel("School name").fill("Cape Coast Learning Academy");
await signupPage.getByLabel("Administrator name").fill("Ama Mensah");
await signupPage.getByLabel("Work email").fill("ama@example.edu.gh");
await signupPage.getByLabel("Plan interest").selectOption("growth");
await signupPage.getByLabel(/I confirm I am authorized/).check();
results.push({
  interaction: "onboarding form",
  selectedPlan: await signupPage.getByLabel("Plan interest").inputValue(),
  submitReady: await signupPage.getByRole("button", { name: "Submit for review" }).isEnabled(),
});
await formContext.close();

await browser.close();
console.log(JSON.stringify(results, null, 2));
