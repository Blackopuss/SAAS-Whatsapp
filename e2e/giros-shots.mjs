// Capturas de la tienda de ana con cada giro, sin tocar la base: reescribe el HTML (tema, fuente y "style").
// Uso: node giros-shots.mjs  → out/giro-<giro>-<ancho>.png y out/giro-<giro>-<ancho>-scroll.png
import { chromium } from "playwright";

const BASE = process.env.BASE ?? "http://ana.localhost:8080";
const FONT = { vitrina: "bricolage", boutique: "cormorant", barberia: "bigshoulders", carniceria: "alfaslab", abarrotes: "lilita", papeleria: "fredoka" };
const giros = process.argv[2] ? process.argv[2].split(",") : Object.keys(FONT);
const b = await chromium.launch();
for (const giro of giros) {
  for (const [w, h] of [[360, 740], [1366, 860]]) {
    const ctx = await b.newContext({ viewport: { width: w, height: h } });
    const page = await ctx.newPage();
    await page.route(`${BASE}/`, async (route) => {
      const r = await route.fetch({ url: "http://127.0.0.1:8080/", headers: { ...route.request().headers(), host: new URL(BASE).host } });
      let html = await r.text();
      const cur = html.match(/data-astryx-theme="([a-z]+)"/)[1];
      html = html.replaceAll(`theme-${cur}.css`, `theme-${giro}.css`).replaceAll(`data-astryx-theme="${cur}"`, `data-astryx-theme="${giro}"`)
        .replaceAll(`"style":"${cur}"`, `"style":"${giro}"`).replaceAll(`fonts/${FONT[cur]}.woff2`, `fonts/${FONT[giro]}.woff2`);
      await route.fulfill({ response: r, body: html });
    });
    await page.goto(`${BASE}/`);
    await page.waitForTimeout(1600);
    await page.screenshot({ path: `out/giro-${giro}-${w}.png` });
    const wide = await page.evaluate(() => document.documentElement.scrollWidth);
    if (wide > w) console.log(`${giro} ${w}: se desborda (${wide}px)`);
    // A media página: el enunciado y el destacado ya pasaron por su momento.
    await page.evaluate(() => document.querySelector(".statement")?.scrollIntoView({ block: "center" }));
    await page.waitForTimeout(500);
    await page.screenshot({ path: `out/giro-${giro}-${w}-statement.png` });
    await page.evaluate(() => (document.querySelector(".spot") ?? document.querySelector(".chapters"))?.scrollIntoView({ block: "start" }));
    await page.evaluate(() => scrollBy(0, -80));
    await page.waitForTimeout(1900);
    await page.screenshot({ path: `out/giro-${giro}-${w}-scroll.png` });
    // Capítulos (boutique): a la mitad de la subida de la segunda foto y ya en la tercera.
    if (await page.locator(".chapters.live").count()) {
      for (const [k, f] of [["a", 0.18], ["b", 0.8]]) {
        await page.evaluate((f) => { const s = document.querySelector(".chapters"); scrollTo(0, s.getBoundingClientRect().top + scrollY + (s.offsetHeight - innerHeight) * f); }, f);
        await page.waitForTimeout(700);
        await page.screenshot({ path: `out/giro-${giro}-${w}-chap-${k}.png` });
      }
    }
    await ctx.close();
  }
  console.log(giro, "listo");
}
await b.close();
