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

// WhatsApp de la tienda (número de prueba que no existe) y "Mi tienda".
await page.goto(`${BASE}/panel/tienda`);
await page.getByLabel("WhatsApp para recibir pedidos").fill("55 0000 0000");
// Gris medio: no llega a 4.5:1 ni con blanco ni con tinta, así que el servidor debe ajustarlo y avisar.
await page.getByLabel("Color de tu marca").fill("#7f7f7f");
await page.getByLabel("Datos para pagar por transferencia (opcional)").fill("BBVA\nCLABE 012 180 00000000000 0\nA nombre de Ana Demo");
await page.getByLabel("Tus preguntas frecuentes (opcional)").fill("¿Hacen envíos?\nSí, a todo México por paquetería.\n\n¿Apartan?\nSí, con el 50 %.");
await page.getByRole("button", { name: "Guardar datos" }).click();
await page.getByRole("status").filter({ hasText: "Datos guardados" }).waitFor();
if (!(await page.getByText("Ajustamos un poco tu color").count())) {
  failed = true;
  console.log("  mi tienda: no avisó el ajuste de color");
}
await page.screenshot({ path: "out/mi-tienda-ajuste.png", fullPage: true });
await page.getByLabel("Color de tu marca").fill("#c2185b");
await page.getByRole("button", { name: "Guardar datos" }).click();
await page.getByRole("status").filter({ hasText: "Datos guardados" }).waitFor();
const preview = page.frameLocator('iframe[title="Vista previa de tu tienda"]');
await preview.getByRole("heading", { level: 1 }).waitFor();
await audit("mi-tienda", `${BASE}/panel/tienda`);

// Tienda pública (plantilla Vitrina)
await audit("tienda", `${BASE}/`, { auth: false });
await page.goto(`${BASE}/`);
const catURL = await page.getByRole("navigation", { name: "Categorías" }).getByRole("link").nth(1).getAttribute("href");
const prodURL = await page.locator("a.card:not(.sold)").first().getAttribute("href");
await audit("tienda-categoria", `${BASE}${catURL}`, { auth: false });
await audit("tienda-producto", `${BASE}${prodURL}`, { auth: false });
await audit("tienda-preguntas", `${BASE}/preguntas`, { auth: false });
await page.goto(`${BASE}/preguntas`);
for (const t of ["¿Hacen envíos?", "CLABE"]) {
  if (!(await page.getByText(t).count())) {
    failed = true;
    console.log(`  preguntas: falta "${t}"`);
  }
}
const og = await page.goto(`${BASE}${prodURL}`).then(() => page.locator('meta[property="og:image"]').getAttribute("content"));
if (!og?.startsWith("http")) {
  failed = true;
  console.log(`  producto: og:image no es absoluta: ${og}`);
}

// Carrito: elegir talla y color disponibles, agregar, revisar el mensaje de WhatsApp.
await page.goto(`${BASE}${prodURL}`);
const name = (await page.getByRole("heading", { level: 1 }).textContent()).trim();
for (const group of ["Talla", "Color"]) {
  const radios = page.getByRole("group", { name: group }).getByRole("radio");
  for (let i = 0; i < (await radios.count()); i++) {
    if (await radios.nth(i).isEnabled()) {
      await radios.nth(i).check({ force: true });
      if (!(await page.getByText("Esa combinación está agotada").count())) break;
    }
  }
}
await page.getByRole("button", { name: "Agregar al carrito" }).click();
const dialog = page.getByRole("dialog", { name: "Tu carrito" });
await dialog.waitFor();
const wa = dialog.getByRole("link", { name: "Enviar pedido por WhatsApp" });
await wa.waitFor();
const href = await wa.getAttribute("href");
const msg = decodeURIComponent(href.split("?text=")[1]);
console.log(`Mensaje de WhatsApp:\n${msg}`);
if (!href.startsWith("https://wa.me/525500000000?text=") || !msg.includes(name) || !msg.includes("Total: $") || !msg.includes(`${BASE}/p/`)) {
  failed = true;
  console.log("  carrito: mensaje de WhatsApp mal formado");
}
await page.waitForTimeout(400); // deja terminar la animación del cajón
await page.screenshot({ path: "out/carrito.png" });
const axeCart = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"]).analyze();
for (const v of axeCart.violations) {
  failed = true;
  console.log(`  axe [carrito] ${v.id}: ${v.help} ${v.nodes.map((n) => n.target.join(" ")).slice(0, 3)}`);
}
results.push({ página: "carrito (abierto)", axe: axeCart.violations.length });

console.table(results);
console.log(`Capturas y reportes en e2e/out/`);
await browser.close();
process.exit(failed ? 1 : 0);
