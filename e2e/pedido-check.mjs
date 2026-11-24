// Nombre para el pedido y mensaje de WhatsApp, sin escribir en la base: POST /pedido lo contesta un servidor simulado
// y wa.me se bloquea. Revisa: el nombre se pide al preparar (no al agregar), sin nombre avisa y no manda nada,
// el nombre va limpio, el mensaje usa lo que "registró" el servidor (nombre e importes), no lleva enlaces,
// y "Pedir solo este" pasa por el mismo paso. Uso: node pedido-check.mjs → out/pedido-*.png
import { chromium } from "playwright";
import AxeBuilder from "@axe-core/playwright";

const BASE = process.env.BASE ?? "http://ana.localhost:8080";
const b = await chromium.launch();
let failed = false;
const bad = (m) => { failed = true; console.log("  ✗ " + m); };

const ctx = await b.newContext({ viewport: { width: 390, height: 844 }, locale: "es-MX", reducedMotion: "reduce" });
const page = await ctx.newPage();
const posts = [];
await page.route("**/pedido", async (route) => {
  const body = route.request().postDataJSON();
  posts.push(body);
  // El servidor simulado responde con otros importes para comprobar que el mensaje usa lo registrado.
  await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({
    code: body.code, name: body.name.trim().replace(/\s+/g, " "), total: 123456,
    items: [{ name: "Producto registrado", size: "M", color: "Negro", qty: 2000, unit: "pza", subtotal: 100000 },
      { name: "Queso", size: "", color: "", qty: 500, unit: "kg", subtotal: 23456 }] }) });
});
await page.route(/wa\.me|api\.whatsapp/, (route) => route.abort());

await page.goto(`${BASE}/`);
await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
await page.evaluate(() => { localStorage.removeItem("carrito-2"); localStorage.removeItem("nombre-pedido"); });
const data = await page.evaluate(() => JSON.parse(document.getElementById("page-data").textContent).data);
const plain = data.cards.find((c) => !c.sizes.length && !c.soldOut);
if (!plain) throw new Error("no hay producto sin tallas disponible");

// Agregar desde la ficha: abre el carrito; el nombre aparece solo al preparar el pedido (pie del carrito).
await page.goto(`${BASE}/p/${plain.id}`);
await page.waitForFunction(() => document.getElementById("root")?.childElementCount > 0);
if (await page.getByLabel("Tu nombre para el pedido").count()) bad("el nombre se pide en la ficha, antes de preparar el pedido");
await page.getByRole("button", { name: "Agregar al carrito" }).click();
const cart = page.getByRole("dialog", { name: "Tu carrito" });
const prep = cart.getByRole("button", { name: "Preparar pedido para WhatsApp" });
await prep.waitFor();
const inputs = await cart.locator("input:not([type=hidden])").count();
if (inputs !== 1) bad(`el carrito pide ${inputs} campos; solo debe pedir el nombre`);

// Sin nombre y con nombre inválido: avisa y no manda nada.
for (const [value, want] of [["", "Escribe tu nombre"], ["   ", "Escribe tu nombre"], ["A", "al menos 2"], ["=SUMA(1)", "Usa solo letras"]]) {
  await cart.getByLabel("Tu nombre para el pedido").fill(value);
  await prep.click();
  if (!(await cart.getByText(want).count())) bad(`nombre "${value}": no avisó "${want}"`);
  if ((await cart.getByLabel("Tu nombre para el pedido").getAttribute("aria-invalid")) !== "true") bad(`nombre "${value}": falta aria-invalid`);
}
if (posts.length) bad(`con nombre inválido se mandaron ${posts.length} pedidos`);
await page.screenshot({ path: "out/pedido-sin-nombre.png" });
if (await cart.getByLabel("Tu nombre para el pedido").getAttribute("maxlength") !== "60") bad("el nombre no tiene límite de 60");

// Con nombre: manda el nombre limpio y arma el mensaje con lo registrado.
await cart.getByLabel("Tu nombre para el pedido").fill("  María   José ");
await prep.click();
const wa = cart.getByRole("link", { name: "Enviar pedido por WhatsApp" });
await wa.waitFor();
if (posts.length !== 1 || posts[0].name !== "María José" || !posts[0].items?.length || !/^[A-HJ-NP-Z2-9]{6}$/.test(posts[0].code)) bad(`pedido enviado: ${JSON.stringify(posts[0])}`);
const msg = decodeURIComponent((await wa.getAttribute("href")).split("?text=")[1]);
console.log("Mensaje:\n" + msg + "\n");
for (const part of ["*María José*", `*${data.shop.name}*`, `*Pedido ${posts[0].code}*`, "2 × Producto registrado (Talla M · Negro)", "Subtotal: $1,000.00",
  "0.5 kg de Queso", "Subtotal: $234.56", "*Total: $1,234.56*", "pago y la entrega"]) {
  if (!msg.includes(part)) bad(`al mensaje le falta "${part}"`);
}
if (/https?:\/\/|\/p\/\d/.test(msg)) bad("el mensaje lleva enlaces");
if (!(await cart.getByText(`Pedido ${posts[0].code}`).count()) || !(await cart.getByText("$1,234.56").count())) bad("el carrito no muestra el pedido registrado");
await page.screenshot({ path: "out/pedido-listo.png" });
const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "best-practice"]).analyze();
for (const v of axe.violations) bad(`axe ${v.id}: ${v.help} → ${v.nodes.slice(0, 2).map((n) => n.target.join(" ")).join(" | ")}`);

// Cambiar el pedido vuelve al carrito editable; al preparar otra vez, el nombre ya está guardado.
await cart.getByRole("button", { name: "Cambiar mi pedido" }).click();
if ((await cart.getByLabel("Tu nombre para el pedido").inputValue()) !== "María José") bad("no recordó el nombre limpio");
await page.keyboard.press("Escape");

// "Pedir solo este" pasa por el mismo paso del nombre y no toca el carrito guardado.
const before = await page.evaluate(() => localStorage.getItem("carrito-2"));
await page.getByRole("button", { name: "Pedir solo este por WhatsApp" }).click();
const only = page.getByRole("dialog", { name: "Tu pedido" });
await only.getByRole("button", { name: "Preparar pedido para WhatsApp" }).click();
await only.getByRole("link", { name: "Enviar pedido por WhatsApp" }).waitFor();
if (posts.length !== 2 || posts[1].items.length !== 1 || posts[1].items[0].id !== plain.id) bad(`"Pedir solo este" mandó ${JSON.stringify(posts[1])}`);
if (posts[1].code === posts[0].code) bad("dos pedidos con el mismo código");
await only.getByRole("link", { name: "Enviar pedido por WhatsApp" }).evaluate((a) => a.addEventListener("click", (e) => e.preventDefault()));
await only.getByRole("link", { name: "Enviar pedido por WhatsApp" }).click();
await only.getByRole("button", { name: "Listo" }).click();
if ((await page.evaluate(() => localStorage.getItem("carrito-2"))) !== before) bad('"Pedir solo este" cambió el carrito guardado');

await b.close();
console.log(failed ? "HAY FALLAS" : "todo bien (sin escribir en la base ni abrir WhatsApp)");
process.exit(failed ? 1 : 0);
