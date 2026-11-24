// Revisa los seis giros con los datos reales de ana sin guardar nada: reescribe el HTML de cada página
// (tema, fuente y "style") como hace la vista previa del panel. Captura portada, categoría, catálogo y ficha
// a 1440 y 390 px; corre axe, busca desborde lateral, fotos rotas y errores de consola; prueba buscar,
// ir a una categoría, agregar sin talla, agregar con talla, el carrito y quitar. No abre WhatsApp.
// Uso: node giros-full.mjs [giro,…]  → out/g-<giro>-<vista>-<ancho>.png y out/g-comparacion-<ancho>.png
import { chromium } from "playwright";
import AxeBuilder from "@axe-core/playwright";

const BASE = process.env.BASE ?? "http://ana.localhost:8080";
const EMAIL = process.env.EMAIL ?? "ana@demo.mx";
const PASS = process.env.PASS ?? "demo-ana-2026";
const FONT = { vitrina: "bricolage", boutique: "cormorant", barberia: "bigshoulders", carniceria: "alfaslab", abarrotes: "lilita", papeleria: "fredoka" };
const giros = process.argv[2] ? process.argv[2].split(",") : Object.keys(FONT);
const host = new URL(BASE).host;
const b = await chromium.launch();
let failed = false;
const bad = (m) => { failed = true; console.log("  ✗ " + m); };

async function rewrite(page, giro) {
  await page.route((u) => u.host === host, async (route) => {
    if (route.request().resourceType() !== "document") return route.continue();
    const u = new URL(route.request().url());
    const r = await route.fetch({ url: `http://127.0.0.1:8080${u.pathname}${u.search}`, headers: { ...route.request().headers(), host } });
    let html = await r.text();
    const cur = html.match(/data-astryx-theme="([a-z]+)"/)?.[1];
    if (cur) {
      html = html.replaceAll(`theme-${cur}.css`, `theme-${giro}.css`).replaceAll(`data-astryx-theme="${cur}"`, `data-astryx-theme="${giro}"`)
        .replaceAll(`"style":"${cur}"`, `"style":"${giro}"`).replaceAll(`fonts/${FONT[cur]}.woff2`, `fonts/${FONT[giro]}.woff2`);
    }
    await route.fulfill({ response: r, body: html });
  });
}

async function audit(page, label, w) {
  const wide = await page.evaluate(() => document.documentElement.scrollWidth);
  if (wide > w) bad(`${label} ${w}: se desborda (${wide}px)`);
  const broken = await page.evaluate(() => [...document.images].filter((i) => i.complete && i.currentSrc && i.naturalWidth === 0).map((i) => i.currentSrc));
  if (broken.length) bad(`${label} ${w}: fotos rotas ${broken.slice(0, 2).join(", ")}`);
  const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "best-practice"]).analyze();
  for (const v of axe.violations) bad(`axe ${label} ${w} ${v.id}: ${v.help} → ${v.nodes.slice(0, 2).map((n) => n.target.join(" ")).join(" | ")}`);
}

const settle = async (page) => {
  await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
  await page.evaluate(async () => { for (let y = 0; y < document.body.scrollHeight; y += 600) { scrollTo(0, y); await new Promise((r) => setTimeout(r, 60)); } scrollTo(0, 0); });
  await page.waitForTimeout(700);
};

for (const giro of giros) {
  for (const [w, h] of [[1440, 900], [390, 844]]) {
    const ctx = await b.newContext({ viewport: { width: w, height: h }, deviceScaleFactor: 1, reducedMotion: "reduce" });
    const page = await ctx.newPage();
    const errors = [];
    page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
    page.on("pageerror", (e) => errors.push(String(e)));
    await rewrite(page, giro);

    // Portada y catálogo.
    await page.goto(`${BASE}/`);
    await settle(page);
    const data = await page.evaluate(() => JSON.parse(document.getElementById("page-data").textContent).data);
    await page.screenshot({ path: `out/g-${giro}-portada-${w}.png` });
    await page.screenshot({ path: `out/g-${giro}-portada-completa-${w}.png`, fullPage: true });
    await audit(page, `${giro} portada`, w);
    await page.locator("#catalogo").scrollIntoViewIfNeeded();
    await page.evaluate(() => { document.getElementById("catalogo").scrollIntoView({ block: "start" }); scrollBy(0, -130); });
    await page.waitForTimeout(300);
    await page.screenshot({ path: `out/g-${giro}-catalogo-${w}.png` });

    // Buscar: el buscador del catálogo encuentra por nombre.
    const term = data.cards[0].name.split(" ")[0];
    await page.locator("#q").fill(term);
    const res = await page.locator("#todo").textContent();
    if (!/resultado/.test(res)) bad(`${giro} ${w}: buscar "${term}" no dio resultados (${res})`);
    await page.locator("#q").fill("");

    // Categoría: la pestaña lleva a su catálogo.
    const cat = data.shop.categories[0];
    await page.goto(`${BASE}/`);
    await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
    await page.locator(`nav[aria-label="Categorías"] a[href="/c/${cat.id}"]`).click();
    await page.waitForURL(`${BASE}/c/${cat.id}`);
    await settle(page);
    const title = await page.locator("#cat-title").textContent();
    if (!title.includes(cat.name.split(" ")[0])) bad(`${giro} ${w}: la categoría muestra "${title}" y no "${cat.name}"`);
    await page.screenshot({ path: `out/g-${giro}-categoria-${w}.png`, fullPage: true });
    await audit(page, `${giro} categoría`, w);

    // Ficha con tallas: agregar sin elegir, elegir, carrito, quitar.
    const withSizes = data.cards.find((c) => c.sizes.length > 0 && !c.soldOut) ?? data.cards[0];
    await page.goto(`${BASE}/p/${withSizes.id}`);
    await settle(page);
    await page.screenshot({ path: `out/g-${giro}-ficha-${w}.png`, fullPage: true });
    await audit(page, `${giro} ficha`, w);
    if (withSizes.sizes.length) {
      await page.getByRole("button", { name: "Agregar al carrito" }).click();
      const msg = await page.locator("#buy .status").textContent();
      if (!/Elige/.test(msg)) bad(`${giro} ${w}: sin talla no avisó (${msg})`);
      for (const name of ["talla", "color"]) {
        const opt = page.locator(`input[name="${name}"]:not(:disabled)`).first();
        if (await opt.count()) await opt.check({ force: true });
      }
      // Si la combinación está agotada, prueba otra talla.
      for (let i = 0; i < 6 && (await page.locator("#buy .warn").count()); i++) {
        await page.locator('input[name="talla"]:not(:disabled)').nth(i).check({ force: true });
      }
    }
    await page.getByRole("button", { name: "Agregar al carrito" }).click();
    const cart = page.locator("dialog.cart");
    await cart.waitFor({ state: "visible" });
    await page.locator(".cart-foot").waitFor();
    const total = await page.locator(".total strong").textContent();
    const line = await page.locator(".cart-price").first().textContent();
    if (total !== line) bad(`${giro} ${w}: total ${total} distinto del renglón ${line}`);
    const note = await page.locator(".cart-foot p").first().textContent();
    if (!/aquí no pagas nada/.test(note)) bad(`${giro} ${w}: el carrito no aclara que no se paga aquí`);
    // El pedido se prepara con nombre (lo revisa pedido-check.mjs con el servidor simulado); aquí no se registra nada.
    if (!(await page.getByLabel("Tu nombre para el pedido").count())) bad(`${giro} ${w}: falta el nombre para el pedido`);
    if (w < 400) await page.screenshot({ path: `out/g-${giro}-carrito-${w}.png` });
    await page.getByRole("button", { name: /^Quitar .* del carrito$/ }).click();
    await page.getByText("Tu carrito está vacío").waitFor();
    await page.keyboard.press("Escape");

    // Teclado: el primer Tab llega al salto al contenido y se ve el foco.
    await page.goto(`${BASE}/`);
    await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
    await page.keyboard.press("Tab");
    const skip = await page.evaluate(() => document.activeElement?.className === "skip" && getComputedStyle(document.activeElement).insetBlockStart !== "-160px");
    if (!skip) bad(`${giro} ${w}: el primer Tab no muestra "Ir al contenido"`);

    if (errors.length) bad(`${giro} ${w}: consola: ${[...new Set(errors)].slice(0, 3).join(" | ")}`);
    await ctx.close();
  }
  console.log(giro, "listo");
}

// Vista previa del panel: misma portada sin guardar el estilo.
{
  const ctx = await b.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/panel/login`);
  await page.getByLabel("Email").fill(EMAIL);
  await page.getByLabel("Contraseña").fill(PASS);
  await page.getByRole("button", { name: "Entrar" }).click();
  await page.waitForURL(`${BASE}/panel/`);
  for (const giro of giros) {
    await page.goto(`${BASE}/panel/tienda/vista?estilo=${giro}`);
    await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
    const got = await page.evaluate(() => document.documentElement.dataset.astryxTheme);
    if (got !== giro) bad(`vista previa ${giro}: salió ${got}`);
    await page.waitForTimeout(500);
    await page.screenshot({ path: `out/g-${giro}-vistaprevia-1440.png` });
  }
  await ctx.close();
}
await b.close();
console.log(failed ? "HAY FALLAS" : "todo bien");
process.exit(failed ? 1 : 0);
