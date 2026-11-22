// Carrito en localStorage. Precios y disponibilidad siempre salen de /carrito.json (datos del servidor);
// lo guardado en el celular solo dice qué y cuánto, y se valida al leer.
const KEY = "carrito";
const MAX = 20;
const fmt = new Intl.NumberFormat("es-MX", { style: "currency", currency: "MXN" });
const money = (c) => fmt.format(c / 100);
const $ = (s, el = document) => el.querySelector(s);

const dlg = $("#carrito");
const ui = {
  list: $("[data-cart-list]", dlg), empty: $("[data-cart-empty]", dlg), foot: $("[data-cart-foot]", dlg),
  total: $("[data-cart-total]", dlg), wa: $("[data-cart-wa]", dlg), nowa: $("[data-cart-nowa]", dlg),
  status: $("[data-cart-status]", dlg),
};

const ok = (i) =>
  i && Number.isSafeInteger(i.id) && i.id > 0 && typeof i.size === "string" && i.size.length <= 20 &&
  typeof i.color === "string" && i.color.length <= 30 && Number.isInteger(i.qty) && i.qty >= 1 && i.qty <= MAX &&
  Number.isInteger(i.price) && i.price >= 0;

function load() {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) || "[]");
    return Array.isArray(v) ? v.filter(ok).slice(0, 50) : [];
  } catch {
    return [];
  }
}

function save(items) {
  try {
    localStorage.setItem(KEY, JSON.stringify(items));
  } catch {
    // Modo privado o sin espacio: el carrito vive solo en esta página.
  }
  count(items);
}

function count(items) {
  const n = items.reduce((s, i) => s + i.qty, 0);
  for (const el of document.querySelectorAll("[data-cart-count]")) el.textContent = n;
}

let catalog;
async function getCatalog() {
  if (!catalog) {
    const r = await fetch("/carrito.json", { headers: { Accept: "application/json" } });
    if (!r.ok) throw new Error(r.status);
    catalog = await r.json();
  }
  return catalog;
}

const el = (tag, props = {}, ...kids) => {
  const e = Object.assign(document.createElement(tag), props);
  e.append(...kids);
  return e;
};

async function render() {
  const items = load();
  ui.list.replaceChildren();
  ui.status.replaceChildren();
  ui.empty.hidden = items.length > 0;
  ui.foot.hidden = true;
  if (!items.length) return;

  let cat;
  try {
    cat = await getCatalog();
  } catch {
    const retry = el("button", { type: "button", className: "link", textContent: "Reintentar" });
    retry.onclick = () => { catalog = null; render(); };
    ui.status.append(el("p", { className: "warn", textContent: "No pudimos cargar tu carrito. Revisa tu conexión y vuelve a intentar." }), retry);
    return;
  }

  let total = 0, changed = false;
  const lines = [];
  items.forEach((it, i) => {
    const p = cat[it.id];
    const combo = it.size + "|" + it.color;
    const hasCombos = p && Object.keys(p.v).length > 0;
    let warn = "";
    if (!p) warn = "Este producto ya no está disponible.";
    else if (p.x || (hasCombos && p.v[combo] !== false)) warn = "Se agotó.";
    else if (p.p !== it.price) {
      warn = `Cambió de precio: antes ${money(it.price)}, ahora ${money(p.p)}.`;
      it.price = p.p;
      changed = true;
    }
    const name = p ? p.n : "Producto";
    const detail = [it.size && `Talla ${it.size}`, it.color].filter(Boolean).join(" · ");
    const available = p && !(p.x || (hasCombos && p.v[combo] !== false));
    const sub = available ? p.p * it.qty : 0;
    if (available) {
      total += sub;
      lines.push(`• ${it.qty} × ${name}${detail ? ` (${detail})` : ""}: ${money(sub)}\n  ${location.origin}${p.u}`);
    }

    const li = el("li");
    li.append(el("div", { className: "row" },
      p ? el("a", { href: p.u, textContent: name }) : el("span", { textContent: name }),
      el("span", { textContent: available ? money(sub) : "" })));
    if (detail) li.append(el("span", { className: "muted", textContent: detail }));
    if (warn) li.append(el("span", { className: "warn", textContent: warn }));
    const step = el("div", { className: "step" });
    if (available) {
      step.append(
        el("button", { type: "button", textContent: "−", ariaLabel: `Quitar uno de ${name}` }),
        el("span", { textContent: it.qty, ariaLabel: `Cantidad: ${it.qty}` }),
        el("button", { type: "button", textContent: "+", ariaLabel: `Agregar uno de ${name}` }));
      step.children[0].onclick = () => update(i, -1);
      step.children[2].onclick = () => update(i, +1);
      step.children[2].disabled = it.qty >= MAX;
    }
    const del = el("button", { type: "button", className: "link", textContent: "Quitar" });
    del.append(el("span", { className: "sr-only", textContent: ` ${name}` }));
    del.onclick = () => update(i, -Infinity);
    li.append(el("div", { className: "row" }, step, del));
    ui.list.append(li);
  });
  if (changed) save(items);

  ui.foot.hidden = lines.length === 0;
  ui.total.textContent = money(total);
  const wa = dlg.dataset.wa;
  ui.wa.hidden = !wa;
  ui.nowa.hidden = !!wa;
  if (wa) {
    const msg = `Hola, quiero hacer este pedido en ${dlg.dataset.tienda}:\n\n${lines.join("\n")}\n\nTotal: ${money(total)}`;
    ui.wa.href = `https://wa.me/${encodeURIComponent(wa)}?text=${encodeURIComponent(msg)}`;
  }
}

function update(i, delta) {
  const items = load();
  if (!items[i]) return;
  items[i].qty = Math.min(MAX, items[i].qty + delta);
  if (!(items[i].qty >= 1)) items.splice(i, 1);
  save(items);
  render();
}

function open() {
  if (!dlg.open) dlg.showModal();
  render();
}

for (const b of document.querySelectorAll("[data-cart-open]")) {
  b.hidden = false;
  b.addEventListener("click", open);
}

for (const f of document.querySelectorAll("form[data-add]")) {
  f.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const d = new FormData(f);
    const size = String(d.get("talla") || ""), color = String(d.get("color") || "");
    const status = $("[data-add-status]", f);
    let sold = [];
    try { sold = JSON.parse(f.dataset.agotadas); } catch {}
    if (sold.includes(size + "|" + color)) {
      status.textContent = "Esa combinación está agotada. Elige otra talla o color.";
      return;
    }
    const qty = Math.min(MAX, Math.max(1, parseInt(d.get("cantidad"), 10) || 1));
    const item = { id: Number(f.dataset.id), size, color, qty, price: Number(f.dataset.price) };
    const items = load();
    const same = items.find((i) => i.id === item.id && i.size === size && i.color === color);
    if (same) same.qty = Math.min(MAX, same.qty + qty);
    else items.push(item);
    save(items);
    status.textContent = "Agregado al carrito.";
    open();
  });
}

count(load());
