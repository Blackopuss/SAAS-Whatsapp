// Capturas y axe del panel sin escribir nada en la base (check.mjs sí guarda datos).
// Uso: node panel-shots.mjs  → out/panel-<página>-<ancho>.png
import { chromium } from "playwright";
import AxeBuilder from "@axe-core/playwright";

const BASE = process.env.BASE ?? "http://ana.localhost:8080";
const EMAIL = process.env.EMAIL ?? "ana@demo.mx";
const PASS = process.env.PASS ?? "demo-ana-2026";
const b = await chromium.launch();
let failed = false;
for (const [w, h] of [[360, 740], [1280, 860]]) {
  const page = await (await b.newContext({ viewport: { width: w, height: h }, deviceScaleFactor: w < 400 ? 2 : 1 })).newPage();
  await page.goto(`${BASE}/panel/login`);
  await page.getByLabel("Email").fill(EMAIL);
  await page.getByLabel("Contraseña").fill(PASS);
  await page.getByRole("button", { name: "Entrar" }).click();
  await page.waitForURL(`${BASE}/panel/`);
  for (const [name, path] of [["inicio", "/panel/"], ["productos", "/panel/productos"], ["ventas-resumen", "/panel/ventas/resumen"]]) {
    await page.goto(BASE + path);
    await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
    await page.waitForTimeout(800);
    await page.screenshot({ path: `out/panel-${name}-${w}.png`, fullPage: true });
    const wide = await page.evaluate(() => document.documentElement.scrollWidth);
    if (wide > w) { failed = true; console.log(`  ${name} ${w}: se desborda (${wide}px)`); }
    const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "best-practice"]).analyze();
    for (const v of axe.violations) {
      failed = true;
      console.log(`  axe [${name} ${w}] ${v.id}: ${v.help}`);
      for (const n of v.nodes.slice(0, 3)) console.log(`      ${n.target.join(" ")}`);
    }
    console.log(`${name} ${w}: axe ${axe.violations.length}`);
  }
}
await b.close();
process.exit(failed ? 1 : 0);
