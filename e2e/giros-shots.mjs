// Capturas de la tienda de ana con cada giro, sin tocar la base: reescribe el HTML (tema, fuente y "style").
// Uso: node giros-shots.mjs  → out/giro-<giro>-<ancho>.png y out/giro-<giro>-<ancho>-{mid,scroll}.png
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
    // Categorías y ofertas, luego el catálogo.
    for (const [k, sel] of [["mid", "#cats, #oferta"], ["scroll", "#catalogo"]]) {
      await page.evaluate((sel) => { document.querySelector(sel)?.scrollIntoView({ block: "start" }); scrollBy(0, -120); }, sel);
      await page.waitForTimeout(900);
      await page.screenshot({ path: `out/giro-${giro}-${w}-${k}.png` });
    }
    await ctx.close();
  }
  console.log(giro, "listo");
}
await b.close();
