// Tienda pública (<slug>.<dominio>). Datos desde views/store.templ.
// Sin componentes de Astryx a propósito: aquí pesaban ~190 KB y la compradora suele estar en un celular con datos.
// El aspecto sale del tema del estilo (tokens --color-*, --font-*, --radius-*) que Go pone en <html>.
import { Fragment, StrictMode, createContext, useContext, useEffect, useMemo, useRef, useState, type ComponentType, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { ShoppingBag, MessageCircle, Minus, Plus, Search, X, Share2, CreditCard, Truck, CheckCircle2, RotateCcw, ChevronRight, HelpCircle } from "lucide-react";
import "./shop.css";

type ShopInfo = {
  name: string; categories: { id: number; name: string }[]; current: number; home: boolean; whatsapp: string; logo: string;
  faq: { q: string; a: string }[]; payment: string; url: string; preview: boolean; style: string;
};
type Photo = { src: string; srcSet: string };
type CardData = {
  id: number; name: string; price: number; was: number; soldOut: boolean; new: boolean; photo: Photo; color: string; sizes: string[]; cat: number;
  unit: string; step: number; // unidad (pza, kg, g, L, m) y de cuánto en cuánto, en milésimas
  left?: number; colors?: { name: string; swatch: string }[]; // disponible en milésimas (-1 = sin inventario) y colores con existencia
};
type Opt = { name: string; soldOut: boolean; swatch: string };
type Product = {
  card: CardData; description: string; photos: Photo[]; categoryID: number; categoryName: string;
  sizes: Opt[]; colors: Opt[]; soldOutCombos: string[]; share: string; related: CardData[];
};

const fmt2 = new Intl.NumberFormat("es-MX", { style: "currency", currency: "MXN" });
const fmt0 = new Intl.NumberFormat("es-MX", { style: "currency", currency: "MXN", minimumFractionDigits: 0 });
const money = (c: number) => (c % 100 === 0 ? fmt0 : fmt2).format(c / 100);
const off = (c: CardData) => Math.round((1 - c.price / c.was) * 100);
const wa = (phone: string, text: string) => `https://wa.me/${encodeURIComponent(phone)}?text=${encodeURIComponent(text)}`;
const fold = (s: string) => s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();

// ——— Movimiento ———
// Un solo momento: el título de la portada (o de la categoría) entra una vez al cargar; lo demás solo responde al tacto (CSS).
const reduced = () => matchMedia("(prefers-reduced-motion: reduce)").matches;

// Giro de la tienda (vitrina, boutique, barberia, carniceria, abarrotes, papeleria): decide portada y detalles de tarjeta y ficha.
let giro = "vitrina";

// Título que sube palabra por palabra desde su renglón (CSS .w); 80 ms entre palabras, 240 ms como máximo.
function Rise({ text }: { text: string }) {
  return <>{text.split(/\s+/).filter(Boolean).map((w, i) => (
    <Fragment key={i}>{i > 0 && " "}<span className="w" style={{ "--d": `${Math.min(i, 3) * 80}ms`, "--len": w.length } as React.CSSProperties}>{w}</span></Fragment>
  ))}</>;
}

// ——— Carrito ———
// Vive en localStorage. Precios y existencias siempre salen de /carrito.json; lo guardado solo dice qué y cuánto.
// Cantidades en milésimas de la unidad: 1 pieza = 1000, 0.25 kg = 250 (igual que en la base).
type Item = { id: number; size: string; color: string; qty: number; price: number };
// s y v: disponible en milésimas; -1 = la tienda no lleva inventario de eso.
type CatProduct = { n: string; p: number; x: boolean; u: string; i: string; un: string; t: number; s: number; v: Record<string, number> };
type Catalog = Record<string, CatProduct>;
const KEY = "carrito-2"; // -2: antes las cantidades eran piezas, no milésimas
const LAST = "ultimo-pedido-2";
const MAX_UNITS = 99; // tope cuando la tienda no lleva inventario

const valid = (i: Item) =>
  i && Number.isSafeInteger(i.id) && i.id > 0 && typeof i.size === "string" && i.size.length <= 20 &&
  typeof i.color === "string" && i.color.length <= 30 && Number.isInteger(i.qty) && i.qty >= 1 && i.qty <= 1e9 &&
  Number.isInteger(i.price) && i.price >= 0;

const qtyNum = (milli: number) => String(+(milli / 1000).toFixed(3));
// "2" para piezas, "0.5 kg" para lo demás.
const qtyLabel = (milli: number, unit: string) => (unit === "pza" || !unit ? qtyNum(milli) : `${qtyNum(milli)} ${unit}`);
const lineTotal = (price: number, milli: number) => Math.round((price * milli) / 1000);

// Cuánto se puede pedir de un producto o combinación (milésimas); -1 = sin límite de inventario.
function availableOf(p: CatProduct, size: string, color: string) {
  if (p.x) return 0;
  const hasCombos = Object.keys(p.v).length > 0;
  if (!hasCombos) return p.s;
  return p.v[size + "|" + color] ?? 0;
}
const maxOf = (avail: number, step: number) => (avail < 0 ? step * MAX_UNITS : Math.floor(avail / step) * step);

// Código del pedido: lo ve la vendedora en el mensaje y en su panel. Sin I, O, 0 ni 1 para no confundirlos.
function newCode() {
  const abc = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  return Array.from(crypto.getRandomValues(new Uint8Array(6)), (b) => abc[b % abc.length]).join("");
}

function read(key: string): Item[] {
  try {
    const v = JSON.parse(localStorage.getItem(key) || "[]");
    return Array.isArray(v) ? v.filter(valid).slice(0, 50) : [];
  } catch {
    return [];
  }
}

function write(key: string, items: Item[]) {
  try {
    localStorage.setItem(key, JSON.stringify(items));
  } catch {
    // Modo privado o sin espacio: el carrito vive solo en esta página.
  }
}

function merge(into: Item[], add: Item[]) {
  for (const it of add) {
    const same = into.find((i) => i.id === it.id && i.size === it.size && i.color === it.color);
    if (same) same.qty += it.qty; // el tope por existencia lo aplica el carrito con /carrito.json
    else into.push({ ...it });
  }
  return into;
}

let catalog: Promise<Catalog> | null = null;
const getCatalog = () =>
  (catalog ??= fetch("/carrito.json", { headers: { Accept: "application/json" } })
    .then((r) => {
      if (!r.ok) throw new Error(String(r.status));
      return r.json();
    })
    .catch((e) => {
      catalog = null;
      throw e;
    }));

const detail = (it: { size: string; color: string }) => [it.size && `Talla ${it.size}`, it.color].filter(Boolean).join(" · ");

// Pedido tal como lo registró el servidor (POST /pedido): con esto se arma el mensaje, así el nombre y los importes
// son los mismos que ve la vendedora en Ventas.
type Placed = { code: string; name: string; total: number; items: { name: string; size: string; color: string; qty: number; unit: string; subtotal: number }[] };
type PlaceResult = { ok: true; order: Placed } | { ok: false; error: string };

async function placeOrder(code: string, name: string, items: { id: number; size: string; color: string; qty: number }[]): Promise<PlaceResult> {
  try {
    const r = await fetch("/pedido", { method: "POST", headers: { "Content-Type": "application/json", Accept: "application/json" }, body: JSON.stringify({ code, name, items }) });
    if (r.ok) return { ok: true, order: await r.json() };
    const text = (await r.text()).trim();
    return { ok: false, error: r.status >= 500 || !text ? "No pudimos registrar tu pedido. Inténtalo otra vez." : text };
  } catch {
    return { ok: false, error: "No pudimos registrar tu pedido. Revisa tu conexión e inténtalo otra vez." };
  }
}

// Nombre para el pedido: igual que en el servidor (customerName en orders.go). Solo el nombre; nada de teléfono ni dirección.
const NAME_KEY = "nombre-pedido";
const MAX_NAME = 60;
const cleanName = (s: string) => s.trim().split(/\s+/).filter(Boolean).join(" ");
function nameError(s: string) {
  const n = cleanName(s);
  if (!n) return "Escribe tu nombre para que la tienda sepa de quién es el pedido.";
  if ([...n].length < 2) return "Escribe al menos 2 letras.";
  if ([...n].length > MAX_NAME) return `Usa ${MAX_NAME} caracteres o menos.`;
  if (!/^[\p{L}\p{M}\p{N}][\p{L}\p{M}\p{N} '.\-]*$/u.test(n)) return "Usa solo letras, números, espacios, puntos, guiones o apóstrofos.";
  return "";
}
const savedName = () => { try { return localStorage.getItem(NAME_KEY) ?? ""; } catch { return ""; } };

// Mensaje fácil de leer en el teléfono (negritas de WhatsApp con *); sin enlaces: el código basta para hallarlo en Ventas.
const plain = (s: string) => s.replace(/[*_~`]/g, "");
function orderText(shop: string, o: Placed) {
  const lines = o.items.map((it) => {
    const qty = it.unit === "pza" || !it.unit ? `${qtyNum(it.qty)} ×` : `${qtyLabel(it.qty, it.unit)} de`;
    const d = detail(it);
    return `• ${qty} ${plain(it.name)}${d ? ` (${plain(d)})` : ""}\n   Subtotal: ${fmt2.format(it.subtotal / 100)}`;
  });
  return [
    `Hola, soy *${plain(o.name)}*. Quiero hacer este pedido en *${plain(shop)}*:`,
    `*Pedido ${o.code}*`,
    lines.join("\n\n"),
    `*Total: ${fmt2.format(o.total / 100)}*`,
    "¿Cómo quedan el pago y la entrega? Gracias.",
  ].join("\n\n");
}

type CartAPI = { items: Item[]; add: (i: Item[]) => void; open: () => void; orderOnly: (i: Item[]) => void };
const CartCtx = createContext<CartAPI>({ items: [], add: () => {}, open: () => {}, orderOnly: () => {} });

// only: "Pedir solo este" desde la ficha; ese pedido no toca el carrito guardado.
function Cart({ shop, items, save, open, setOpen, only }: { shop: ShopInfo; items: Item[]; save: (i: Item[]) => void; open: boolean; setOpen: (o: boolean) => void; only: boolean }) {
  const ref = useRef<HTMLDialogElement>(null);
  const [cat, setCat] = useState<Catalog | null>(null);
  const [failed, setFailed] = useState(false);
  const [sent, setSent] = useState(false);
  const [code, setCode] = useState(newCode);
  const [name, setName] = useState(savedName);
  const [nameErr, setNameErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [placeErr, setPlaceErr] = useState("");
  const [placed, setPlaced] = useState<Placed | null>(null);
  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  useEffect(() => {
    if (!open || !items.length) return;
    getCatalog().then(setCat, () => setFailed(true));
  }, [open, items.length, failed]);

  // Guarda el precio nuevo para que el aviso salga una sola vez.
  useEffect(() => {
    if (!cat || only) return;
    let changed = false;
    const next = read(KEY).map((it) => {
      const p = cat[it.id];
      if (p && p.p !== it.price) {
        changed = true;
        return { ...it, price: p.p };
      }
      return it;
    });
    if (changed) write(KEY, next);
  }, [cat, only]);

  // dir: +1 / -1 paso; -Infinity quita el renglón.
  const update = (i: number, dir: number, step: number, max: number) => {
    const next = items.map((it) => ({ ...it }));
    if (!next[i]) return;
    next[i].qty = Math.min(max, next[i].qty + dir * step);
    if (!(next[i].qty >= step)) next.splice(i, 1);
    save(next);
  };

  let total = 0;
  const order: { id: number; size: string; color: string; qty: number }[] = [];
  const rows = cat
    ? items.map((it, i) => {
        const p = cat[it.id];
        const step = p?.t || 1000;
        const max = p ? maxOf(availableOf(p, it.size, it.color), step) : 0;
        // Se ajusta a lo que hay y al paso de venta (si cambió desde que lo agregó).
        const qty = Math.min(max, Math.floor(it.qty / step) * step);
        const available = !!p && qty > 0;
        let warn = "";
        if (!p) warn = "Este producto ya no está disponible.";
        else if (!available) warn = "Se agotó.";
        else if (qty < it.qty) warn = `Solo hay ${qtyLabel(max, p.un)}; ajustamos la cantidad.`;
        else if (p.p !== it.price) warn = `Cambió de precio: antes ${money(it.price)}, ahora ${money(p.p)}.`;
        const sub = available ? lineTotal(p.p, qty) : 0;
        if (available) {
          total += sub;
          order.push({ id: it.id, size: it.size, color: it.color, qty });
        }
        return { i, it, p, qty, step, max, name: p ? p.n : "Producto", sub, available, warn };
      })
    : [];
  const count = items.length;

  // Al cambiar el carrito, lo registrado ya no corresponde: se vuelve a preparar con otro código.
  const key = JSON.stringify(order);
  const lastKey = useRef(key);
  useEffect(() => {
    if (key === lastKey.current) return;
    lastKey.current = key;
    if (placed) { setPlaced(null); setCode(newCode()); }
  }, [key, placed]);

  async function prepare() {
    const err = nameError(name);
    setNameErr(err);
    if (err) return nameRef.current?.focus();
    const clean = cleanName(name);
    setName(clean);
    try { localStorage.setItem(NAME_KEY, clean); } catch { /* sin espacio: se vuelve a pedir */ }
    setBusy(true);
    setPlaceErr("");
    const res = await placeOrder(code, clean, order);
    setBusy(false);
    if (res.ok) setPlaced(res.order);
    else {
      setPlaceErr(res.error);
      catalog = null; // precios o existencias pudieron cambiar: se vuelven a pedir
      getCatalog().then(setCat, () => setFailed(true));
    }
  }

  function reset() {
    setPlaced(null);
    setCode(newCode());
  }

  function done() {
    if (!only) write(KEY, []);
    save([]);
    setSent(false);
    setPlaced(null);
    setOpen(false);
  }

  const close = () => { setOpen(false); setSent(false); };

  return (
    <dialog ref={ref} className="cart" aria-labelledby="cart-title" onClose={close}
      onClick={(e) => { if (e.target === e.currentTarget) setOpen(false); }}>
      <div className="cart-in">
        <header className="cart-head">
          <h2 id="cart-title">{only ? "Tu pedido" : "Tu carrito"}</h2>
          {count > 0 && !sent && <span className="muted">{count} {count === 1 ? "producto" : "productos"}</span>}
          <button type="button" className="icon-btn" aria-label={only ? "Cerrar pedido" : "Cerrar carrito"} onClick={() => setOpen(false)}><X aria-hidden size={20} /></button>
        </header>

        {sent ? (
          <div className="cart-sent">
            <CheckCircle2 aria-hidden size={44} className="ok-icon" />
            <h3>Se abrió WhatsApp con tu pedido</h3>
            <p>Si ya lo enviaste, {shop.name} te responde por ahí para acordar el pago y la entrega.</p>
            <button type="button" className="btn primary block" onClick={done}>{only ? "Listo" : "Ya lo envié, vaciar carrito"}</button>
            <button type="button" className="btn ghost block" onClick={() => setSent(false)}>Todavía no, volver</button>
          </div>
        ) : items.length === 0 ? (
          <div className="empty">
            <ShoppingBag aria-hidden size={40} />
            <h3>Tu carrito está vacío</h3>
            <p>Elige algo que te guste; aquí lo juntas y lo mandas en un solo mensaje.</p>
            <a className="btn primary" href="/">Ver productos</a>
          </div>
        ) : failed ? (
          <div className="empty" role="alert">
            <h3>No pudimos cargar tu carrito</h3>
            <p>Revisa tu conexión e inténtalo otra vez.</p>
            <button type="button" className="btn primary" onClick={() => setFailed(false)}>Reintentar</button>
          </div>
        ) : !cat ? (
          <p className="muted pad" role="status">Cargando tu carrito…</p>
        ) : (
          <>
            <ul className="cart-list">
              {rows.map((r) => (
                <li key={r.i} className="cart-row">
                  <span className="thumb">{r.p?.i && <img src={r.p.i} alt="" width={64} height={80} loading="lazy" />}</span>
                  <div className="cart-info">
                    {r.p ? <a href={r.p.u} className="cart-name">{r.name}</a> : <span className="cart-name">{r.name}</span>}
                    {detail(r.it) && <span className="muted small">{detail(r.it)}</span>}
                    {r.warn && <span className="warn small">{r.warn}</span>}
                    {!placed && (
                      <div className="cart-actions">
                        {r.available && (
                          <span className="stepper">
                            <button type="button" aria-label={`Quitar ${qtyLabel(r.step, r.p?.un ?? "")} de ${r.name}`} onClick={() => update(r.i, -1, r.step, r.max)}><Minus aria-hidden size={14} /></button>
                            <span aria-label={`Cantidad: ${qtyLabel(r.qty, r.p?.un ?? "")}`}>{qtyLabel(r.qty, r.p?.un ?? "")}</span>
                            <button type="button" aria-label={`Agregar ${qtyLabel(r.step, r.p?.un ?? "")} de ${r.name}`} disabled={r.qty >= r.max} onClick={() => update(r.i, 1, r.step, r.max)}><Plus aria-hidden size={14} /></button>
                          </span>
                        )}
                        <button type="button" className="link-btn" aria-label={`Quitar ${r.name} del ${only ? "pedido" : "carrito"}`} onClick={() => update(r.i, -Infinity, r.step, r.max)}>Quitar</button>
                      </div>
                    )}
                    {placed && <span className="muted small">{qtyLabel(r.qty, r.p?.un ?? "")}</span>}
                  </div>
                  <span className="cart-price">{r.available ? money(r.sub) : ""}</span>
                </li>
              ))}
            </ul>
            {order.length > 0 && (
              <footer className="cart-foot">
                {placed ? (
                  <>
                    <div className="total"><span>Total</span><strong>{money(placed.total)}</strong></div>
                    <p className="small" role="status">
                      <strong>Pedido {placed.code}</strong> a nombre de <strong>{placed.name}</strong>. Envíalo para que {shop.name} lo vea; el pago y la entrega se acuerdan por WhatsApp.
                    </p>
                    <a className="btn wa block" href={wa(shop.whatsapp, orderText(shop.name, placed))} rel="noopener" target="_blank"
                      onClick={() => { write(LAST, items); setCode(newCode()); setSent(true); }}>
                      <MessageCircle aria-hidden size={20} /> Enviar pedido por WhatsApp
                    </a>
                    <button type="button" className="btn ghost block" onClick={reset}>Cambiar mi pedido</button>
                  </>
                ) : (
                  <>
                    <div className="total"><span>Total</span><strong>{money(total)}</strong></div>
                    <p className="muted small">Todo lo de tu {only ? "pedido" : "carrito"} está en existencia. El pago y la entrega los acuerdas con {shop.name} por WhatsApp; aquí no pagas nada.</p>
                    {shop.whatsapp ? (
                      <form className="order-form" noValidate onSubmit={(e) => { e.preventDefault(); if (!busy) prepare(); }}>
                        <label htmlFor="cliente">Tu nombre para el pedido</label>
                        <input ref={nameRef} id="cliente" name="cliente" type="text" autoComplete="name" autoCapitalize="words" required maxLength={MAX_NAME}
                          value={name} onChange={(e) => { setName(e.target.value); if (nameErr) setNameErr(""); }}
                          aria-invalid={nameErr ? true : undefined} aria-describedby={nameErr ? "cliente-err" : "cliente-hint"} />
                        {nameErr ? <span id="cliente-err" className="warn small">{nameErr}</span>
                          : <span id="cliente-hint" className="muted small">Así sabrá {shop.name} de quién es el pedido.</span>}
                        {placeErr && <p className="warn" role="alert">{placeErr}</p>}
                        <button type="submit" className="btn wa block" disabled={busy}>
                          <MessageCircle aria-hidden size={20} /> {busy ? "Preparando tu pedido…" : "Preparar pedido para WhatsApp"}
                        </button>
                      </form>
                    ) : (
                      <p className="muted">Esta tienda aún no tiene WhatsApp. Vuelve pronto.</p>
                    )}
                  </>
                )}
              </footer>
            )}
          </>
        )}
      </div>
    </dialog>
  );
}

// ——— Marco ———

// bar: la barra "Ver carrito" de abajo; la ficha de producto la apaga porque trae su propia barra de compra.
function Shell({ shop, children, bar = true }: { shop: ShopInfo; children: ReactNode; bar?: boolean }) {
  const [items, setItems] = useState<Item[]>(() => read(KEY));
  const [only, setOnly] = useState<Item[] | null>(null);
  const [open, setOpen] = useState(false);
  const [bump, setBump] = useState(0);
  const save = (next: Item[]) => {
    write(KEY, next);
    setItems(next);
  };
  const api: CartAPI = {
    items,
    add: (add) => {
      setOnly(null);
      save(merge(read(KEY), add));
      setBump((b) => b + 1);
      setOpen(true);
    },
    open: () => { setOnly(null); setOpen(true); },
    orderOnly: (it) => { setOnly(it); setOpen(true); },
  };
  const count = items.length; // productos distintos: las cantidades van en milésimas y pueden ser kg
  const cats = [{ href: "/", name: "Todo", on: shop.home }, ...shop.categories.map((c) => ({ href: `/c/${c.id}`, name: c.name, on: shop.current === c.id }))];
  const hello = shop.whatsapp ? wa(shop.whatsapp, `Hola, vengo de tu tienda en línea (${shop.url}).`) : "";

  return (
    <CartCtx.Provider value={api}>
      <a className="skip" href="#contenido">Ir al contenido</a>
      <header className="top" aria-label={`Encabezado de ${shop.name}`}>
        <div className="top-bar wrap">
          <a href="/" className="brand">
            {shop.logo && <img src={shop.logo} alt="" width={36} height={36} />}
            <span>{shop.name}</span>
          </a>
          <a href="/preguntas" className="top-link">Preguntas</a>
          <button type="button" className={"cart-btn" + (bump ? " bump" : "")} key={bump} onClick={api.open}
            aria-label={count ? `Carrito, ${count} ${count === 1 ? "producto" : "productos"}` : "Carrito vacío"}>
            <ShoppingBag aria-hidden size={22} strokeWidth={1.75} />
            {count > 0 && <span className="badge" aria-hidden>{count}</span>}
          </button>
        </div>
        {cats.length > 1 && (
          <nav aria-label="Categorías" className="tabs-wrap">
            <ul className="tabs wrap">
              {cats.map((c) => (
                <li key={c.href}><a href={c.href} className="tab" aria-current={c.on ? "page" : undefined}>{c.name}</a></li>
              ))}
            </ul>
          </nav>
        )}
      </header>
      <main id="contenido" className="wrap main" aria-label={shop.preview ? "Vista previa de la tienda" : undefined}>{children}</main>
      <footer className="foot">
        <div className="wrap foot-in">
          <div className="foot-brand">
            <strong>{shop.name}</strong>
            <p>Arma tu carrito con lo que hay en existencia, envíalo por WhatsApp y acuerda el pago y la entrega con {shop.name}.</p>
          </div>
          <nav aria-label="Pie de página" className="foot-cols">
            <div>
              <h2>Tienda</h2>
              <ul>
                <li><a href="/">Todos los productos</a></li>
                {shop.categories.slice(0, 6).map((c) => <li key={c.id}><a href={`/c/${c.id}`}>{c.name}</a></li>)}
              </ul>
            </div>
            <div>
              <h2>Ayuda</h2>
              <ul>
                <li><a href="/preguntas">Preguntas frecuentes</a></li>
                {hello && <li><a href={hello} rel="noopener" target="_blank">Escribir por WhatsApp</a></li>}
              </ul>
            </div>
          </nav>
        </div>
      </footer>
      {bar && count > 0 && !open && (
        <button type="button" className="cart-bar" onClick={api.open}>
          <ShoppingBag aria-hidden size={18} /> Ver carrito ({count})
        </button>
      )}
      <Cart shop={shop} items={only ?? items} save={only ? (n) => setOnly(n) : save} open={open} only={!!only}
        setOpen={(o) => { setOpen(o); if (!o) setOnly(null); }} />
    </CartCtx.Provider>
  );
}

// ——— Catálogo ———

// "/ kg" junto al precio de lo que se vende a granel.
const perUnit = (c: { unit: string }) => (c.unit && c.unit !== "pza" ? ` / ${c.unit}` : "");
// % de descuento solo si redondea a 1 o más (un peso de diferencia en $15,000 no es "oferta").
const pct = (c: CardData) => (c.was > c.price ? off(c) : 0);

// "Nuevo" deja de servir si casi todo es nuevo (tienda recién abierta): entonces no se marca nada.
function tame(cards: CardData[]) {
  if (cards.filter((c) => c.new).length * 2 > cards.length) for (const c of cards) c.new = false;
  return cards;
}

function Price({ c, big }: { c: CardData; big?: boolean }) {
  return (
    <p className={"price" + (big ? " big" : "")}>
      <span className={c.was ? "now sale" : "now"}>{money(c.price)}{perUnit(c)}</span>
      {c.was > 0 && <s className="was"><span className="sr-only">antes </span>{money(c.was)}</s>}
    </p>
  );
}

// "Agregar rápido": sin salir de la página. Las tallas y colores salen de /carrito.json ("talla|color" → agotada).
function QuickAdd({ c, onClose }: { c: CardData | null; onClose: () => void }) {
  const cart = useContext(CartCtx);
  const ref = useRef<HTMLDialogElement>(null);
  const [cat, setCat] = useState<Catalog | null>(null);
  const [failed, setFailed] = useState(false);
  const [size, setSize] = useState("");
  const [color, setColor] = useState("");
  const [msg, setMsg] = useState("");

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (c && !d.open) {
      setSize("");
      setColor("");
      setMsg("");
      d.showModal();
      getCatalog().then(setCat, () => setFailed(true));
    }
    if (!c && d.open) d.close();
  }, [c]);

  const p = c && cat ? cat[c.id] : undefined;
  const step = p?.t || c?.step || 1000;
  const combos = p ? Object.entries(p.v).map(([k, avail]) => ({ size: k.split("|")[0], color: k.split("|")[1] ?? "", sold: avail >= 0 && avail < step })) : [];
  // En el orden en que la vendedora las capturó (S, M, L…), no en el de las llaves del JSON.
  const rank = (s: string) => ((c?.sizes.indexOf(s) ?? -1) + 1 || 99);
  const sizes = [...new Set(combos.map((x) => x.size).filter(Boolean))].sort((a, b) => rank(a) - rank(b));
  const colors = [...new Set(combos.map((x) => x.color).filter(Boolean))];
  const soldAll = (f: (x: (typeof combos)[number]) => boolean) => combos.filter(f).every((x) => x.sold);

  function add() {
    if (!c) return;
    if ((sizes.length && !size) || (colors.length && !color)) return setMsg(`Elige ${sizes.length && !size ? "una talla" : "un color"}.`);
    if (combos.length && combos.find((x) => x.size === size && x.color === color)?.sold !== false) return setMsg("Esa combinación está agotada. Elige otra.");
    onClose();
    cart.add([{ id: c.id, size, color, qty: step, price: p?.p ?? c.price }]);
  }

  return (
    <dialog ref={ref} className="sheet" aria-labelledby="qa-title" onClose={onClose} onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      {c && (
        <div className="sheet-in">
          <div className="sheet-top">
            <span className="sheet-ph" style={c.color ? { backgroundColor: c.color } : undefined}>{c.photo.src && <img src={c.photo.src} alt="" width={96} height={120} />}</span>
            <div className="sheet-info">
              <h2 id="qa-title">{c.name}</h2>
              <Price c={c} />
              <a href={`/p/${c.id}`} className="small">Ver detalles</a>
            </div>
            <button type="button" className="icon-btn" aria-label="Cerrar" onClick={onClose}><X aria-hidden size={20} /></button>
          </div>
          {failed ? <p className="warn" role="alert">No pudimos cargar las opciones. Revisa tu conexión.</p> : !cat ? <p className="muted" role="status">Cargando opciones…</p> : (
            <>
              {sizes.length > 0 && (
                <Choice legend="Talla" name="qa-talla" value={size} onChange={(v) => { setSize(v); setMsg(""); }}
                  opts={sizes.map((s) => ({ name: s, swatch: "", soldOut: soldAll((x) => x.size === s) }))} />
              )}
              {colors.length > 0 && (
                <Choice legend="Color" name="qa-color" value={color} onChange={(v) => { setColor(v); setMsg(""); }}
                  opts={colors.map((s) => ({ name: s, swatch: "", soldOut: soldAll((x) => x.color === s) }))} />
              )}
              <p role="status" className="status">{msg}</p>
              <button type="button" className="btn primary block lg" onClick={add}><ShoppingBag aria-hidden size={20} /> Agregar al carrito</button>
            </>
          )}
        </div>
      )}
    </dialog>
  );
}

// Cualquier tarjeta abre el "agregar rápido" de la página.
const QuickCtx = createContext<(c: CardData) => void>(() => {});

function WithQuick({ children }: { children: ReactNode }) {
  const [c, setC] = useState<CardData | null>(null);
  return (
    <QuickCtx.Provider value={setC}>
      {children}
      <QuickAdd c={c} onClose={() => setC(null)} />
    </QuickCtx.Provider>
  );
}

function Img({ c, sizes, eager }: { c: CardData; sizes: string; eager?: boolean }) {
  const [loaded, setLoaded] = useState(false);
  if (!c.photo.src) return null;
  return (
    <img src={c.photo.src} srcSet={c.photo.srcSet} sizes={sizes} alt="" width={400} height={500}
      className={loaded ? "in" : ""} onLoad={() => setLoaded(true)} ref={(el) => { if (el?.complete) setLoaded(true); }}
      {...(eager ? { fetchPriority: "high" as const } : { loading: "lazy" as const, decoding: "async" as const })} />
  );
}

// Tarjeta: foto, nombre y precio. Sobre la foto solo va lo que cambia la decisión: agotado o % de descuento.
// "Tallas 26 a 32" si son números en orden; si no, la lista con comas ("Tallas CH, M, G").
function sizesText(s: string[]) {
  if (s.length === 1) return s[0] === "Unitalla" ? "Unitalla" : `Talla ${s[0]}`;
  const n = s.map(Number);
  if (s.length > 3 && n.every((x, i) => Number.isFinite(x) && (i === 0 || x > n[i - 1]))) return `Tallas ${s[0]} a ${s[s.length - 1]}`;
  return `Tallas ${s.join(", ")}`;
}

// Existencia real: "Quedan 2" si va por acabarse (3 pasos o menos), "8 disponibles" si hay; nada si no lleva inventario.
function stockText(c: CardData): { text: string; low: boolean } | null {
  const left = c.left ?? -1;
  if (left < 0 || c.soldOut) return null;
  const amount = c.unit && c.unit !== "pza" ? qtyLabel(left, c.unit) : qtyNum(left);
  return left <= c.step * 3 ? { text: `Quedan ${amount}`, low: true } : { text: `${amount} disponibles`, low: false };
}

function Card({ c, eager }: { c: CardData; eager?: boolean }) {
  const quick = useContext(QuickCtx);
  const p = pct(c);
  const fresh = c.new && !c.soldOut;
  const stock = stockText(c);
  const colors = c.soldOut ? [] : c.colors ?? [];
  // Boutique y papelería enseñan las muestras de color; los demás, cuántos colores hay.
  const swatches = (giro === "boutique" || giro === "papeleria") && colors.some((x) => x.swatch);
  const opts = [c.sizes.length > 0 && !c.soldOut && sizesText(c.sizes), colors.length > 1 && !swatches && `${colors.length} colores`].filter(Boolean);
  // Carnicería y abarrotes: se ve de inmediato si va por kilo o por pieza.
  const unitTag = (giro === "carniceria" || giro === "abarrotes") && (c.unit && c.unit !== "pza" ? `Por ${c.unit}` : "Por pieza");
  const wide = giro === "abarrotes";
  return (
    <li className="pcard">
      <a className={"card" + (c.soldOut ? " sold" : "")} href={`/p/${c.id}`}>
        <span className="ph" style={c.color ? { backgroundColor: c.color } : undefined}>
          <Img c={c} eager={eager} sizes="(min-width: 1024px) 22vw, (min-width: 640px) 30vw, 46vw" />
          {!c.photo.src && <span className="ph-none" aria-hidden>{c.name}</span>}
          {c.soldOut ? <span className="flag dark">Agotado</span> : p > 0 ? <span className="flag sale">−{p} %</span> : null}
        </span>
        <span className="card-body">
          {(fresh || unitTag) && <span className="card-tags">{unitTag && <span className={"card-unit" + (c.unit && c.unit !== "pza" ? " bulk" : "")}>{unitTag}</span>}{fresh && <span className="card-new">Nuevo</span>}</span>}
          <span className="card-name">{c.name}</span>
          <span className="card-price">
            <strong className={c.was ? "sale" : undefined}>{money(c.price)}{perUnit(c)}</strong>
            {c.was > 0 && <s><span className="sr-only">antes </span>{money(c.was)}</s>}
          </span>
          {opts.length > 0 && <span className="card-meta">{opts.join(" · ")}</span>}
          {swatches && (
            <span className="card-sw">
              <span className="sr-only">Colores: {colors.map((x) => x.name).join(", ")}</span>
              {colors.slice(0, 6).map((x) => <i key={x.name} aria-hidden style={{ background: x.swatch || undefined }} title={x.name} />)}
              {colors.length > 6 && <span aria-hidden>+{colors.length - 6}</span>}
            </span>
          )}
          {stock && <span className={"card-stock" + (stock.low ? " low" : "")}>{stock.text}</span>}
        </span>
      </a>
      {!c.soldOut && (
        <button type="button" className={"quick" + (wide ? " wide" : "")} aria-label={`Agregar ${c.name} al carrito`} onClick={() => quick(c)}>
          <Plus aria-hidden size={18} />{wide && <span aria-hidden>Agregar</span>}
        </button>
      )}
    </li>
  );
}

function Grid({ cards, eager = 0 }: { cards: CardData[]; eager?: number }) {
  return <ul className="grid">{cards.map((c, i) => <Card key={c.id} c={c} eager={i < eager} />)}</ul>;
}

function Rail({ title, cards, id }: { title: string; cards: CardData[]; id: string }) {
  if (!cards.length) return null;
  return (
    <section className="sec" aria-labelledby={id}>
      <div className="sec-head"><h2 id={id}>{title}</h2></div>
      <ul className="rail">{cards.map((c) => <Card key={c.id} c={c} />)}</ul>
    </section>
  );
}

function Empty() {
  return (
    <div className="empty">
      <ShoppingBag aria-hidden size={40} strokeWidth={1.5} />
      <h2>Aún no hay productos aquí</h2>
      <p>Esta tienda está acomodando su vitrina. Vuelve pronto.</p>
    </div>
  );
}

// "Pedir de nuevo": lo del último pedido vuelve al carrito, donde se revalida precio y existencia.
function Reorder() {
  const cart = useContext(CartCtx);
  const [last] = useState(() => read(LAST));
  if (!last.length || cart.items.length) return null;
  const n = last.length;
  return (
    <section className="reorder" aria-label="Tu último pedido">
      <RotateCcw aria-hidden size={20} />
      <p><strong>¿Lo mismo de la otra vez?</strong> <span className="muted">Tu último pedido tenía {n} {n === 1 ? "producto" : "productos"}.</span></p>
      <button type="button" className="btn secondary" onClick={() => cart.add(last)}>Pedir de nuevo</button>
    </section>
  );
}

const withPhoto = (cs: CardData[]) => cs.filter((c) => c.photo.src && !c.soldOut);

// Cómo funciona comprar aquí, en tres datos ciertos (no hay pago en línea ni cuentas).
function Trust({ shop }: { shop: ShopInfo }) {
  return (
    <ul className="trust" aria-label="Así compras aquí">
      <li><MessageCircle aria-hidden size={20} strokeWidth={1.75} /><span><strong>Pides por WhatsApp</strong> sin crear cuenta</span></li>
      <li><CreditCard aria-hidden size={20} strokeWidth={1.75} /><span><strong>Pagas al acordar</strong> con {shop.name}</span></li>
      <li><Truck aria-hidden size={20} strokeWidth={1.75} /><span><strong>Entrega o envío</strong> se acuerda por chat</span></li>
    </ul>
  );
}

// Categorías con lo que tienen; las vacías no salen.
const groups = (shop: ShopInfo, cards: CardData[]) =>
  shop.categories.map((cat) => ({ cat, items: cards.filter((c) => c.cat === cat.id) })).filter((g) => g.items.length);
const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
// Lo que conviene mostrar primero: ofertas reales, luego novedades, luego lo demás con foto.
const featured = (cards: CardData[]) => {
  const score = (c: CardData) => (pct(c) ? 2 : 0) + (c.new ? 1 : 0);
  return withPhoto([...cards].sort((a, b) => score(b) - score(a)));
};

function Categories({ shop, cards, title = "Compra por categoría", kind = "tiles" }: { shop: ShopInfo; cards: CardData[]; title?: string; kind?: "tiles" | "round" }) {
  const tiles = groups(shop, cards);
  if (tiles.length < 2) return null;
  return (
    <section aria-labelledby="cats" className="sec">
      <div className="sec-head"><h2 id="cats">{title}</h2></div>
      <ul className={kind === "round" ? "cats round" : "cats"}>
        {tiles.map(({ cat, items }) => {
          const pic = withPhoto(items)[0];
          return (
            <li key={cat.id}>
              <a href={`/c/${cat.id}`} className="cat">
                <span className="cat-ph">{pic ? <Img c={pic} sizes="(min-width: 900px) 12rem, 40vw" /> : <span className="cat-none" aria-hidden>{cat.name.slice(0, 1)}</span>}</span>
                <span className="cat-name">{cat.name}</span>
                <span className="cat-n">{count(items.length, "producto", "productos")}</span>
              </a>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function Closing({ shop }: { shop: ShopInfo }) {
  return (
    <section className="closing" aria-labelledby="pedir">
      <h2 id="pedir">¿Tienes una duda antes de pedir?</h2>
      <p>{shop.whatsapp ? `${shop.name} te responde por WhatsApp. ` : ""}También puedes revisar las preguntas frecuentes.</p>
      <div className="actions">
        {shop.whatsapp && (
          <a href={wa(shop.whatsapp, `Hola, tengo una pregunta sobre tu tienda (${shop.url}).`)} className="btn wa" rel="noopener" target="_blank">
            <MessageCircle aria-hidden size={18} /> Preguntar por WhatsApp
          </a>
        )}
        <a href="/preguntas" className="btn secondary"><HelpCircle aria-hidden size={18} /> Preguntas frecuentes</a>
      </div>
    </section>
  );
}

// Buscador: el mismo estado sirve para la portada y el catálogo. Enter lleva a los resultados.
function SearchBox({ q, setQ, total, big, label = "Buscar productos" }: { q: string; setQ: (v: string) => void; total: number; big?: boolean; label?: string }) {
  return (
    <form role="search" aria-label={big ? "Buscar en la tienda" : "Buscar en el catálogo"} className={"search" + (big ? " big" : "")} onSubmit={(e) => { e.preventDefault(); document.getElementById("catalogo")?.scrollIntoView({ block: "start", behavior: reduced() ? "auto" : "smooth" }); }}>
      <Search aria-hidden size={big ? 22 : 18} />
      <label className="sr-only" htmlFor={big ? "q-top" : "q"}>{label}</label>
      <input id={big ? "q-top" : "q"} type="search" placeholder={`Buscar en ${total} productos`} value={q} onChange={(e) => setQ(e.target.value)} />
    </form>
  );
}

// Todos los productos (o los resultados): la sección a la que llevan el buscador y "Ver productos".
function Catalog({ shop, cards, found, q, setQ, title = "Todos los productos", children }:
  { shop: ShopInfo; cards: CardData[]; found: CardData[]; q: string; setQ: (v: string) => void; title?: string; children?: ReactNode }) {
  const term = q.trim();
  return (
    <section aria-labelledby="todo" id="catalogo" className="sec catalog">
      <div className="sec-head row">
        <h2 id="todo" aria-live="polite">{term ? (found.length ? count(found.length, "resultado", "resultados") : "Sin resultados") : title}</h2>
        <SearchBox q={q} setQ={setQ} total={cards.length} />
      </div>
      {!found.length ? (
        <p className="muted">
          Prueba con otra palabra
          {shop.whatsapp ? <> o <a href={wa(shop.whatsapp, `Hola, ¿tienen ${term}?`)} rel="noopener" target="_blank">pregunta por WhatsApp si lo tienen</a></> : null}.
        </p>
      ) : term || !children ? <Grid cards={found} eager={2} /> : children}
    </section>
  );
}

function useSearch(cards: CardData[]) {
  const [q, setQ] = useState("");
  const term = q.trim();
  const found = useMemo(() => (term ? cards.filter((c) => fold(c.name).includes(fold(term))) : cards), [term, cards]);
  return { q, setQ, term, found };
}

const sales = (cards: CardData[]) => cards.filter((c) => pct(c) && !c.soldOut);
const news = (cards: CardData[]) => cards.filter((c) => c.new && !c.soldOut);
// Un riel solo aporta si es parte del catálogo, no todo.
const part = (cs: CardData[], all: CardData[]) => cs.length > 0 && cs.length < all.length;

function Actions({ shop, n, label = "Ver productos", href = "#catalogo" }: { shop: ShopInfo; n: number; label?: string; href?: string }) {
  return (
    <div className="actions">
      {n > 0 && <a href={href} className="btn primary lg">{label}</a>}
      {shop.whatsapp && (
        <a href={wa(shop.whatsapp, `Hola, vengo de tu tienda en línea (${shop.url}).`)} className="btn secondary lg" rel="noopener" target="_blank">
          <MessageCircle aria-hidden size={18} /> Escribir
        </a>
      )}
    </div>
  );
}

// Producto de portada: foto con su nombre y precio reales.
function Feature({ c, sizes, eager, className = "" }: { c: CardData; sizes: string; eager?: boolean; className?: string }) {
  return (
    <a href={`/p/${c.id}`} className={"feature " + className}>
      <span className="feature-ph" style={c.color ? { backgroundColor: c.color } : undefined}><Img c={c} eager={eager} sizes={sizes} /></span>
      <span className="feature-txt">
        <span className="feature-name">{c.name}</span>
        <span className="feature-price">{money(c.price)}{perUnit(c)}{c.was > 0 && <s><span className="sr-only">antes </span>{money(c.was)}</s>}</span>
      </span>
    </a>
  );
}

// ——— Vitrina: almacén general. Buscador grande, colección destacada, categorías redondas, bandas alternas. ———
function VitrinaHome({ shop, cards }: HomeProps) {
  const s = useSearch(cards);
  const top = featured(cards).slice(0, 3);
  const sale = sales(cards), fresh = news(cards);
  const lead = part(sale, cards) ? "En oferta" : part(fresh, cards) ? "Novedades" : "Lo destacado";
  return (
    <>
      <section className="v-hero band" aria-labelledby="tienda">
        <div className="v-hero-text">
          <h1 id="tienda"><Rise text={shop.name} /></h1>
          <p className="hero-sub">{cards.length ? `${count(cards.length, "producto", "productos")} en ${count(groups(shop, cards).length, "categoría", "categorías")}. ` : ""}Arma tu pedido aquí y envíalo por WhatsApp.</p>
          {cards.length > 0 && <SearchBox q={s.q} setQ={s.setQ} total={cards.length} big />}
        </div>
        {top.length > 0 && (
          <div className="v-feat" aria-label={lead} role="group">
            <p className="v-feat-label">{lead}</p>
            <div className={`v-feat-grid n${top.length}`}>
              {top.map((c, i) => <Feature key={c.id} c={c} eager sizes={i === 0 ? "(min-width: 900px) 30vw, 90vw" : "(min-width: 900px) 15vw, 45vw"} />)}
            </div>
          </div>
        )}
      </section>
      <Trust shop={shop} />
      <Reorder />
      {!cards.length ? <Empty /> : (
        <>
          {!s.term && <Categories shop={shop} cards={cards} kind="round" />}
          {!s.term && part(sale, cards) && <div className="band"><Rail title="En oferta" cards={sale} id="oferta" /></div>}
          {!s.term && part(fresh, cards) && <Rail title="Novedades" cards={fresh} id="nuevo" />}
          <Catalog shop={shop} cards={cards} {...s} />
        </>
      )}
      <Closing shop={shop} />
    </>
  );
}

// ——— Boutique: revista. Foto de campaña dominante, colecciones en filas alternas, mucho aire. ———
function BoutiqueHome({ shop, cards }: HomeProps) {
  const s = useSearch(cards);
  const [lead, second] = featured(cards);
  const cols = groups(shop, cards).slice(0, 4);
  const fresh = news(cards);
  return (
    <>
      <section className={"b-hero" + (lead ? "" : " solo")} aria-labelledby="tienda">
        {lead && <a href={`/p/${lead.id}`} className="b-hero-main" tabIndex={-1} aria-hidden style={lead.color ? { backgroundColor: lead.color } : undefined}><Img c={lead} eager sizes="(min-width: 900px) 58vw, 100vw" /></a>}
        <div className="b-hero-text">
          <p className="kicker">{cols[0] ? `Colección ${cols[0].cat.name}` : "Colección"}</p>
          <h1 id="tienda"><Rise text={shop.name} /></h1>
          <p className="hero-sub">{cards.length ? `${count(cards.length, "pieza", "piezas")} disponibles para pedir. ` : ""}Elige tu talla y color; lo acuerdas por WhatsApp.</p>
          <Actions shop={shop} n={cards.length} label={cols[0] ? `Explorar ${cols[0].cat.name.toLowerCase()}` : "Ver la colección"} href={cols[0] ? `/c/${cols[0].cat.id}` : "#catalogo"} />
          {second && <Feature c={second} eager sizes="(min-width: 900px) 16vw, 40vw" className="b-hero-second" />}
        </div>
      </section>
      <Reorder />
      {!cards.length ? <Empty /> : (
        <>
          {!s.term && cols.length > 1 && (
            <section className="sec b-cols" aria-labelledby="cols">
              <div className="sec-head"><h2 id="cols">Colecciones</h2></div>
              {cols.map(({ cat, items }) => {
                const pic = withPhoto(items)[0];
                return (
                  <a key={cat.id} href={`/c/${cat.id}`} className="b-col">
                    <span className="b-col-ph">{pic && <Img c={pic} sizes="(min-width: 900px) 40vw, 100vw" />}</span>
                    <span className="b-col-txt">
                      <span className="b-col-name">{cat.name}</span>
                      <span className="muted">{count(items.length, "pieza", "piezas")}{items.some((c) => c.sizes.length) ? ` · tallas ${[...new Set(items.flatMap((c) => c.sizes))].slice(0, 5).join(", ")}` : ""}</span>
                      <span className="b-col-go">Ver colección <ChevronRight aria-hidden size={16} /></span>
                    </span>
                  </a>
                );
              })}
            </section>
          )}
          {!s.term && part(fresh, cards) && <Rail title="Recién llegado" cards={fresh} id="nuevo" />}
          <Catalog shop={shop} cards={cards} {...s} title="Toda la colección" />
        </>
      )}
      <Trust shop={shop} />
      <Closing shop={shop} />
    </>
  );
}

// ——— Barbería: señalética. Banda carbón con titular compacto, producto protagonista, familias numeradas. ———
function BarberiaHome({ shop, cards }: HomeProps) {
  const s = useSearch(cards);
  const [lead] = featured(cards);
  const fams = groups(shop, cards);
  const sale = sales(cards);
  return (
    <>
      <section className={"k-hero" + (lead ? "" : " solo")} aria-labelledby="tienda">
        <div className="k-hero-text">
          <p className="kicker">Productos · {shop.name}</p>
          <h1 id="tienda"><Rise text={shop.name} /></h1>
          <p className="hero-sub">{cards.length ? `${count(cards.length, "producto", "productos")} en existencia. ` : ""}Pide por WhatsApp y pasa por ellos o acuerda la entrega.</p>
          <Actions shop={shop} n={cards.length} />
        </div>
        {lead && <Feature c={lead} eager sizes="(min-width: 900px) 34vw, 90vw" className="k-hero-pic" />}
      </section>
      <Reorder />
      {!cards.length ? <Empty /> : (
        <>
          {!s.term && fams.length > 1 && (
            <nav className="sec k-fams" aria-labelledby="fams">
              <div className="sec-head"><h2 id="fams">Familias</h2></div>
              <ol>
                {fams.map(({ cat, items }, i) => (
                  <li key={cat.id}>
                    <a href={`/c/${cat.id}`}>
                      <span className="k-n" aria-hidden>{String(i + 1).padStart(2, "0")}</span>
                      <span className="k-name">{cat.name}</span>
                      <span className="k-count">{count(items.length, "producto", "productos")}</span>
                      <ChevronRight aria-hidden size={20} />
                    </a>
                  </li>
                ))}
              </ol>
            </nav>
          )}
          {!s.term && part(sale, cards) && <div className="band dark"><Rail title="En oferta" cards={sale} id="oferta" /></div>}
          <Catalog shop={shop} cards={cards} {...s} />
        </>
      )}
      <Trust shop={shop} />
      <Closing shop={shop} />
    </>
  );
}

// ——— Carnicería: mostrador. Pizarra con producto del día y precios reales; catálogo por corte en renglones. ———
function CarniceriaHome({ shop, cards }: HomeProps) {
  const s = useSearch(cards);
  // Mostrador: primero lo que se vende por kilo (lo típico de la carnicería), luego por pieza; precios reales.
  const bulk = (c: CardData) => (c.unit && c.unit !== "pza" ? 0 : 1);
  const [lead] = featured(cards).sort((a, b) => bulk(a) - bulk(b));
  const board = cards.filter((c) => !c.soldOut).sort((a, b) => bulk(a) - bulk(b)).slice(0, 6);
  const cuts = groups(shop, cards);
  return (
    <>
      <section className="m-hero" aria-labelledby="tienda">
        <div className="m-hero-text">
          <p className="kicker">Mostrador en línea</p>
          <h1 id="tienda"><Rise text={shop.name} /></h1>
          <p className="hero-sub">Pide por kilo o por pieza; te confirmamos peso y entrega por WhatsApp.</p>
          <Actions shop={shop} n={cards.length} label="Ver el mostrador" />
          {lead && <Feature c={lead} eager sizes="(min-width: 900px) 22vw, 90vw" className="m-lead" />}
        </div>
        {board.length > 0 && (
          <section className="m-board" aria-labelledby="precios">
            <h2 id="precios">Precios de hoy</h2>
            <ul>
              {board.map((c) => (
                <li key={c.id}>
                  <a href={`/p/${c.id}`}><span className="m-board-name">{c.name}</span><span className="m-dots" aria-hidden /><span className="m-board-price">{money(c.price)}<small>{c.unit && c.unit !== "pza" ? ` / ${c.unit}` : " pza"}</small></span></a>
                </li>
              ))}
            </ul>
          </section>
        )}
      </section>
      <Trust shop={shop} />
      <Reorder />
      {!cards.length ? <Empty /> : (
        <>
          {!s.term && <Categories shop={shop} cards={cards} title="Cortes y secciones" />}
          <Catalog shop={shop} cards={cards} {...s} title="Todo el mostrador">
            {cuts.length > 1 ? cuts.map(({ cat, items }) => (
              <section key={cat.id} className="m-cut" aria-labelledby={`cut-${cat.id}`}>
                <h3 id={`cut-${cat.id}`}><a href={`/c/${cat.id}`}>{cat.name}</a> <span className="muted">{count(items.length, "producto", "productos")}</span></h3>
                <Grid cards={items} />
              </section>
            )) : undefined}
          </Catalog>
        </>
      )}
      <Closing shop={shop} />
    </>
  );
}

// ——— Abarrotes: pasillos. Buscador primero, accesos a categorías, un riel por pasillo, compra rápida. ———
function AbarrotesHome({ shop, cards }: HomeProps) {
  const s = useSearch(cards);
  const aisles = groups(shop, cards);
  const sale = sales(cards);
  return (
    <>
      <section className="a-hero band" aria-labelledby="tienda">
        <h1 id="tienda"><Rise text={shop.name} /></h1>
        <p className="hero-sub">¿Qué te hace falta? Búscalo, agrégalo y manda tu lista por WhatsApp.</p>
        {cards.length > 0 && <SearchBox q={s.q} setQ={s.setQ} total={cards.length} big />}
        {aisles.length > 1 && (
          <nav aria-label="Pasillos" className="a-aisles">
            {aisles.map(({ cat, items }) => <a key={cat.id} href={`#pasillo-${cat.id}`} className="a-aisle">{cat.name} <span>{items.length}</span></a>)}
          </nav>
        )}
      </section>
      <Reorder />
      {!cards.length ? <Empty /> : (
        <>
          {!s.term && part(sale, cards) && <div className="band hot"><Rail title="Ofertas" cards={sale} id="oferta" /></div>}
          <Catalog shop={shop} cards={cards} {...s} title="Todo el surtido">
            {aisles.length > 1 ? aisles.map(({ cat, items }) => (
              <section key={cat.id} id={`pasillo-${cat.id}`} className="a-row" aria-labelledby={`p-${cat.id}`}>
                <div className="sec-head row"><h3 id={`p-${cat.id}`}>{cat.name}</h3><a href={`/c/${cat.id}`} className="more">Ver {items.length} <ChevronRight aria-hidden size={16} /></a></div>
                <ul className="rail">{items.map((c) => <Card key={c.id} c={c} />)}</ul>
              </section>
            )) : undefined}
          </Catalog>
        </>
      )}
      <Trust shop={shop} />
      <Closing shop={shop} />
    </>
  );
}

// ——— Papelería: cuaderno y carpetas. Hoja cuadriculada, etiquetas de categoría, catálogo por carpeta. ———
function PapeleriaHome({ shop, cards }: HomeProps) {
  const s = useSearch(cards);
  const pics = featured(cards).slice(0, 4);
  const folders = groups(shop, cards);
  return (
    <>
      <section className={"p-hero" + (pics.length ? "" : " solo")} aria-labelledby="tienda">
        <div className="p-hero-text">
          <h1 id="tienda"><Rise text={shop.name} /></h1>
          <p className="hero-sub">{cards.length ? `${count(cards.length, "artículo", "artículos")} listos para pedir. ` : ""}Haz tu lista aquí y mándala por WhatsApp.</p>
          <Actions shop={shop} n={cards.length} />
          {folders.length > 1 && (
            <ul className="p-tags" aria-label="Categorías">
              {folders.map(({ cat, items }) => <li key={cat.id}><a href={`/c/${cat.id}`}>{cat.name} <span>{items.length}</span></a></li>)}
            </ul>
          )}
        </div>
        {pics.length > 0 && (
          <div className={`p-mosaic n${pics.length}`} aria-hidden>
            {pics.map((c) => <a key={c.id} href={`/p/${c.id}`} tabIndex={-1} style={c.color ? { backgroundColor: c.color } : undefined}><Img c={c} eager sizes="(min-width: 900px) 20vw, 45vw" /></a>)}
          </div>
        )}
      </section>
      <Trust shop={shop} />
      <Reorder />
      {!cards.length ? <Empty /> : (
        <Catalog shop={shop} cards={cards} {...s}>
          {folders.length > 1 ? folders.map(({ cat, items }, i) => (
            <section key={cat.id} className={`p-folder c${i % 3}`} aria-labelledby={`f-${cat.id}`}>
              <h3 id={`f-${cat.id}`} className="p-tab"><a href={`/c/${cat.id}`}>{cat.name}</a> <span>{items.length}</span></h3>
              <Grid cards={items} />
            </section>
          )) : undefined}
        </Catalog>
      )}
      <Closing shop={shop} />
    </>
  );
}

type HomeProps = { shop: ShopInfo; cards: CardData[] };
const HOMES: Record<string, (p: HomeProps) => ReactNode> = {
  vitrina: VitrinaHome, boutique: BoutiqueHome, barberia: BarberiaHome, carniceria: CarniceriaHome, abarrotes: AbarrotesHome, papeleria: PapeleriaHome,
};

// Cada giro tiene su propia portada y orden; el marco (encabezado, carrito, ficha) es el mismo para no reaprender a comprar.
function Home({ shop, cards }: HomeProps) {
  tame(cards);
  const Body = HOMES[shop.style] ?? VitrinaHome;
  return (
    <Shell shop={shop}>
      <WithQuick><Body shop={shop} cards={cards} /></WithQuick>
    </Shell>
  );
}

function Category({ shop, title, cards }: { shop: ShopInfo; title: string; cards: CardData[] }) {
  tame(cards);
  return (
    <Shell shop={shop}>
      <WithQuick>
        <section className="page-head" aria-labelledby="cat-title">
          <p className="kicker"><a href="/">Inicio</a></p>
          <h1 id="cat-title"><Rise text={title} /></h1>
          <p className="muted">{count(cards.length, "producto", "productos")}</p>
        </section>
        {!cards.length ? <Empty /> : <Grid cards={cards} eager={2} />}
      </WithQuick>
    </Shell>
  );
}


// ——— Producto ———

function Choice({ legend, name, opts, value, onChange }: { legend: string; name: string; opts: Opt[]; value: string; onChange: (v: string) => void }) {
  return (
    <fieldset className="opts">
      <legend>{legend}{value && <span className="muted">: {value}</span>}</legend>
      <div className="opt-list">
        {opts.map((o) => (
          <label key={o.name} className="opt">
            <input type="radio" name={name} value={o.name} checked={value === o.name} disabled={o.soldOut} onChange={() => onChange(o.name)} />
            <span>
              {o.swatch && <i className="sw" style={{ background: o.swatch }} aria-hidden />}
              {o.name}
              {o.soldOut && <span className="sr-only"> (agotado)</span>}
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

function Gallery({ p }: { p: Product }) {
  const ref = useRef<HTMLDivElement>(null);
  const [cur, setCur] = useState(0);
  const go = (i: number) => {
    const g = ref.current;
    if (g) g.scrollTo({ left: i * g.clientWidth, behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" });
  };
  if (!p.photos.length) return <div className="gallery none" style={p.card.color ? { backgroundColor: p.card.color } : undefined} />;
  return (
    <div className="gallery-wrap">
      <div ref={ref} className={"gallery" + (p.card.soldOut ? " sold" : "")} tabIndex={0} role="group" aria-label={`Fotos de ${p.card.name}`}
        style={p.card.color ? { backgroundColor: p.card.color } : undefined}
        onScroll={(e) => setCur(Math.round(e.currentTarget.scrollLeft / e.currentTarget.clientWidth))}>
        {p.photos.map((ph, i) => (
          <img key={i} src={ph.src} srcSet={ph.srcSet} sizes="(min-width: 900px) 560px, 100vw" alt={i === 0 ? p.card.name : `${p.card.name}, foto ${i + 1}`}
            width={800} height={1000} {...(i === 0 ? { fetchPriority: "high" as const } : { loading: "lazy" as const, decoding: "async" as const })} />
        ))}
      </div>
      {p.photos.length > 1 && (
        <>
          <span className="dots" aria-hidden>{p.photos.map((_, i) => <i key={i} className={i === cur ? "on" : undefined} />)}</span>
          <div className="thumbs" role="group" aria-label="Elegir foto">
            {p.photos.map((ph, i) => (
              <button key={i} type="button" aria-label={`Ver foto ${i + 1}`} aria-pressed={i === cur} onClick={() => go(i)}>
                <img src={ph.src} alt="" width={56} height={70} loading="lazy" />
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function ProductPage({ shop, product }: { shop: ShopInfo; product: Product }) {
  return <Shell shop={shop} bar={product.card.soldOut}><WithQuick><ProductBody shop={shop} p={product} /></WithQuick></Shell>;
}

function ProductBody({ shop, p }: { shop: ShopInfo; p: Product }) {
  const cart = useContext(CartCtx);
  const c = p.card;
  const [size, setSize] = useState("");
  const [color, setColor] = useState("");
  const step = c.step || 1000;
  const [text, setText] = useState(() => qtyNum(step)); // lo que escribe; n es la cantidad válida
  const setQty = (milli: number) => setText(qtyNum(milli));
  const [status, setStatus] = useState("");
  const [cat, setCat] = useState<Catalog | null>(null);
  useEffect(() => { getCatalog().then(setCat, () => {}); }, []);
  const blocked = p.soldOutCombos.includes(size + "|" + color);
  // Existencia de lo elegido; sin elegir talla/color aún, no se limita.
  const chosen = (!p.sizes.length || size) && (!p.colors.length || color);
  const avail = cat?.[c.id] && chosen ? availableOf(cat[c.id], size, color) : -1;
  const max = maxOf(avail, step);
  const url = `/p/${c.id}`;
  const ask = (t: string) => wa(shop.whatsapp, `Hola, ${t} ${c.name}: ${location.origin}${url}`);

  // Barra de compra fija en el celular mientras el botón principal no se ve.
  const buyRef = useRef<HTMLButtonElement>(null);
  const [bar, setBar] = useState(false);
  useEffect(() => {
    const el = buyRef.current;
    if (!el || !("IntersectionObserver" in window)) return;
    const io = new IntersectionObserver(([e]) => setBar(!e.isIntersecting));
    io.observe(el);
    return () => io.disconnect();
  }, []);

  function ready() {
    const fail = (m: string) => {
      setStatus(m);
      if (bar) document.getElementById("buy")?.scrollIntoView({ block: "center", behavior: reduced() ? "auto" : "smooth" });
      return false;
    };
    if ((p.sizes.length && !size) || (p.colors.length && !color)) return fail(`Elige ${p.sizes.length && !size ? "una talla" : "un color"} primero.`);
    if (blocked) return fail("Esa combinación está agotada. Elige otra talla o color.");
    return true;
  }
  const typed = Math.round(parseFloat(text.replace(",", ".")) * 1000) || step;
  const n = Math.min(max, Math.max(step, Math.round(typed / step) * step));
  const few = avail > 0 && avail <= step * 5; // dato real: solo se muestra cuando queda poco
  const p100 = pct(c);

  return (
    <>
      <nav aria-label="Ruta" className="crumbs">
        <a href="/">Inicio</a>
        {p.categoryName && <><ChevronRight aria-hidden size={14} /><a href={`/c/${p.categoryID}`}>{p.categoryName}</a></>}
      </nav>
      <article className="product">
        <Gallery p={p} />
        <div className="info">
          {c.soldOut ? <span className="pill dark">Agotado</span> : c.new ? <span className="pill">Nuevo</span> : null}
          <h1>{c.name}</h1>
          <Price c={c} big />
          {c.was > c.price && <p className="save">Ahorras {money(c.was - c.price)}{p100 > 0 ? ` (${p100} %)` : ""}</p>}

          {c.soldOut ? (
            <div className="sold-box">
              <p>Se agotó por ahora.</p>
              {shop.whatsapp && <a className="btn secondary block" href={ask("¿cuándo vuelve a haber")} rel="noopener" target="_blank"><MessageCircle aria-hidden size={18} /> Preguntar cuándo vuelve</a>}
            </div>
          ) : (
            <form id="buy" className="buy" noValidate onSubmit={(e) => {
              e.preventDefault();
              if (!ready()) return;
              cart.add([{ id: c.id, size, color, qty: n, price: c.price }]);
              setStatus("Agregado al carrito.");
            }}>
              {p.sizes.length > 0 && <Choice legend="Talla" name="talla" opts={p.sizes} value={size} onChange={(v) => { setSize(v); setStatus(""); }} />}
              {p.colors.length > 0 && <Choice legend="Color" name="color" opts={p.colors} value={color} onChange={(v) => { setColor(v); setStatus(""); }} />}
              {blocked && <p className="warn">Esa combinación está agotada. Elige otra talla o color.</p>}
              <div className="qty">
                <label htmlFor="qty">Cantidad{c.unit && c.unit !== "pza" ? ` (${c.unit})` : ""}</label>
                <span className="stepper big">
                  <button type="button" aria-label={`Quitar ${qtyLabel(step, c.unit)}`} disabled={n <= step} onClick={() => setQty(n - step)}><Minus aria-hidden size={16} /></button>
                  <input id="qty" type="text" inputMode="decimal" value={text} aria-describedby={few ? "quedan" : undefined}
                    onChange={(e) => setText(e.target.value)} onBlur={() => setQty(n)} />
                  <button type="button" aria-label={`Agregar ${qtyLabel(step, c.unit)}`} disabled={n >= max} onClick={() => setQty(n + step)}><Plus aria-hidden size={16} /></button>
                </span>
                {few && <span id="quedan" className="muted small">Quedan {qtyLabel(avail, c.unit)}</span>}
              </div>
              {c.unit && c.unit !== "pza" && (
                <p className="calc" aria-live="polite">
                  <span>{qtyLabel(n, c.unit)} × {money(c.price)} / {c.unit}</span>
                  <strong>= {money(lineTotal(c.price, n))}</strong>
                </p>
              )}
              <p role="status" className="status">{status}</p>
              <button ref={buyRef} type="submit" className="btn primary block lg"><ShoppingBag aria-hidden size={20} /> Agregar al carrito</button>
              {shop.whatsapp && (
                <button type="button" className="btn secondary block" onClick={() => { if (ready()) cart.orderOnly([{ id: c.id, size, color, qty: n, price: c.price }]); }}>
                  <MessageCircle aria-hidden size={18} /> Pedir solo este por WhatsApp
                </button>
              )}
            </form>
          )}

          <ul className="how" aria-label="Así compras aquí">
            <li><MessageCircle aria-hidden size={20} strokeWidth={1.75} /><span><strong>Hay existencia.</strong> Solo puedes pedir lo que {shop.name} tiene disponible; tu pedido llega por WhatsApp.</span></li>
            <li><CreditCard aria-hidden size={20} strokeWidth={1.75} /><span><strong>Pago.</strong> {shop.payment ? <>Puedes pagar por transferencia; los datos están en <a href="/preguntas">Preguntas</a>.</> : "La forma de pago se acuerda por WhatsApp."}</span></li>
            <li><Truck aria-hidden size={20} strokeWidth={1.75} /><span><strong>Entrega.</strong> Envío o entrega se acuerdan por WhatsApp.</span></li>
          </ul>

          {(giro === "barberia" || giro === "carniceria") && (
            <dl className="specs" aria-label="Ficha del producto">
              {p.categoryName && <><dt>{giro === "carniceria" ? "Sección" : "Familia"}</dt><dd>{p.categoryName}</dd></>}
              <dt>Se vende por</dt><dd>{c.unit && c.unit !== "pza" ? `${c.unit} (de ${qtyLabel(step, c.unit)} en ${qtyLabel(step, c.unit)})` : "pieza"}</dd>
              {p.sizes.length > 0 && <><dt>Presentación</dt><dd>{p.sizes.map((o) => o.name).join(", ")}</dd></>}
              {p.colors.length > 0 && <><dt>{giro === "barberia" ? "Aroma o color" : "Color"}</dt><dd>{p.colors.map((o) => o.name).join(", ")}</dd></>}
            </dl>
          )}

          {p.description && (
            <section className="desc" aria-labelledby="desc">
              <h2 id="desc">Detalles</h2>
              {p.description.split("\n").filter((l) => l.trim()).map((l, i) => <p key={i}>{l}</p>)}
            </section>
          )}

          <div className="row-links">
            {shop.whatsapp && <a href={ask("tengo una pregunta sobre")} rel="noopener" target="_blank"><HelpCircle aria-hidden size={16} /> Preguntar por este producto</a>}
            <a href={p.share} rel="noopener" target="_blank"><Share2 aria-hidden size={16} /> Compartir</a>
          </div>
        </div>
      </article>
      <Rail title="También te puede gustar" cards={p.related} id="relacionados" />
      {!c.soldOut && (
        <div className={"buybar" + (bar ? " on" : "")} inert={!bar}>
          <span className="buybar-info">
            <span className="buybar-name">{c.name}</span>
            <strong>{money(c.price)}{perUnit(c)}</strong>
          </span>
          <button type="submit" form="buy" className="btn primary"><ShoppingBag aria-hidden size={18} /> Agregar</button>
        </div>
      )}
    </>
  );
}

// ——— Preguntas y pausa ———

function FAQ({ shop }: { shop: ShopInfo }) {
  const base = [
    { q: "¿Cómo hago mi pedido?", a: `Elige tus productos, talla y color, agrégalos al carrito y toca "Enviar pedido por WhatsApp". Se abre WhatsApp con tu pedido ya escrito; solo envíalo.` },
    { q: "¿Cómo pago?", a: `${shop.name} te responde por WhatsApp para acordar el pago.${shop.payment ? "\n\n" + shop.payment : ""}` },
    { q: "¿Cómo me llega?", a: `La entrega o el envío se acuerdan por WhatsApp con ${shop.name}.` },
    { q: "¿Mi carrito se guarda?", a: "Sí, en este celular. Si cambias de celular o borras los datos del navegador, se vacía." },
  ];
  return (
    <Shell shop={shop}>
      <div className="page-head rise">
        <h1>Preguntas frecuentes</h1>
        <p className="muted">Todo lo que necesitas saber para pedir en {shop.name}.</p>
      </div>
      <div className="faq">
        {[...base, ...shop.faq].map((qa, i) => (
          <details key={i} open={i === 0}>
            <summary>{qa.q}</summary>
            {qa.a && <p className="pre">{qa.a}</p>}
          </details>
        ))}
      </div>
      {shop.whatsapp && (
        <section className="help-card" aria-labelledby="ayuda">
          <h2 id="ayuda">¿No encontraste tu respuesta?</h2>
          <p className="muted">Escríbele a {shop.name}; te responde por WhatsApp.</p>
          <a className="btn wa" href={wa(shop.whatsapp, `Hola, tengo una pregunta sobre tu tienda (${shop.url}).`)} rel="noopener" target="_blank">
            <MessageCircle aria-hidden size={18} /> Preguntar por WhatsApp
          </a>
        </section>
      )}
    </Shell>
  );
}

function Paused() {
  return (
    <main className="paused rise">
      <h1>Tienda en pausa</h1>
      <p>Esta tienda no está recibiendo pedidos por ahora. Vuelve a intentarlo más tarde.</p>
    </main>
  );
}

const pages: Record<string, ComponentType<any>> = { home: Home, category: Category, product: ProductPage, faq: FAQ, paused: Paused };
const { page, data } = JSON.parse(document.getElementById("page-data")?.textContent || "{}");
giro = data?.shop?.style || document.documentElement.dataset.astryxTheme || "vitrina";
const Page = pages[page];
const root = document.getElementById("root");
if (Page && root) createRoot(root).render(<StrictMode><Page {...data} /></StrictMode>);
