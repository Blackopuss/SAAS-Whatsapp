// Tienda pública (<slug>.<dominio>). Datos desde views/store.templ.
// Sin componentes de Astryx a propósito: aquí pesaban ~190 KB y la compradora suele estar en un celular con datos.
// El aspecto sale del tema del estilo (tokens --color-*, --font-*, --radius-*) que Go pone en <html>.
import { StrictMode, createContext, useContext, useEffect, useMemo, useRef, useState, type ComponentType, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { ShoppingBag, MessageCircle, Minus, Plus, Search, X, Share2, CreditCard, Truck, CheckCircle2, RotateCcw, ChevronRight, HelpCircle, BookOpen } from "lucide-react";
import "./shop.css";

type ShopInfo = {
  name: string; categories: { id: number; name: string }[]; current: number; home: boolean; whatsapp: string; logo: string;
  faq: { q: string; a: string }[]; payment: string; url: string; preview: boolean; style: string;
};
type Photo = { src: string; srcSet: string };
type CardData = {
  id: number; name: string; price: number; was: number; soldOut: boolean; new: boolean; photo: Photo; color: string; sizes: string[]; cat: number;
  unit: string; step: number; // unidad (pza, kg, g, L, m) y de cuánto en cuánto, en milésimas
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

// Avisa al panel de la vendedora. keepalive: sigue aunque el celular ya se haya ido a WhatsApp.
function sendOrder(code: string, items: { id: number; size: string; color: string; qty: number }[]) {
  fetch("/pedido", { method: "POST", keepalive: true, headers: { "Content-Type": "application/json" }, body: JSON.stringify({ code, items }) }).catch(() => {});
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

// El mensaje lleva el código del pedido (la vendedora lo ubica en su panel) y termina con una pregunta concreta.
function orderText(shop: string, code: string, lines: string[], total: number) {
  return `Hola, quiero hacer este pedido en ${shop} (pedido ${code}):\n\n${lines.join("\n")}\n\nTotal: ${fmt2.format(total / 100)}\n\n¿Cómo quedan el pago y la entrega? Gracias.`;
}
const orderLine = (qty: number, unit: string, name: string, d: string, sub: number, url: string) =>
  `• ${unit === "pza" || !unit ? `${qtyNum(qty)} ×` : `${qtyLabel(qty, unit)} de`} ${name}${d ? ` (${d})` : ""}: ${fmt2.format(sub / 100)}\n  ${location.origin}${url}`;

type CartAPI = { items: Item[]; add: (i: Item[]) => void; open: () => void };
const CartCtx = createContext<CartAPI>({ items: [], add: () => {}, open: () => {} });

function Cart({ shop, items, save, open, setOpen }: { shop: ShopInfo; items: Item[]; save: (i: Item[]) => void; open: boolean; setOpen: (o: boolean) => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const [cat, setCat] = useState<Catalog | null>(null);
  const [failed, setFailed] = useState(false);
  const [sent, setSent] = useState(false);
  const [code, setCode] = useState(newCode);

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
    if (!cat) return;
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
  }, [cat]);

  // dir: +1 / -1 paso; -Infinity quita el renglón.
  const update = (i: number, dir: number, step: number, max: number) => {
    const next = read(KEY);
    if (!next[i]) return;
    next[i].qty = Math.min(max, next[i].qty + dir * step);
    if (!(next[i].qty >= step)) next.splice(i, 1);
    save(next);
  };

  let total = 0;
  const lines: string[] = [];
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
          lines.push(orderLine(qty, p.un, p.n, detail(it), sub, p.u));
          order.push({ id: it.id, size: it.size, color: it.color, qty });
        }
        return { i, it, p, qty, step, max, name: p ? p.n : "Producto", sub, available, warn };
      })
    : [];
  const count = items.length;

  function done() {
    write(KEY, []);
    save([]);
    setSent(false);
    setOpen(false);
  }

  return (
    <dialog ref={ref} className="cart" aria-labelledby="cart-title" onClose={() => { setOpen(false); setSent(false); }}
      onClick={(e) => { if (e.target === e.currentTarget) setOpen(false); }}>
      <div className="cart-in">
        <header className="cart-head">
          <h2 id="cart-title">Tu carrito</h2>
          {count > 0 && !sent && <span className="muted">{count} {count === 1 ? "producto" : "productos"}</span>}
          <button type="button" className="icon-btn" aria-label="Cerrar carrito" onClick={() => setOpen(false)}><X aria-hidden size={20} /></button>
        </header>

        {sent ? (
          <div className="cart-sent">
            <CheckCircle2 aria-hidden size={44} className="ok-icon" />
            <h3>Se abrió WhatsApp con tu pedido</h3>
            <p>Si ya lo enviaste, {shop.name} te responde por ahí para acordar el pago y la entrega.</p>
            <button type="button" className="btn primary block" onClick={done}>Ya lo envié, vaciar carrito</button>
            <button type="button" className="btn ghost block" onClick={() => setSent(false)}>Todavía no, volver al carrito</button>
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
                    <div className="cart-actions">
                      {r.available && (
                        <span className="stepper">
                          <button type="button" aria-label={`Quitar ${qtyLabel(r.step, r.p?.un ?? "")} de ${r.name}`} onClick={() => update(r.i, -1, r.step, r.max)}><Minus aria-hidden size={14} /></button>
                          <span aria-label={`Cantidad: ${qtyLabel(r.qty, r.p?.un ?? "")}`}>{qtyLabel(r.qty, r.p?.un ?? "")}</span>
                          <button type="button" aria-label={`Agregar ${qtyLabel(r.step, r.p?.un ?? "")} de ${r.name}`} disabled={r.qty >= r.max} onClick={() => update(r.i, 1, r.step, r.max)}><Plus aria-hidden size={14} /></button>
                        </span>
                      )}
                      <button type="button" className="link-btn" aria-label={`Quitar ${r.name} del carrito`} onClick={() => update(r.i, -Infinity, r.step, r.max)}>Quitar</button>
                    </div>
                  </div>
                  <span className="cart-price">{r.available ? money(r.sub) : ""}</span>
                </li>
              ))}
            </ul>
            {lines.length > 0 && (
              <footer className="cart-foot">
                <div className="total"><span>Total</span><strong>{money(total)}</strong></div>
                <p className="muted small">Todo lo de tu carrito está en existencia. El pago y la entrega los acuerdas con {shop.name} por WhatsApp; aquí no pagas nada.</p>
                {shop.whatsapp ? (
                  <a className="btn wa block" href={wa(shop.whatsapp, orderText(shop.name, code, lines, total))} rel="noopener" target="_blank"
                    onClick={() => { sendOrder(code, order); setCode(newCode()); write(LAST, read(KEY)); setSent(true); }}>
                    <MessageCircle aria-hidden size={20} /> Enviar pedido por WhatsApp
                  </a>
                ) : (
                  <p className="muted">Esta tienda aún no tiene WhatsApp. Vuelve pronto.</p>
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

function Shell({ shop, children }: { shop: ShopInfo; children: ReactNode }) {
  const [items, setItems] = useState<Item[]>(() => read(KEY));
  const [open, setOpen] = useState(false);
  const [bump, setBump] = useState(0);
  const save = (next: Item[]) => {
    write(KEY, next);
    setItems(next);
  };
  const api: CartAPI = {
    items,
    add: (add) => {
      save(merge(read(KEY), add));
      setBump((b) => b + 1);
      setOpen(true);
    },
    open: () => setOpen(true),
  };
  const count = items.length; // productos distintos: las cantidades van en milésimas y pueden ser kg
  const cats = [{ href: "/", name: "Todo", on: shop.home }, ...shop.categories.map((c) => ({ href: `/c/${c.id}`, name: c.name, on: shop.current === c.id }))];

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
          <button type="button" className={"cart-btn" + (bump ? " bump" : "")} key={bump} onClick={() => setOpen(true)}
            aria-label={count ? `Carrito, ${count} ${count === 1 ? "producto" : "productos"}` : "Carrito vacío"}>
            <ShoppingBag aria-hidden size={22} />
            {count > 0 && <span className="badge" aria-hidden>{count}</span>}
          </button>
        </div>
        {cats.length > 1 && (
          <nav aria-label="Categorías" className="chips-wrap">
            <ul className="chips wrap">
              {cats.map((c) => (
                <li key={c.href}><a href={c.href} className="chip" aria-current={c.on ? "page" : undefined}>{c.name}</a></li>
              ))}
            </ul>
          </nav>
        )}
      </header>
      <main id="contenido" className="wrap main" aria-label={shop.preview ? "Vista previa de la tienda" : undefined}>{children}</main>
      <footer className="foot">
        <div className="wrap foot-in">
          <strong>{shop.name}</strong>
          <p className="muted">Pedidos por WhatsApp: armas tu carrito con lo que hay en existencia, lo envías y acuerdas el pago y la entrega con {shop.name}.</p>
          <ul className="foot-links">
            <li><a href="/">Todos los productos</a></li>
            <li><a href="/preguntas">Preguntas frecuentes</a></li>
            {shop.whatsapp && <li><a href={wa(shop.whatsapp, `Hola, vengo de tu tienda en línea (${shop.url}).`)} rel="noopener" target="_blank">Escribir por WhatsApp</a></li>}
          </ul>
        </div>
      </footer>
      {count > 0 && !open && (
        <button type="button" className="cart-bar" onClick={() => setOpen(true)}>
          <ShoppingBag aria-hidden size={18} /> Ver carrito ({count})
        </button>
      )}
      <Cart shop={shop} items={items} save={save} open={open} setOpen={setOpen} />
    </CartCtx.Provider>
  );
}

// ——— Catálogo ———

// "/ kg" junto al precio de lo que se vende a granel.
const perUnit = (c: { unit: string }) => (c.unit && c.unit !== "pza" ? ` / ${c.unit}` : "");

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

// Etiqueta de precio en bloque de color; en oferta, el % y el precio anterior tachado.
function Tag({ c, big }: { c: CardData; big?: boolean }) {
  if (c.soldOut) return <span className={"ptag sold" + (big ? " big" : "")}>Agotado</span>;
  return (
    <span className={"ptag" + (c.was ? " sale" : "") + (big ? " big" : "")}>
      {c.was > 0 && <span className="ptag-off">−{off(c)} %</span>}
      <strong>{money(c.price)}{perUnit(c)}</strong>
      {c.was > 0 && <s><span className="sr-only">antes </span>{money(c.was)}</s>}
    </span>
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

function Card({ c, i, eager }: { c: CardData; i: number; eager?: boolean }) {
  const quick = useContext(QuickCtx);
  return (
    <li className="pcard rise" style={{ "--i": i } as React.CSSProperties}>
      <a className={"card" + (c.soldOut ? " sold" : "")} href={`/p/${c.id}`}>
        <span className="ph" style={c.color ? { backgroundColor: c.color } : undefined}>
          <Img c={c} eager={eager} sizes="(min-width: 1024px) 22vw, (min-width: 640px) 30vw, 46vw" />
          {c.new && !c.soldOut && <span className="ribbon">Nuevo</span>}
          <Tag c={c} />
        </span>
        <span className="card-body">
          <span className="card-name">{c.name}</span>
          {c.sizes.length > 0 && !c.soldOut && (
            <span className="sizes"><span className="sr-only">Tallas: </span>{c.sizes.map((s) => <i key={s}>{s}</i>)}</span>
          )}
        </span>
      </a>
      {!c.soldOut && (
        <button type="button" className="quick" aria-label={`Agregar ${c.name} al carrito`} onClick={() => quick(c)}>
          <Plus aria-hidden size={20} />
        </button>
      )}
    </li>
  );
}

function Grid({ cards, eager = 0 }: { cards: CardData[]; eager?: number }) {
  return <ul className="grid">{cards.map((c, i) => <Card key={c.id} c={c} i={i} eager={i < eager} />)}</ul>;
}

function Rail({ title, kicker, cards, id }: { title: string; kicker?: string; cards: CardData[]; id: string }) {
  if (!cards.length) return null;
  return (
    <section className="rail-sec" aria-labelledby={id}>
      <div className="sec-head">
        {kicker && <span className="kicker">{kicker}</span>}
        <h2 id={id} className="sec-title">{title}</h2>
      </div>
      <ul className="rail">{cards.map((c, i) => <Card key={c.id} c={c} i={i} />)}</ul>
    </section>
  );
}

// Renglón de lista de precios (carnicería): nombre a la izquierda, precio por unidad a la derecha.
function Row({ c, i }: { c: CardData; i: number }) {
  const quick = useContext(QuickCtx);
  return (
    <li className="brow rise" style={{ "--i": i } as React.CSSProperties}>
      <a className={"brow-link" + (c.soldOut ? " sold" : "")} href={`/p/${c.id}`}>
        {c.photo.src && <span className="brow-ph" style={c.color ? { backgroundColor: c.color } : undefined}><Img c={c} sizes="72px" /></span>}
        <span className="brow-name">
          {c.name}
          {c.new && !c.soldOut && <em className="brow-new">Nuevo</em>}
          {c.unit !== "pza" && !c.soldOut && <small>{`De ${qtyLabel(c.step, c.unit)} en ${qtyLabel(c.step, c.unit)}`}</small>}
        </span>
        <span className="brow-dots" aria-hidden />
        <Tag c={c} />
      </a>
      {!c.soldOut && (
        <button type="button" className="quick" aria-label={`Agregar ${c.name} al carrito`} onClick={() => quick(c)}>
          <Plus aria-hidden size={20} />
        </button>
      )}
    </li>
  );
}

function Rows({ cards }: { cards: CardData[] }) {
  return <ul className="rows">{cards.map((c, i) => <Row key={c.id} c={c} i={i} />)}</ul>;
}

// Productos agrupados por categoría (en el orden de la tienda); lo que no tiene categoría va al final.
function groups(shop: ShopInfo, cards: CardData[]) {
  const known = new Set(shop.categories.map((k) => k.id));
  return [
    ...shop.categories.map((k) => ({ id: k.id, name: k.name, items: cards.filter((c) => c.cat === k.id) })),
    { id: 0, name: shop.categories.length ? "Otros" : "Productos", items: cards.filter((c) => !known.has(c.cat)) },
  ].filter((g) => g.items.length);
}

// Pizarrón: una lista de precios por categoría, como el letrero del mostrador.
function Board({ shop, cards }: { shop: ShopInfo; cards: CardData[] }) {
  return (
    <div className="board">
      {groups(shop, cards).map((g) => (
        <section key={g.id} className="board-sec" aria-labelledby={`g${g.id}`}>
          <h3 id={`g${g.id}`} className="board-title">{g.name}</h3>
          <Rows cards={g.items} />
        </section>
      ))}
    </div>
  );
}

// Pasillos (abarrotes) o estaciones (barbería): una repisa deslizable por categoría.
function Aisles({ shop, cards, label }: { shop: ShopInfo; cards: CardData[]; label: string }) {
  return (
    <div className="aisles">
      {groups(shop, cards).map((g, n) => (
        <section key={g.id} className="aisle" aria-labelledby={`g${g.id}`}>
          <div className="sec-head row">
            <div>
              <span className="kicker">{`${label} ${String(n + 1).padStart(2, "0")}`}</span>
              <h3 id={`g${g.id}`} className="aisle-title">{g.name}</h3>
            </div>
            {g.id > 0 && g.items.length > 12 && <a href={`/c/${g.id}`} className="more">{`Ver los ${g.items.length}`} <ChevronRight aria-hidden size={16} /></a>}
          </div>
          <ul className="rail shelf">{g.items.slice(0, g.id ? 12 : undefined).map((c, i) => <Card key={c.id} c={c} i={i} />)}</ul>
        </section>
      ))}
    </div>
  );
}

function Empty() {
  return (
    <div className="empty">
      <ShoppingBag aria-hidden size={40} />
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
      <RotateCcw aria-hidden size={22} />
      <p><strong>¿Lo mismo de la otra vez?</strong> <span className="muted">Tu último pedido tenía {n} {n === 1 ? "producto" : "productos"}.</span></p>
      <button type="button" className="btn secondary" onClick={() => cart.add(last)}>Pedir de nuevo</button>
    </section>
  );
}

const withPhoto = (cs: CardData[]) => cs.filter((c) => c.photo.src && !c.soldOut);

const KICKER: Record<string, string> = { carniceria: "Lista de precios", boutique: "Colección", abarrotes: "Surtido" };

// Portada: nombre grande y un collage de tres productos con su precio.
function Cover({ shop, cards }: { shop: ShopInfo; cards: CardData[] }) {
  const pick = [...withPhoto(cards.filter((c) => c.was || c.new)), ...withPhoto(cards)].filter((c, i, a) => a.indexOf(c) === i).slice(0, 3);
  const month = new Intl.DateTimeFormat("es-MX", { month: "long", year: "numeric" }).format(new Date());
  return (
    <section className="cover" aria-labelledby="tienda">
      <div className="cover-text rise">
        <span className="kicker">{KICKER[shop.style] ?? "Catálogo"} · {month}</span>
        <h1 id="tienda">{shop.name}</h1>
        <p className="cover-sub">{cards.length} {cards.length === 1 ? "producto" : "productos"} para pedir directo por WhatsApp.</p>
        <div className="cover-cta">
          <a href="#catalogo" className="btn primary lg"><BookOpen aria-hidden size={20} /> Ver catálogo</a>
          {shop.whatsapp && (
            <a href={wa(shop.whatsapp, `Hola, vengo de tu tienda en línea (${shop.url}).`)} className="btn secondary lg" rel="noopener" target="_blank">
              <MessageCircle aria-hidden size={20} /> Escribir
            </a>
          )}
        </div>
      </div>
      {pick.length > 0 && (
        <div className={`collage n${pick.length}`} aria-hidden>
          {pick.map((c, i) => (
            <a key={c.id} href={`/p/${c.id}`} tabIndex={-1} className={`shot s${i}`} style={c.color ? { backgroundColor: c.color } : undefined}>
              <Img c={c} eager sizes="(min-width: 900px) 22vw, 45vw" />
              <Tag c={c} />
            </a>
          ))}
        </div>
      )}
    </section>
  );
}

function Steps() {
  return (
    <ol className="steps-band" aria-label="Cómo comprar">
      <li><span className="step-n" aria-hidden>1</span><span><strong>Elige</strong> lo que te guste</span></li>
      <li><span className="step-n" aria-hidden>2</span><span><strong>Agrégalo</strong> al carrito</span></li>
      <li><span className="step-n" aria-hidden>3</span><span><strong>Envía</strong> tu pedido por WhatsApp</span></li>
    </ol>
  );
}

function Shots({ cards, eager }: { cards: CardData[]; eager?: boolean }) {
  return (
    <span className="tile-shots" aria-hidden>
      {cards.map((c, j) => (
        <span key={c.id} className={`tshot k${j}`} style={c.color ? { backgroundColor: c.color } : undefined}><Img c={c} eager={eager} sizes="160px" /></span>
      ))}
    </span>
  );
}

// Mosaico de categorías: cada una con su color y fotos asomándose.
function CategoryTiles({ shop, cards }: { shop: ShopInfo; cards: CardData[] }) {
  const tiles = shop.categories.map((cat) => ({ cat, items: cards.filter((c) => c.cat === cat.id) })).filter((t) => t.items.length);
  if (tiles.length < 2) return null;
  return (
    <section aria-labelledby="cats" className="tiles-sec">
      <div className="sec-head"><span className="kicker">Explora</span><h2 id="cats" className="sec-title">Categorías</h2></div>
      <ul className="tiles">
        {tiles.map(({ cat, items }, i) => (
          <li key={cat.id} className="rise" style={{ "--i": i } as React.CSSProperties}>
            <a href={`/c/${cat.id}`} className={`tile t${i % 4}`}>
              <span className="tile-text">
                <strong>{cat.name}</strong>
                <span>{items.length} {items.length === 1 ? "producto" : "productos"} <ChevronRight aria-hidden size={16} /></span>
              </span>
              <Shots cards={withPhoto(items).slice(0, 2)} />
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}

// Destacado: la mejor oferta real (o la novedad) en grande.
function Spotlight({ cards }: { cards: CardData[] }) {
  const quick = useContext(QuickCtx);
  const c = withPhoto(cards).filter((x) => x.was).sort((a, b) => off(b) - off(a))[0] ?? withPhoto(cards).find((x) => x.new);
  if (!c) return null;
  return (
    <section className="spot" aria-labelledby="spot" style={c.color ? ({ "--spot": c.color } as React.CSSProperties) : undefined}>
      <a href={`/p/${c.id}`} className="spot-ph" tabIndex={-1} aria-hidden><Img c={c} sizes="(min-width: 900px) 40vw, 90vw" /></a>
      <div className="spot-text">
        <span className="kicker">{c.was ? "La mejor oferta" : "Lo más nuevo"}</span>
        <h2 id="spot"><a href={`/p/${c.id}`}>{c.name}</a></h2>
        <Tag c={c} big />
        {c.sizes.length > 0 && <p className="muted">Tallas: {c.sizes.join(" · ")}</p>}
        <div className="cover-cta">
          <button type="button" className="btn primary lg" onClick={() => quick(c)}><ShoppingBag aria-hidden size={20} /> Agregar al carrito</button>
          <a href={`/p/${c.id}`} className="btn secondary lg">Ver detalles</a>
        </div>
      </div>
    </section>
  );
}

function Closing({ shop }: { shop: ShopInfo }) {
  return (
    <section className="closing" aria-labelledby="pedir">
      <h2 id="pedir">¿Lista para pedir?</h2>
      <p>Arma tu carrito con lo que hay en existencia y envíalo por WhatsApp. Ahí acuerdas el pago y la entrega con {shop.name}.</p>
      <div className="cover-cta">
        {shop.whatsapp && (
          <a href={wa(shop.whatsapp, `Hola, tengo una pregunta sobre tu tienda (${shop.url}).`)} className="btn wa lg" rel="noopener" target="_blank">
            <MessageCircle aria-hidden size={20} /> Preguntar por WhatsApp
          </a>
        )}
        <a href="/preguntas" className="btn on-dark lg"><HelpCircle aria-hidden size={20} /> Preguntas frecuentes</a>
      </div>
    </section>
  );
}

// Cada giro arma su inicio distinto con los mismos datos (el CSS de su tema hace el resto):
// carnicería = pizarrón de precios por categoría; abarrotes y barbería = pasillos/estaciones;
// boutique = novedades y destacado primero; papelería y vitrina = categorías y mosaico.
function Home({ shop, cards }: { shop: ShopInfo; cards: CardData[] }) {
  const [q, setQ] = useState("");
  const term = q.trim();
  const found = useMemo(() => (term ? cards.filter((c) => fold(c.name).includes(fold(term))) : cards), [term, cards]);
  const fresh = cards.filter((c) => c.new && !c.soldOut);
  const sale = cards.filter((c) => c.was && !c.soldOut);
  const st = shop.style;
  const board = st === "carniceria";
  const aisles = st === "abarrotes" || st === "barberia";
  const some = (cs: CardData[]) => !term && cs.length > 0 && cs.length < cards.length;
  const news = some(fresh) && <Rail kicker="Recién llegado" title="Novedades" cards={fresh} id="nuevo" />;
  const deals = some(sale) && <div className="band"><Rail kicker="Precios especiales" title="En oferta" cards={sale} id="oferta" /></div>;
  const catalog = (
    <section aria-labelledby="todo" id="catalogo" className="all">
      <div className="sec-head row">
        <div>
          <span className="kicker">{board ? "Precios" : aisles ? "Recorre la tienda" : "Catálogo completo"}</span>
          <h2 id="todo" className="sec-title" aria-live="polite">
            {term ? (found.length ? `${found.length} ${found.length === 1 ? "resultado" : "resultados"}` : "Sin resultados") : board ? "Lista de precios" : aisles ? (st === "barberia" ? "Estaciones" : "Pasillos") : "Todo"}
          </h2>
        </div>
        <label className="search">
          <Search aria-hidden size={18} />
          <span className="sr-only">Buscar productos</span>
          <input type="search" placeholder={`Buscar en ${cards.length} productos`} value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
      </div>
      {!found.length ? (
        <p className="muted">
          Prueba con otra palabra
          {shop.whatsapp ? <> o <a href={wa(shop.whatsapp, `Hola, ¿tienen ${term}?`)} rel="noopener" target="_blank">pregunta por WhatsApp si lo tienen</a></> : null}.
        </p>
      ) : board ? (term ? <Rows cards={found} /> : <Board shop={shop} cards={cards} />)
        : aisles && !term ? <Aisles shop={shop} cards={cards} label={st === "barberia" ? "Estación" : "Pasillo"} />
        : <Grid cards={found} />}
    </section>
  );
  return (
    <Shell shop={shop}>
      <WithQuick>
        <Cover shop={shop} cards={cards} />
        <Steps />
        <Reorder />
        {!cards.length ? <Empty /> : board ? (
          catalog
        ) : aisles ? (
          <>
            {!term && <Spotlight cards={cards} />}
            {deals}
            {catalog}
          </>
        ) : st === "boutique" ? (
          <>
            {news}
            {!term && <Spotlight cards={cards} />}
            {!term && <CategoryTiles shop={shop} cards={cards} />}
            {deals}
            {catalog}
          </>
        ) : (
          <>
            {!term && <CategoryTiles shop={shop} cards={cards} />}
            {!term && <Spotlight cards={cards} />}
            {news}
            {deals}
            {catalog}
          </>
        )}
        <Closing shop={shop} />
      </WithQuick>
    </Shell>
  );
}

function Category({ shop, title, cards }: { shop: ShopInfo; title: string; cards: CardData[] }) {
  return (
    <Shell shop={shop}>
      <WithQuick>
        <section className="cat-banner rise" aria-labelledby="cat-title">
          <div>
            <span className="kicker">{shop.name}</span>
            <h1 id="cat-title">{title}</h1>
            <p>{cards.length} {cards.length === 1 ? "producto" : "productos"}</p>
          </div>
          <Shots cards={withPhoto(cards).slice(0, 3)} eager />
        </section>
        {!cards.length ? <Empty /> : shop.style === "carniceria" ? <Rows cards={cards} /> : <Grid cards={cards} eager={2} />}
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
        <div className="thumbs" role="group" aria-label="Elegir foto">
          {p.photos.map((ph, i) => (
            <button key={i} type="button" aria-label={`Ver foto ${i + 1}`} aria-pressed={i === cur} onClick={() => go(i)}>
              <img src={ph.src} alt="" width={56} height={70} loading="lazy" />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function ProductPage({ shop, product }: { shop: ShopInfo; product: Product }) {
  return <Shell shop={shop}><WithQuick><ProductBody shop={shop} p={product} /></WithQuick></Shell>;
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

  function ready() {
    if ((p.sizes.length && !size) || (p.colors.length && !color)) {
      setStatus(`Elige ${p.sizes.length && !size ? "una talla" : "un color"} primero.`);
      return false;
    }
    if (blocked) {
      setStatus("Esa combinación está agotada. Elige otra talla o color.");
      return false;
    }
    return true;
  }
  const typed = Math.round(parseFloat(text.replace(",", ".")) * 1000) || step;
  const n = Math.min(max, Math.max(step, Math.round(typed / step) * step));
  const few = avail > 0 && avail <= step * 5; // dato real: solo se muestra cuando queda poco

  return (
    <>
      <nav aria-label="Ruta" className="crumbs">
        <a href="/">Inicio</a>
        {p.categoryName && <><ChevronRight aria-hidden size={14} /><a href={`/c/${p.categoryID}`}>{p.categoryName}</a></>}
      </nav>
      <article className="product">
        <Gallery p={p} />
        <div className="info rise">
          {c.soldOut ? <span className="tag dark inline">Agotado</span> : c.new ? <span className="tag inline">Nuevo</span> : null}
          <h1>{c.name}</h1>
          <Price c={c} big />
          {c.was > 0 && <p className="save">Ahorras {money(c.was - c.price)} ({off(c)} %)</p>}

          {c.soldOut ? (
            <div className="sold-box">
              <p>Se agotó por ahora.</p>
              {shop.whatsapp && <a className="btn secondary block" href={ask("¿cuándo vuelve a haber")} rel="noopener" target="_blank"><MessageCircle aria-hidden size={18} /> Preguntar cuándo vuelve</a>}
            </div>
          ) : (
            <form className="buy" noValidate onSubmit={(e) => {
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
              <button type="submit" className="btn primary block lg"><ShoppingBag aria-hidden size={20} /> Agregar al carrito</button>
              {shop.whatsapp && (
                <a className="btn secondary block" href={wa(shop.whatsapp, "")} rel="noopener" target="_blank"
                  onClick={(e) => {
                    if (!ready()) return e.preventDefault();
                    const sub = lineTotal(c.price, n);
                    const code = newCode();
                    sendOrder(code, [{ id: c.id, size, color, qty: n }]);
                    e.currentTarget.href = wa(shop.whatsapp, orderText(shop.name, code, [orderLine(n, c.unit, c.name, detail({ size, color }), sub, url)], sub));
                  }}>
                  <MessageCircle aria-hidden size={18} /> Pedir solo este por WhatsApp
                </a>
              )}
              <p role="status" className="status">{status}</p>
            </form>
          )}

          <ul className="how" aria-label="Así compras aquí">
            <li><MessageCircle aria-hidden size={20} /><span><strong>Hay existencia.</strong> Solo puedes pedir lo que {shop.name} tiene disponible; tu pedido llega por WhatsApp.</span></li>
            <li><CreditCard aria-hidden size={20} /><span><strong>Pago.</strong> {shop.payment ? <>Puedes pagar por transferencia; los datos están en <a href="/preguntas">Preguntas</a>.</> : "La forma de pago se acuerda por WhatsApp."}</span></li>
            <li><Truck aria-hidden size={20} /><span><strong>Entrega.</strong> Envío o entrega se acuerdan por WhatsApp.</span></li>
          </ul>

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
      <Rail kicker="Combínalo" title="También te puede gustar" cards={p.related} id="relacionados" />
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
          <details key={i} className="rise" style={{ "--i": i } as React.CSSProperties} open={i === 0}>
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
const Page = pages[page];
const root = document.getElementById("root");
if (Page && root) createRoot(root).render(<StrictMode><Page {...data} /></StrictMode>);
