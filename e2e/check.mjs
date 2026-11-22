// Revisión en móvil (360 px): recorre el panel como una vendedora, toma capturas, corre axe-core y Lighthouse.
// Uso: docker compose up app; cd e2e; npm run check
// Sale con código 1 si hay violaciones de axe o accesibilidad de Lighthouse menor a 100.
import { chromium, devices } from "playwright";
import { AxeBuilder } from "@axe-core/playwright";
import lighthouse from "lighthouse";
import { mkdirSync, writeFileSync } from "node:fs";

const BASE = process.env.BASE ?? "http://ana.localhost:8080";
const EMAIL = process.env.EMAIL ?? "ana@demo.mx";
const PASS = process.env.PASS ?? "demo-ana-2026";
const PORT = 9333;
mkdirSync("out", { recursive: true });

const browser = await chromium.launch({ args: [`--remote-debugging-port=${PORT}`] });
const ctx = await browser.newContext({ ...devices["Pixel 7"], viewport: { width: 360, height: 800 }, locale: "es-MX" });
const page = await ctx.newPage();
const results = [];
let failed = false;

async function audit(name, url, { auth = true } = {}) {
  await page.goto(url);
  await page.screenshot({ path: `out/${name}.png`, fullPage: true });
  const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"]).analyze();
  for (const v of axe.violations) {
    failed = true;
    console.log(`  axe [${name}] ${v.id} (${v.impact}): ${v.help}`);
    for (const n of v.nodes.slice(0, 3)) console.log(`      ${n.target.join(" ")}`);
  }
  const cookie = auth ? (await ctx.cookies(url)).map((c) => `${c.name}=${c.value}`).join("; ") : "";
  const lh = await lighthouse(url, {
    port: PORT,
    output: "html",
    logLevel: "error",
    onlyCategories: ["accessibility", "best-practices", "seo", "performance"],
    extraHeaders: cookie ? { Cookie: cookie } : undefined,
  });
  writeFileSync(`out/${name}.lighthouse.html`, lh.report);
  const s = Object.fromEntries(Object.entries(lh.lhr.categories).map(([k, c]) => [k, Math.round(c.score * 100)]));
  if (s.accessibility < 100) {
    failed = true;
    for (const a of Object.values(lh.lhr.audits)) {
      if (a.score !== null && a.score < 1 && lh.lhr.categories.accessibility.auditRefs.some((r) => r.id === a.id && r.weight > 0)) {
        console.log(`  lighthouse [${name}] ${a.id}: ${a.title}`);
      }
    }
  }
  results.push({ página: name, axe: axe.violations.length, ...s });
}

// Sin sesión
await audit("login", `${BASE}/panel/login`, { auth: false });
await audit("tienda", `${BASE}/`, { auth: false });
await audit("recuperar", `${BASE}/panel/recuperar`, { auth: false });
await audit("admin-login", BASE.replace("ana.", "admin.") + "/login", { auth: false });

// Entrar
await page.goto(`${BASE}/panel/login`);
await page.getByLabel("Email").fill(EMAIL);
await page.getByLabel("Contraseña").fill(PASS);
await page.getByRole("button", { name: "Entrar" }).click();
await page.waitForURL(`${BASE}/panel/`);

// Datos de ejemplo (solo si no existen)
await page.goto(`${BASE}/panel/categorias`);
if (!(await page.getByRole("textbox", { name: "Nombre de la categoría" }).count())) {
  for (const c of ["Blusas", "Vestidos"]) {
    await page.getByLabel("Nueva categoría").fill(c);
    await page.getByRole("button", { name: "Agregar" }).click();
  }
}
await page.goto(`${BASE}/panel/`);
if (!(await page.getByRole("link", { name: "Blusa lino" }).count())) {
  await page.getByRole("link", { name: "Agregar producto" }).click();
  await page.getByLabel("Nombre").fill("Blusa lino");
  await page.getByLabel("Precio", { exact: true }).fill("289");
  await page.getByLabel("Categoría").selectOption({ label: "Blusas" });
  await page.getByLabel("Tallas").fill("S, M, L");
  await page.getByLabel("Colores").fill("Blanco, Negro");
  await page.getByLabel("Descripción").fill("Blusa de lino fresco.\nIdeal para calor.");
  await page.getByRole("button", { name: "Guardar producto" }).click();
  await page.getByRole("status").filter({ hasText: "Producto guardado" }).waitFor();
}
await page.goto(`${BASE}/panel/`);
const editURL = await page.getByRole("link", { name: "Blusa lino" }).getAttribute("href");

// Subir una foto (generada en el navegador) si el producto no tiene; pasa por la compresión de fotos.js.
await page.goto(`${BASE}${editURL}`);
if (!(await page.getByRole("img", { name: /Foto 1 de/ }).count())) {
  const bytes = await page.evaluate(async () => {
    const c = new OffscreenCanvas(1200, 1500);
    const g = c.getContext("2d");
    const grad = g.createLinearGradient(0, 0, 1200, 1500);
    grad.addColorStop(0, "#e6a5b8");
    grad.addColorStop(1, "#7a2f4b");
    g.fillStyle = grad;
    g.fillRect(0, 0, 1200, 1500);
    return [...new Uint8Array(await (await c.convertToBlob({ type: "image/png" })).arrayBuffer())];
  });
  await page.getByLabel("Agregar foto").setInputFiles({ name: "blusa.png", mimeType: "image/png", buffer: Buffer.from(bytes) });
  await page.getByRole("button", { name: "Subir foto" }).click();
  await page.getByRole("status").filter({ hasText: "Foto agregada" }).waitFor({ timeout: 60_000 });
}

await audit("productos", `${BASE}/panel/`);
await audit("producto-editar", `${BASE}${editURL}`);
await audit("producto-nuevo", `${BASE}/panel/productos/nuevo`);
await audit("categorias", `${BASE}/panel/categorias`);

console.table(results);
console.log(`Capturas y reportes en e2e/out/`);
await browser.close();
process.exit(failed ? 1 : 0);
