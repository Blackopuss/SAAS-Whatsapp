// Presupuesto de peso de la tienda: lo que baja una compradora en su primera visita (gzip -9).
// JS de shop.js y todo lo que importa, shop.css, el tema más pesado y la fuente más pesada (cada tienda carga una).
// Uso: node scripts/budget.mjs (después de vite build). Sale con error si se pasa.
import { readFileSync, readdirSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { join, dirname } from "node:path";

const LIMIT = 200 * 1024; // subido de 130 a 200 KB (2026-11-22) para que cada giro tenga su diseño
const app = new URL("../../web/static/app/", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");
const fonts = join(app, "../fonts");
const gz = (f) => gzipSync(readFileSync(f), { level: 9 }).length;

const seen = new Set();
(function walk(f) {
  if (seen.has(f)) return;
  seen.add(f);
  for (const m of readFileSync(f, "utf8").matchAll(/(?:import|from)\s*["'](\.\/[^"']+\.js)["']/g)) walk(join(dirname(f), m[1]));
})(join(app, "shop.js"));

const js = [...seen].reduce((s, f) => s + gz(f), 0);
const css = gz(join(app, "shop.css"));
const theme = Math.max(...readdirSync(app).filter((f) => /^theme-.*\.css$/.test(f)).map((f) => gz(join(app, f))));
const font = Math.max(...readdirSync(fonts).filter((f) => f.endsWith(".woff2")).map((f) => readFileSync(join(fonts, f)).length));
const total = js + css + theme + font;
const kb = (n) => (n / 1024).toFixed(1) + " KB";
console.log(`js=${kb(js)} (${seen.size} archivos) css=${kb(css)} tema=${kb(theme)} fuente=${kb(font)} total=${kb(total)} límite=${kb(LIMIT)}`);
if (total > LIMIT) {
  console.error("La tienda se pasó del presupuesto de peso.");
  process.exit(1);
}
