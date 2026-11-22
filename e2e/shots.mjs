// Revisión rápida sin Lighthouse: capturas a 360 px y axe de las páginas públicas de la tienda.
// Uso: node shots.mjs  (deja las capturas en out/movil-*.png)
import { chromium } from "playwright";
import AxeBuilder from "@axe-core/playwright";

const BASE = process.env.BASE ?? "http://ana.localhost:8080";
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 360, height: 740 }, deviceScaleFactor: 2 });
const page = await ctx.newPage();
let failed = false;
const prod = await (async () => {
  await page.goto(`${BASE}/`);
  await page.locator("a.card:not(.sold)").first().waitFor();
  return page.locator("a.card:not(.sold)").first().getAttribute("href");
})();
for (const [name, path] of [["inicio", "/"], ["categoria", "/c/" + (await page.locator('nav[aria-label="Categorías"] a').nth(1).getAttribute("href")).split("/c/")[1]], ["producto", prod], ["preguntas", "/preguntas"]]) {
  await page.goto(BASE + path);
  await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
  await page.evaluate(() => Promise.race([Promise.all(document.getAnimations().map((a) => a.finished.catch(() => {}))), new Promise((r) => setTimeout(r, 3000))]));
  const wide = await page.evaluate(() => document.documentElement.scrollWidth);
  if (wide > 360) { failed = true; console.log(`  ${name}: la página mide ${wide}px de ancho (se desborda)`); }
  await page.screenshot({ path: `out/movil-${name}.png`, fullPage: true });
  const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"]).analyze();
  for (const v of axe.violations) {
    failed = true;
    console.log(`  axe [${name}] ${v.id}: ${v.help}`);
    for (const n of v.nodes.slice(0, 3)) console.log(`      ${n.target.join(" ")}\n        ${n.failureSummary?.split("\n").slice(1).join(" ")}`);
  }
  console.log(`${name}: axe ${axe.violations.length}`);
}
// Agregar rápido desde la tarjeta.
await page.goto(`${BASE}/`);
await page.getByRole("button", { name: /^Agregar .* al carrito$/ }).first().click();
const sheet = page.getByRole("dialog").filter({ has: page.getByRole("button", { name: "Agregar al carrito" }) });
await sheet.waitFor();
await page.waitForTimeout(400);
await page.screenshot({ path: "out/movil-agregar-rapido.png" });
const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "best-practice"]).analyze();
for (const v of axe.violations) { failed = true; console.log(`  axe [agregar-rapido] ${v.id}: ${v.help}`); }
console.log(`agregar-rapido: axe ${axe.violations.length}`);
await b.close();
process.exit(failed ? 1 : 0);
