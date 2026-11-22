// Tienda pública (<slug>.<dominio>): inicio, categoría, producto, preguntas y carrito. Datos desde views/store.templ.
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { ShoppingBag, Share2, MessageCircle, Minus, Plus, Store as StoreIcon } from "lucide-react";
import { mount, pageData } from "./boot";
import { Button } from "@astryxdesign/core/Button";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Link } from "@astryxdesign/core/Link";
import { HStack, VStack } from "@astryxdesign/core/Stack";
import { Token } from "@astryxdesign/core/Token";
import { Drawer } from "@astryxdesign/core/Drawer";
import { Collapsible } from "@astryxdesign/core/Collapsible";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Banner } from "@astryxdesign/core/Banner";
import { Center } from "@astryxdesign/core/Center";
import { NumberInput } from "@astryxdesign/core/NumberInput";
import { IconButton } from "@astryxdesign/core/IconButton";
import { Divider } from "@astryxdesign/core/Divider";
// Solo se descarga el tema del estilo de esta tienda.
const THEMES = {
  vitrina: () => import("./themes/vitrina/vitrina.js").then((m) => m.vitrinaTheme),
  boutique: () => import("./themes/boutique/boutique.js").then((m) => m.boutiqueTheme),
  barberia: () => import("./themes/barberia/barberia.js").then((m) => m.barberiaTheme),
  carniceria: () => import("./themes/carniceria/carniceria.js").then((m) => m.carniceriaTheme),
  abarrotes: () => import("./themes/abarrotes/abarrotes.js").then((m) => m.abarrotesTheme),
  papeleria: () => import("./themes/papeleria/papeleria.js").then((m) => m.papeleriaTheme),
};

type ShopInfo = {
  name: string; categories: { id: number; name: string }[]; current: number; home: boolean; whatsapp: string; logo: string;
  faq: { q: string; a: string }[]; payment: string; url: string; preview: boolean; style: keyof typeof THEMES;
};
type Photo = { src: string; srcSet: string };
type CardData = { id: number; name: string; price: number; was: number; soldOut: boolean; new: boolean; photo: Photo; color: string; sizes: string[] };
type Opt = { name: string; soldOut: boolean; swatch: string };
type Product = {
  card: CardData; description: string; photos: Photo[]; categoryID: number; categoryName: string;
  sizes: Opt[]; colors: Opt[]; soldOutCombos: string[]; share: string;
};

const fmt2 = new Intl.NumberFormat("es-MX", { style: "currency", currency: "MXN" });
const fmt0 = new Intl.NumberFormat("es-MX", { style: "currency", currency: "MXN", minimumFractionDigits: 0 });
const money = (c: number) => (c % 100 === 0 ? fmt0 : fmt2).format(c / 100);

// ——— Carrito ———
// Vive en localStorage. Precios y existencias siempre salen de /carrito.json; lo guardado solo dice qué y cuánto.
type Item = { id: number; size: string; color: string; qty: number; price: number };
type Catalog = Record<string, { n: string; p: number; x: boolean; u: string; v: Record<string, boolean> }>;
const KEY = "carrito";
const MAX = 20;

const valid = (i: Item) =>
  i && Number.isSafeInteger(i.id) && i.id > 0 && typeof i.size === "string" && i.size.length <= 20 &&
  typeof i.color === "string" && i.color.length <= 30 && Number.isInteger(i.qty) && i.qty >= 1 && i.qty <= MAX &&
  Number.isInteger(i.price) && i.price >= 0;

function load(): Item[] {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) || "[]");
    return Array.isArray(v) ? v.filter(valid).slice(0, 50) : [];
  } catch {
    return [];
  }
}

function store(items: Item[]) {
  try {
    localStorage.setItem(KEY, JSON.stringify(items));
  } catch {
    // Modo privado o sin espacio: el carrito vive solo en esta página.
  }
}

let catalog: Promise<Catalog> | null = null;
const getCatalog = () =>
  (catalog ??= fetch("/carrito.json", { headers: { Accept: "application/json" } }).then((r) => {
    if (!r.ok) throw new Error(String(r.status));
    return r.json();
  }).catch((e) => { catalog = null; throw e; }));

type CartAPI = { items: Item[]; add: (i: Item) => void; open: () => void };
const CartCtx = createContext<CartAPI>({ items: [], add: () => {}, open: () => {} });

function useCart() {
  const [items, setItems] = useState<Item[]>(load);
  const [open, setOpen] = useState(false);
  const save = (next: Item[]) => { store(next); setItems(next); };
  const add = (it: Item) => {
    const next = load();
    const same = next.find((i) => i.id === it.id && i.size === it.size && i.color === it.color);
    if (same) same.qty = Math.min(MAX, same.qty + it.qty);
    else next.push(it);
    save(next);
    setOpen(true);
  };
  return { items, save, add, open, setOpen };
}

function Cart({ items, save, isOpen, onOpenChange, shop }: { items: Item[]; save: (i: Item[]) => void; isOpen: boolean; onOpenChange: (o: boolean) => void; shop: ShopInfo }) {
  const [cat, setCat] = useState<Catalog | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    if (!isOpen || !items.length) return;
    getCatalog().then(setCat, () => setFailed(true));
  }, [isOpen, items.length, failed]);

  const update = (i: number, delta: number) => {
    const next = load();
    if (!next[i]) return;
    next[i].qty = Math.min(MAX, next[i].qty + delta);
    if (!(next[i].qty >= 1)) next.splice(i, 1);
    save(next);
  };

  let total = 0;
  const lines: string[] = [];
  const rows = cat ? items.map((it, i) => {
    const p = cat[it.id];
    const combo = it.size + "|" + it.color;
    const hasCombos = p && Object.keys(p.v).length > 0;
    const available = !!p && !(p.x || (hasCombos && p.v[combo] !== false));
    let warn = "";
    if (!p) warn = "Este producto ya no está disponible.";
    else if (!available) warn = "Se agotó.";
    else if (p.p !== it.price) warn = `Cambió de precio: antes ${money(it.price)}, ahora ${money(p.p)}.`;
    const name = p ? p.n : "Producto";
    const detail = [it.size && `Talla ${it.size}`, it.color].filter(Boolean).join(" · ");
    const sub = available ? p.p * it.qty : 0;
    if (available) {
      total += sub;
      lines.push(`• ${it.qty} × ${name}${detail ? ` (${detail})` : ""}: ${fmt2.format(sub / 100)}\n  ${location.origin}${p.u}`);
    }
    return { i, it, p, name, detail, sub, available, warn };
  }) : [];

  // Guarda el precio nuevo para que el aviso salga una vez.
  useEffect(() => {
    if (!cat) return;
    let changed = false;
    const next = load().map((it) => {
      const p = cat[it.id];
      if (p && p.p !== it.price) { changed = true; return { ...it, price: p.p }; }
      return it;
    });
    if (changed) store(next);
  }, [cat]);

  const msg = `Hola, quiero hacer este pedido en ${shop.name}:\n\n${lines.join("\n")}\n\nTotal: ${fmt2.format(total / 100)}`;
  const wa = shop.whatsapp ? `https://wa.me/${encodeURIComponent(shop.whatsapp)}?text=${encodeURIComponent(msg)}` : "";

  return (
    <Drawer isOpen={isOpen} onOpenChange={onOpenChange} label="Tu carrito" width={420} padding={5} experimental={{ isFullWidthOnMobile: true }}>
      <VStack gap={4} height="100%">
        <HStack hAlign="between" vAlign="center">
          <Heading level={2}>Tu carrito</Heading>
          <Button variant="ghost" label="Cerrar" onClick={() => onOpenChange(false)} />
        </HStack>
        {items.length === 0 ? (
          <EmptyState title="Tu carrito está vacío" description="Agrega algo que te guste y aquí lo verás." icon={<ShoppingBag aria-hidden size={32} />}
            actions={<Button href="/" label="Ve las novedades" />} />
        ) : failed ? (
          <Banner status="error" title="No pudimos cargar tu carrito." description="Revisa tu conexión y vuelve a intentar." role="alert"
            endContent={<Button size="sm" label="Reintentar" onClick={() => setFailed(false)} />} />
        ) : !cat ? (
          <Text color="secondary" role="status">Cargando tu carrito…</Text>
        ) : (
          <>
            <VStack gap={0} isScrollable>
              {rows.map((r) => (
                <VStack key={r.i} gap={2} paddingBlock={3} className="cart-row">
                  <HStack gap={3} hAlign="between" vAlign="start">
                    <VStack gap={0.5}>
                      {r.p ? <Link href={r.p.u} color="primary" weight="semibold">{r.name}</Link> : <Text weight="semibold">{r.name}</Text>}
                      {r.detail && <Text type="supporting" color="secondary">{r.detail}</Text>}
                    </VStack>
                    <Text hasTabularNumbers weight="semibold">{r.available ? money(r.sub) : ""}</Text>
                  </HStack>
                  {r.warn && <Text type="supporting" color="accent">{r.warn}</Text>}
                  <HStack gap={2} hAlign="between" vAlign="center">
                    {r.available ? (
                      <HStack gap={1} vAlign="center">
                        <IconButton label={`Quitar uno de ${r.name}`} icon={<Minus aria-hidden size={16} />} size="sm" variant="secondary" onClick={() => update(r.i, -1)} />
                        <Text hasTabularNumbers aria-label={`Cantidad: ${r.it.qty}`}>{String(r.it.qty)}</Text>
                        <IconButton label={`Agregar uno de ${r.name}`} icon={<Plus aria-hidden size={16} />} size="sm" variant="secondary" onClick={() => update(r.i, +1)} isDisabled={r.it.qty >= MAX} />
                      </HStack>
                    ) : <span />}
                    <Button variant="ghost" size="sm" label={`Quitar ${r.name}`} onClick={() => update(r.i, -Infinity)}>Quitar</Button>
                  </HStack>
                </VStack>
              ))}
            </VStack>
            {lines.length > 0 && (
              <VStack gap={3} className="cart-foot">
                <Divider />
                <HStack hAlign="between" vAlign="center">
                  <Text type="large">Total</Text>
                  <Text type="large" weight="bold" hasTabularNumbers>{money(total)}</Text>
                </HStack>
                {wa ? (
                  <Button href={wa} rel="noopener" variant="primary" size="lg" width="100%" label="Enviar pedido por WhatsApp" icon={<MessageCircle aria-hidden size={18} />} />
                ) : (
                  <Text color="secondary">Esta tienda aún no tiene WhatsApp. Vuelve pronto.</Text>
                )}
              </VStack>
            )}
          </>
        )}
      </VStack>
    </Drawer>
  );
}

// ——— Marco de la tienda ———

function ShopShell({ shop, children }: { shop: ShopInfo; children: ReactNode }) {
  const cart = useCart();
  const count = cart.items.reduce((s, i) => s + i.qty, 0);
  const [bump, setBump] = useState(0);
  useEffect(() => { if (bump) { const t = setTimeout(() => setBump(0), 400); return () => clearTimeout(t); } }, [bump]);
  const api: CartAPI = { items: cart.items, add: (i) => { cart.add(i); setBump((b) => b + 1); }, open: () => cart.setOpen(true) };
  const links = [{ href: "/", name: "Todo", on: shop.home }, ...shop.categories.map((c) => ({ href: `/c/${c.id}`, name: c.name, on: shop.current === c.id })), { href: "/preguntas", name: "Preguntas", on: false }];
  return (
    <CartCtx.Provider value={api}>
      <a className="skip" href="#contenido">Ir al contenido</a>
      <header className="shop-top" aria-label={shop.name}>
        <HStack gap={3} vAlign="center" paddingInline={4} paddingBlock={4} className="shop-bar">
          {shop.logo && <img className="shop-logo" src={shop.logo} alt="" width={48} height={48} />}
          {shop.home
            ? <Heading level={1} maxLines={1}>{shop.name}</Heading>
            : <a href="/" className="shop-name">{shop.name}</a>}
        </HStack>
        <nav aria-label="Categorías" className="cats">
          <HStack as="ul" gap={2} paddingInline={4} paddingBlockEnd={3}>
            {links.map((l) => (
              <li key={l.href}><a href={l.href} className="cat" aria-current={l.on ? "page" : undefined}>{l.name}</a></li>
            ))}
          </HStack>
        </nav>
      </header>
      <VStack as="main" id="contenido" gap={5} paddingInline={4} paddingBlockStart={2} paddingBlockEnd={10} className="shop-main"
        aria-label={shop.preview ? "Vista previa de la tienda" : undefined}>
        {children}
      </VStack>
      {count > 0 && (
        <span className={"cart-fab" + (bump ? " bump" : "")}>
          <Button variant="primary" size="lg" elevation="high" label={`Carrito, ${count} ${count === 1 ? "producto" : "productos"}`}
            icon={<ShoppingBag aria-hidden size={18} />} onClick={() => cart.setOpen(true)}>{`Carrito · ${count}`}</Button>
        </span>
      )}
      <Cart items={cart.items} save={cart.save} isOpen={cart.open} onOpenChange={cart.setOpen} shop={shop} />
    </CartCtx.Provider>
  );
}

// ——— Catálogo ———

function Price({ c, big }: { c: CardData; big?: boolean }) {
  const type = big ? "large" : "body";
  if (!c.was) return <Text type={type} weight="semibold" hasTabularNumbers>{money(c.price)}</Text>;
  return (
    <HStack gap={2} vAlign="center" wrap="wrap">
      <Text type={type} weight="bold" color="accent" hasTabularNumbers>{money(c.price)}</Text>
      <Text type="supporting" color="secondary" hasStrikethrough hasTabularNumbers><span className="sr-only">antes </span>{money(c.was)}</Text>
    </HStack>
  );
}

function Tag({ c }: { c: CardData }) {
  if (c.soldOut) return <Token size="sm" label="Agotado" />;
  if (c.was) return <Token size="sm" label="Oferta" color="red" />;
  if (c.new) return <Token size="sm" label="Nuevo" color="blue" />;
  return null;
}

function ProductCard({ c, i }: { c: CardData; i: number }) {
  return (
    <li className="rise" style={{ "--i": i } as React.CSSProperties}>
      <a className={"card" + (c.soldOut ? " sold" : "")} href={`/p/${c.id}`}>
        <span className="ph" style={c.color ? { background: c.color } : undefined}>
          {c.photo.src && (
            <img src={c.photo.src} srcSet={c.photo.srcSet} sizes="(min-width: 960px) 25vw, (min-width: 640px) 33vw, 50vw" alt="" width={400} height={500}
              {...(i < 2 ? { fetchPriority: "high" as const } : { loading: "lazy" as const, decoding: "async" as const })} />
          )}
          <span className="tag"><Tag c={c} /></span>
        </span>
        <VStack gap={0.5} paddingBlockStart={2}>
          <Text weight="medium" maxLines={2}>{c.name}</Text>
          <Price c={c} />
          {c.sizes.length > 0 && !c.soldOut && <Text type="supporting" color="secondary"><span className="sr-only">Tallas: </span>{c.sizes.join(" · ")}</Text>}
        </VStack>
      </a>
    </li>
  );
}

function Grid({ cards }: { cards: CardData[] }) {
  if (!cards.length) {
    return <EmptyState title="Aún no hay productos aquí" description="Vuelve pronto: esta tienda está acomodando su vitrina." icon={<StoreIcon aria-hidden size={32} />} />;
  }
  return <ul className="grid">{cards.map((c, i) => <ProductCard key={c.id} c={c} i={i} />)}</ul>;
}

function Home({ shop, cards }: { shop: ShopInfo; cards: CardData[] }) {
  return <ShopShell shop={shop}><Grid cards={cards} /></ShopShell>;
}

function Category({ shop, title, cards }: { shop: ShopInfo; title: string; cards: CardData[] }) {
  return (
    <ShopShell shop={shop}>
      <Heading level={1}>{title}</Heading>
      <Grid cards={cards} />
    </ShopShell>
  );
}

// ——— Producto ———

function Choice({ legend, name, opts, value, onChange }: { legend: string; name: string; opts: Opt[]; value: string; onChange: (v: string) => void }) {
  return (
    <fieldset className="opts">
      <legend><Text type="label" weight="semibold">{legend}</Text></legend>
      <HStack gap={2} wrap="wrap">
        {opts.map((o) => (
          <label key={o.name} className="opt">
            <input type="radio" name={name} value={o.name} checked={value === o.name} disabled={o.soldOut} onChange={() => onChange(o.name)} />
            <span>
              {o.swatch && <span className="sw" style={{ background: o.swatch }} aria-hidden />}
              {o.name}
              {o.soldOut && <span className="sr-only"> (agotado)</span>}
            </span>
          </label>
        ))}
      </HStack>
    </fieldset>
  );
}

function ProductView({ shop, product: p }: { shop: ShopInfo; product: Product }) {
  return <ShopShell shop={shop}><ProductBody p={p} /></ShopShell>;
}

function ProductBody({ p }: { p: Product }) {
  const cart = useContext(CartCtx);
  const c = p.card;
  const [size, setSize] = useState("");
  const [color, setColor] = useState("");
  const [qty, setQty] = useState(1);
  const [status, setStatus] = useState("");
  const [current, setCurrent] = useState(0);
  const combo = size + "|" + color;
  const blocked = p.soldOutCombos.includes(combo);

  function add(ev: React.FormEvent) {
    ev.preventDefault();
    if ((p.sizes.length && !size) || (p.colors.length && !color)) return setStatus(`Elige ${p.sizes.length && !size ? "una talla" : "un color"} primero.`);
    if (blocked) return setStatus("Esa combinación está agotada. Elige otra talla o color.");
    cart.add({ id: c.id, size, color, qty: Math.min(MAX, Math.max(1, qty || 1)), price: c.price });
    setStatus("Agregado al carrito.");
  }

  return (
    <article className="product">
      {p.photos.length > 0 && (
        <VStack gap={2} className="gallery-wrap">
          <span className={"gallery" + (c.soldOut ? " sold" : "")} tabIndex={0} role="group" aria-label={`Fotos de ${c.name}`} style={c.color ? { background: c.color } : undefined}
            onScroll={(e) => { const g = e.currentTarget; setCurrent(Math.round(g.scrollLeft / g.clientWidth)); }}>
            {p.photos.map((ph, i) => (
              <img key={i} id={`foto-${i + 1}`} src={ph.src} srcSet={ph.srcSet} sizes="(min-width: 960px) 560px, 100vw" alt={i === 0 ? c.name : `${c.name}, foto ${i + 1}`}
                width={800} height={1000} {...(i === 0 ? { fetchPriority: "high" as const } : { loading: "lazy" as const, decoding: "async" as const })} />
            ))}
          </span>
          {p.photos.length > 1 && (
            <nav className="dots" aria-label="Fotos">
              {p.photos.map((_, i) => (
                <a key={i} href={`#foto-${i + 1}`} aria-current={i === current ? "true" : undefined}><span className="sr-only">{`Ver foto ${i + 1}`}</span></a>
              ))}
            </nav>
          )}
        </VStack>
      )}
      <VStack gap={4} className="info rise">
        {p.categoryName && <Link href={`/c/${p.categoryID}`} color="secondary" type="supporting">{p.categoryName}</Link>}
        <VStack gap={2}>
          <Heading level={1}>{c.name}</Heading>
          <Price c={c} big />
        </VStack>
        {c.soldOut ? (
          <Banner status="info" title="Agotado por ahora." />
        ) : (
          <form onSubmit={add} className="add" noValidate>
            <VStack gap={4}>
              {p.sizes.length > 0 && <Choice legend="Talla" name="talla" opts={p.sizes} value={size} onChange={(v) => { setSize(v); setStatus(""); }} />}
              {p.colors.length > 0 && <Choice legend="Color" name="color" opts={p.colors} value={color} onChange={(v) => { setColor(v); setStatus(""); }} />}
              {blocked && <Text color="accent">Esa combinación está agotada. Elige otra talla o color.</Text>}
              <NumberInput label="Cantidad" value={qty} onChange={setQty} min={1} max={MAX} isIntegerOnly hasNumberSteppers width={160} />
              <Button type="submit" variant="primary" size="lg" width="100%" label="Agregar al carrito" icon={<ShoppingBag aria-hidden size={18} />} />
              <Text role="status" color="secondary">{status}</Text>
            </VStack>
          </form>
        )}
        <HStack><Button href={p.share} rel="noopener" variant="ghost" label="Compartir por WhatsApp" icon={<Share2 aria-hidden size={16} />} /></HStack>
        {p.description && (
          <VStack gap={2}>
            {p.description.split("\n").filter((l) => l.trim()).map((l, i) => <Text key={i} as="p">{l}</Text>)}
          </VStack>
        )}
      </VStack>
    </article>
  );
}

// ——— Preguntas y pausa ———

function FAQ({ shop }: { shop: ShopInfo }) {
  const base = [
    { q: "¿Cómo hago mi pedido?", a: `Elige tus productos, talla y color, agrégalos al carrito y toca "Enviar pedido por WhatsApp". Se abre WhatsApp con tu pedido ya escrito; solo envíalo.` },
    { q: "¿Cómo pago?", a: `${shop.name} te responde por WhatsApp para confirmar existencias y acordar el pago.${shop.payment ? "\n\n" + shop.payment : ""}` },
    { q: "¿Cómo me llega?", a: `La entrega o el envío se acuerdan por WhatsApp con ${shop.name}.` },
    { q: "¿Mi carrito se guarda?", a: "Sí, en este celular. Si cambias de celular o borras los datos del navegador, se vacía." },
  ];
  return (
    <ShopShell shop={shop}>
      <Heading level={1}>Preguntas frecuentes</Heading>
      <VStack gap={2} className="faq">
        {[...base, ...shop.faq].map((qa, i) => (
          <Collapsible key={i} defaultIsOpen={false} className="faq-item rise" style={{ "--i": i } as React.CSSProperties}
            trigger={<Text weight="semibold">{qa.q}</Text>}>
            {qa.a && <Text as="p" color="secondary" className="pre">{qa.a}</Text>}
          </Collapsible>
        ))}
      </VStack>
    </ShopShell>
  );
}

function Paused() {
  return (
    <Center minHeight="100dvh" padding={6}>
      <VStack as="main" gap={3} maxWidth={420} className="rise">
        <Heading level={1}>Tienda en pausa</Heading>
        <Text>Esta tienda no está recibiendo pedidos por ahora. Vuelve a intentarlo más tarde.</Text>
      </VStack>
    </Center>
  );
}

const wanted = pageData().data?.shop?.style;
const style: keyof typeof THEMES = wanted in THEMES ? wanted : "vitrina";
mount({ home: Home, category: Category, product: ProductView, faq: FAQ, paused: Paused }, await THEMES[style](), style === "barberia" ? "dark" : "light");
