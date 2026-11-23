// Panel de la vendedora (/panel): productos, categorías, mi tienda y acceso. Datos desde views/panel.templ.
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Package, Tags, Store, MessageSquareText, ExternalLink, Plus, ArrowUp, ArrowDown, Trash2, Download, Upload as UploadIcon, Receipt, RotateCcw, Check as Check2, BarChart3 } from "lucide-react";
import { mount } from "./boot";
import { neutralTheme } from "@astryxdesign/theme-neutral/built";
import { Shell, AuthCard, type NavLink } from "./shell";
import { MessagesBody } from "./messages";
import { Txt, Area, Check, Pick, Msg, PostButton, Form, useWide } from "./ui";
import { Button } from "@astryxdesign/core/Button";
import { Banner } from "@astryxdesign/core/Banner";
import { Link } from "@astryxdesign/core/Link";
import { Text } from "@astryxdesign/core/Text";
import { Heading } from "@astryxdesign/core/Heading";
import { HStack, VStack } from "@astryxdesign/core/Stack";
import { Card } from "@astryxdesign/core/Card";
import { Grid } from "@astryxdesign/core/Grid";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { List, ListItem } from "@astryxdesign/core/List";
import { Token } from "@astryxdesign/core/Token";
import { Dialog } from "@astryxdesign/core/Dialog";
import { FileInput } from "@astryxdesign/core/FileInput";
import { RadioList, RadioListItem } from "@astryxdesign/core/RadioList";
import { AspectRatio } from "@astryxdesign/core/AspectRatio";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Selector } from "@astryxdesign/core/Selector";
import { Switch } from "@astryxdesign/core/Switch";
import { Table } from "@astryxdesign/core/Table";
import { TabList, Tab } from "@astryxdesign/core/TabList";
import { Badge } from "@astryxdesign/core/Badge";
import { SegmentedControl, SegmentedControlItem } from "@astryxdesign/core/SegmentedControl";

const NAV: NavLink[] = [
  { href: "/panel/ventas", label: "Ventas", icon: BarChart3 },
  { href: "/panel/", label: "Productos", icon: Package },
  { href: "/panel/categorias", label: "Categorías", icon: Tags },
  { href: "/panel/tienda", label: "Mi tienda", icon: Store },
  { href: "/panel/mensajes", label: "Mensajes", icon: MessageSquareText },
  { href: "/", label: "Ver mi tienda", icon: ExternalLink },
];

// pending: pedidos que esperan respuesta; sale como número junto a "Ventas".
type Base = { store: string; notice: string; pending: number };

function PanelShell({ store, notice, pending, ...p }: Base & { title: string; current: string; back?: { href: string; label: string }; actions?: ReactNode; width?: number; children: ReactNode }) {
  const banner = notice ? <Banner status="warning" title={notice} container="section" /> : undefined;
  const nav = NAV.map((n) => (n.href === "/panel/ventas" && pending > 0 ? { ...n, badge: pending } : n));
  return <Shell app={store} sub="Tu panel" nav={nav} logout="/panel/logout" banner={banner} {...p} />;
}

// ——— Acceso ———

function Login({ store, email, error }: { store: string; email: string; error: string }) {
  return (
    <AuthCard title="Entrar a tu panel" sub={store} footer={<Link href="/panel/recuperar">¿Olvidaste tu contraseña?</Link>}>
      <Form action="/panel/login">
        <Msg error={error} />
        <Txt name="email" label="Email" type="email" initial={email} autoComplete="username" hasAutoFocus={!email} />
        <Txt name="password" label="Contraseña" type="password" autoComplete="current-password" hasAutoFocus={!!email} />
        <Button type="submit" variant="primary" size="lg" label="Entrar" width="100%" />
      </Form>
    </AuthCard>
  );
}

const backToLogin = <Link href="/panel/login">Volver a entrar</Link>;

function Forgot({ store, enabled, sent, error }: { store: string; enabled: boolean; sent: string; error: string }) {
  return (
    <AuthCard title="Recuperar contraseña" sub={store} footer={backToLogin}>
      {!enabled ? (
        <Text>Pide a soporte un enlace para crear una nueva contraseña; te lo mandamos por WhatsApp.</Text>
      ) : sent ? (
        <Msg ok={sent} />
      ) : (
        <Form action="/panel/recuperar">
          <Msg error={error} />
          <Txt name="email" label="Email" type="email" autoComplete="username" hasAutoFocus />
          <Button type="submit" variant="primary" size="lg" label="Enviar enlace" width="100%" />
        </Form>
      )}
    </AuthCard>
  );
}

function Reset({ store, action, error }: { store: string; action: string; error: string }) {
  return (
    <AuthCard title="Nueva contraseña" sub={store}>
      <Form action={action}>
        <Msg error={error} />
        <Txt name="password" label="Nueva contraseña" type="password" autoComplete="new-password" description="Mínimo 10 caracteres." hasAutoFocus />
        <Txt name="confirm" label="Repite la contraseña" type="password" autoComplete="new-password" />
        <Button type="submit" variant="primary" size="lg" label="Guardar contraseña" width="100%" />
      </Form>
    </AuthCard>
  );
}

function ResetDone({ store }: { store: string }) {
  return (
    <AuthCard title="Contraseña cambiada" sub={store}>
      <VStack gap={4}>
        <Msg ok="Listo. Por seguridad cerramos las sesiones abiertas en otros dispositivos." />
        <Button href="/panel/login" variant="primary" size="lg" label="Entrar" width="100%" />
      </VStack>
    </AuthCard>
  );
}

function ResetExpired({ store }: { store: string }) {
  return (
    <AuthCard title="Enlace vencido" sub={store} footer={backToLogin}>
      <VStack gap={4}>
        <Text>Este enlace ya se usó o venció. Pide uno nuevo.</Text>
        <Button href="/panel/recuperar" variant="primary" size="lg" label="Pedir otro enlace" width="100%" />
      </VStack>
    </AuthCard>
  );
}

// ——— Productos ———

type StockRow = { key: string; label: string; stock: string };
type ProductRow = {
  id: number; name: string; category: string; thumb: string; price: string; was: string; active: boolean; soldOut: boolean; isNew: boolean;
  unit: string; stock: string; low: boolean; combos: StockRow[] | null;
};

const fold = (s: string) => s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();

// Visible: el interruptor envía el formulario y el servidor invierte la marca.
function VisibleSwitch({ id, name, on }: { id: number; name: string; on: boolean }) {
  return (
    <form method="post" action={`/panel/productos/${id}/marcar/active`} className="inline-form">
      <Switch label={`Visible en la tienda: ${name}`} isLabelHidden value={on} size="sm"
        onChange={(_, e) => e.currentTarget.form?.requestSubmit()} />
    </form>
  );
}

function StockToken({ p }: { p: ProductRow }) {
  if (p.stock === "") return p.soldOut ? <Token size="sm" label="Agotado" color="red" /> : <Text color="secondary" type="supporting">Sin control</Text>;
  const label = `${p.stock} ${p.unit}`;
  if (p.soldOut) return <Token size="sm" label={`Agotado · ${label}`} color="red" />;
  return <Token size="sm" label={label} color={p.low ? "orange" : "green"} />;
}

// Ajuste rápido: corregir existencias sin abrir el producto completo.
function StockDialog({ p, onClose }: { p: ProductRow | null; onClose: () => void }) {
  return (
    <Dialog isOpen={!!p} onOpenChange={(o) => { if (!o) onClose(); }} width={440} padding={5}>
      {p && (
        <Form action={`/panel/productos/${p.id}/existencias`}>
          <Heading level={2}>{`Existencias de ${p.name}`}</Heading>
          <Text color="secondary">{`En ${p.unit}. Escribe lo que tienes ahora; se descuenta solo cuando marcas un pedido como vendido.`}</Text>
          {p.combos && p.combos.length > 0 ? (
            <Grid columns={{ minWidth: 120 }} gap={3}>
              {p.combos.map((c) => <Txt key={c.key} name={`stock-${c.key}`} label={c.label || "Única"} initial={c.stock} inputMode="decimal" />)}
            </Grid>
          ) : (
            <Txt name="stock" label={`Existencia (${p.unit})`} initial={p.stock} inputMode="decimal" />
          )}
          <HStack gap={2} hAlign="end">
            <Button variant="ghost" label="Cancelar" onClick={onClose} />
            <Button type="submit" variant="primary" label="Guardar existencias" />
          </HStack>
        </Form>
      )}
    </Dialog>
  );
}

function Products({ products, categories, ...b }: Base & { products: ProductRow[]; categories: Opt[] }) {
  const wide = useWide();
  const [q, setQ] = useState("");
  const [cat, setCat] = useState("");
  const [show, setShow] = useState("todos");
  const [adjust, setAdjust] = useState<ProductRow | null>(null);
  const saved = new URLSearchParams(location.search).get("existencia");
  const add = <Button href="/panel/productos/nuevo" variant="primary" label="Agregar producto" icon={<Plus aria-hidden size={16} />} />;
  const catName = categories.find((c) => c.value === cat)?.label;
  const rows = products.filter((p) =>
    (!q.trim() || fold(p.name).includes(fold(q.trim()))) &&
    (!cat || p.category === catName) &&
    (show === "todos" || (show === "poca" && p.low && !p.soldOut) || (show === "agotados" && p.soldOut) || (show === "ocultos" && !p.active)));
  const count = (f: (p: ProductRow) => boolean) => products.filter(f).length;

  return (
    <PanelShell {...b} title="Productos" current="/panel/" width={1040} actions={products.length > 0 ? add : undefined}>
      {saved && <Msg ok="Existencias guardadas." />}
      {products.length === 0 ? (
        <Card className="rise">
          <EmptyState title="Aún no tienes productos" description="Agrega el primero; luego puedes ordenarlos por categoría." icon={<Package aria-hidden size={32} />} actions={add} />
        </Card>
      ) : (
        <VStack gap={4}>
          <Grid columns={{ minWidth: 220, repeat: "fit" }} gap={3}>
            <TextInput label="Buscar" placeholder="Nombre del producto" value={q} onChange={setQ} startIcon="search" />
            {categories.length > 0 && (
              <Selector label="Categoría" options={[{ value: "", label: "Todas" }, ...categories]} value={cat} onChange={(v) => setCat(v ?? "")} presentation="adaptive" width="100%" />
            )}
          </Grid>
          <TabList value={show} onChange={setShow} hasDivider role="tablist">
            <Tab value="todos" label="Todos" endContent={<Badge label={products.length} />} />
            <Tab value="poca" label="Poca existencia" endContent={<Badge label={count((p) => p.low && !p.soldOut)} />} />
            <Tab value="agotados" label="Agotados" endContent={<Badge label={count((p) => p.soldOut)} />} />
            <Tab value="ocultos" label="Ocultos" endContent={<Badge label={count((p) => !p.active)} />} />
          </TabList>
          <Card padding={0} className="rise">
            <Table data={rows} idKey="id" hasHover
              emptyState={<EmptyState title="Nada por aquí" description="Prueba con otra búsqueda o filtro." isCompact />}
              columns={!wide ? [
                { key: "name", header: "Producto", renderCell: (p) => (
                  <HStack gap={3} vAlign="start">
                    {p.thumb ? <img src={p.thumb} alt="" width={44} height={55} className="thumb" loading="lazy" /> : <Package aria-hidden size={24} className="thumb-empty" />}
                    <VStack gap={1}>
                      <Link href={`/panel/productos/${p.id}`} weight="semibold">{p.name}</Link>
                      <HStack gap={2} vAlign="center" wrap="wrap">
                        <Text hasTabularNumbers type="supporting">{p.price}{p.unit !== "pza" ? ` / ${p.unit}` : ""}</Text>
                        <StockToken p={p} />
                      </HStack>
                      <HStack gap={2} vAlign="center" wrap="wrap">
                        <VisibleSwitch id={p.id} name={p.name} on={p.active} />
                        {p.stock !== "" && <Button variant="ghost" size="sm" label="Ajustar existencias" onClick={() => setAdjust(p)} />}
                      </HStack>
                    </VStack>
                  </HStack>
                ) },
              ] : [
                { key: "name", header: "Producto", renderCell: (p) => (
                  <HStack gap={3} vAlign="center">
                    {p.thumb ? <img src={p.thumb} alt="" width={44} height={55} className="thumb" loading="lazy" /> : <Package aria-hidden size={24} className="thumb-empty" />}
                    <VStack gap={0.5}>
                      <Link href={`/panel/productos/${p.id}`} weight="semibold">{p.name}</Link>
                      <HStack gap={2} vAlign="center" wrap="wrap">
                        <Text hasTabularNumbers type="supporting">{p.price}{p.unit !== "pza" ? ` / ${p.unit}` : ""}</Text>
                        {p.was && <Text hasTabularNumbers hasStrikethrough type="supporting" color="secondary">{p.was}</Text>}
                        {p.category && <Text type="supporting" color="secondary">{p.category}</Text>}
                        {p.isNew && <Token size="sm" label="Nuevo" color="blue" />}
                      </HStack>
                    </VStack>
                  </HStack>
                ) },
                { key: "stock", header: "Existencia", renderCell: (p) => <StockToken p={p} /> },
                { key: "active", header: "Visible", renderCell: (p) => <VisibleSwitch id={p.id} name={p.name} on={p.active} /> },
                { key: "actions", header: "Acciones", align: "end", renderCell: (p) => (
                  <HStack gap={1} hAlign="end">
                    {p.stock !== "" && <Button variant="ghost" size="sm" label="Ajustar" onClick={() => setAdjust(p)} />}
                    <Button href={`/panel/productos/${p.id}`} variant="ghost" size="sm" label="Editar" />
                  </HStack>
                ) },
              ]}
            />
          </Card>
        </VStack>
      )}
      <StockDialog p={adjust} onClose={() => setAdjust(null)} />
    </PanelShell>
  );
}

type Opt = { value: string; label: string };
type Photo = { ID: number; Src: string; SrcSet: string };
type ProductForm = {
  ID: number; Name: string; Description: string; Price: string; SalePrice: string; Sizes: string; Colors: string;
  Variants: { Key: string; Label: string; SoldOut: boolean; Stock: string }[] | null; Error: string; Field: string; Saved: boolean;
  Unit: string; Step: string; Stock: string; Track: boolean; OnSale: boolean; IsNew: boolean; SoldOut: boolean;
  Photos: Photo[] | null; PhotoOK: boolean; PhotoError: string;
};

const UNITS = [
  { value: "pieza", label: "Por pieza", short: "pza" },
  { value: "kg", label: "Por kilo (kg)", short: "kg" },
  { value: "g", label: "Por gramo (g)", short: "g" },
  { value: "litro", label: "Por litro", short: "L" },
  { value: "metro", label: "Por metro", short: "m" },
];

// Igual que splitList en Go: separa por comas, sin repetidos ni "|".
const splitList = (s: string) => {
  const seen = new Set<string>();
  return s.split(",").map((x) => x.replaceAll("|", "").trim()).filter((x) => x && !seen.has(x.toLowerCase()) && seen.add(x.toLowerCase()));
};
// Igual que variantMatrix en Go: todas las combinaciones talla × color.
function matrix(sizes: string[], colors: string[]) {
  if (!sizes.length && !colors.length) return [];
  const ss = sizes.length ? sizes : [""], cs = colors.length ? colors : [""];
  return cs.flatMap((c) => ss.map((s) => ({ key: `${s}|${c}`, label: `${s} ${c}`.trim() })));
}

const TAB_OF: Record<string, string> = { name: "datos", category_id: "datos", description: "datos" };

function ProductEdit({ form: f, category, categories, ...b }: Base & { form: ProductForm; category: string; categories: Opt[] }) {
  const isNew = f.ID === 0;
  const params = new URLSearchParams(location.search);
  const [tab, setTab] = useState(f.Error ? TAB_OF[f.Field] ?? "precio" : params.has("foto") ? "fotos" : "datos");
  const [deleting, setDeleting] = useState(false);
  const [unit, setUnit] = useState(f.Unit || "pieza");
  const [sizes, setSizes] = useState(f.Sizes);
  const [colors, setColors] = useState(f.Colors);
  const [track, setTrack] = useState(f.Track);
  const then = useRef<HTMLInputElement>(null);
  const short = UNITS.find((u) => u.value === unit)?.short ?? "pza";
  const combos = matrix(splitList(sizes), splitList(colors));
  const prev = new Map((f.Variants ?? []).map((v) => [v.Key, v]));
  const ok = f.Saved ? (isNew ? "Producto guardado. Ya puedes agregar el siguiente." : "Producto guardado.") : undefined;

  return (
    <PanelShell {...b} title={isNew ? "Nuevo producto" : f.Name || "Editar producto"} current="/panel/" back={{ href: "/panel/", label: "Productos" }}>
      <Msg error={f.Error} ok={ok} />
      <TabList value={tab} onChange={setTab} hasDivider role="tablist">
        <Tab value="datos" label="Datos" panelId="p-datos" />
        <Tab value="precio" label="Precio y existencias" panelId="p-precio" />
        <Tab value="fotos" label="Fotos" panelId="p-fotos" endContent={f.Photos?.length ? <Badge label={f.Photos.length} /> : undefined} />
      </TabList>

      <form method="post" action={isNew ? "/panel/productos" : `/panel/productos/${f.ID}`} noValidate hidden={tab === "fotos"}>
        <input ref={then} type="hidden" name="then" value="" />
        <VStack gap={5}>
          {/* Los dos paneles siguen en el formulario aunque estén ocultos: todo se guarda junto. */}
          <section hidden={tab !== "datos"} id="p-datos" role="tabpanel" aria-label="Datos">
          <Card padding={5} className="rise">
            <VStack gap={5}>
              <Txt name="name" label="Nombre" initial={f.Name} placeholder="Blusa bordada" />
              <Pick name="category_id" label="Categoría" initial={category} placeholder="Sin categoría" options={[{ value: "", label: "Sin categoría" }, ...categories]} />
              <Area name="description" label="Descripción" isOptional initial={f.Description} rows={5} maxLength={4000} description="Material, medidas, cuidados… lo que te suelen preguntar." />
              <Check name="is_new" label="Marcar como nuevo" description="Sale con la etiqueta Nuevo en tu tienda." initial={isNew || f.IsNew} />
            </VStack>
          </Card>
          </section>

          <section hidden={tab !== "precio"} id="p-precio" role="tabpanel" aria-label="Precio y existencias">
          <Card padding={5} className="rise">
            <VStack gap={5}>
              <Grid columns={{ minWidth: 200, repeat: "fit" }} gap={4}>
                <VStack gap={1}>
                  <Selector label="Cómo se vende" options={UNITS} value={unit} onChange={(v) => setUnit(v ?? "pieza")} presentation="adaptive" width="100%" />
                  <input type="hidden" name="unit" value={unit} />
                </VStack>
                <Txt name="step" label={`De cuánto en cuánto (${short})`} initial={f.Step || "1"} inputMode="decimal"
                  description={unit === "pieza" ? "Normalmente 1." : "Ej. 0.25 para vender de cuarto en cuarto."} />
              </Grid>
              <Grid columns={{ minWidth: 200, repeat: "fit" }} gap={4}>
                <Txt name="price" label={`Precio por ${unit === "pieza" ? "pieza" : short}`} initial={f.Price} inputMode="decimal" placeholder="289" />
                <Txt name="sale_price" label="Precio de oferta" isOptional initial={f.SalePrice} inputMode="decimal" placeholder="249"
                  description="Si lo llenas, sale tachado el precio normal." />
              </Grid>
              <Grid columns={{ minWidth: 200, repeat: "fit" }} gap={4}>
                <TextInput label="Tallas" isOptional htmlName="sizes" value={sizes} onChange={setSizes} placeholder="CH, M, G" description="Sepáralas con comas." />
                <TextInput label="Colores" isOptional htmlName="colors" value={colors} onChange={setColors} placeholder="Negro, Blanco" description="Sepáralos con comas." />
              </Grid>
              <Switch label="Llevar inventario" htmlName="track" value={track} onChange={setTrack}
                description="Tu tienda muestra solo lo que hay y se descuenta solo al marcar un pedido como vendido." />
              {track ? (
                combos.length > 0 ? (
                  <Card variant="muted" padding={4}>
                    <VStack gap={3}>
                      <Text weight="semibold">{`¿Cuántas tienes de cada una? (${short})`}</Text>
                      <Grid columns={{ minWidth: 120 }} gap={3}>
                        {combos.map((c) => <Txt key={c.key} name={`stock-${c.key}`} label={c.label} initial={prev.get(c.key)?.Stock ?? ""} inputMode="decimal" placeholder="0" />)}
                      </Grid>
                    </VStack>
                  </Card>
                ) : (
                  <Txt name="stock" label={`Existencia (${short})`} initial={f.Stock} inputMode="decimal" placeholder="0" />
                )
              ) : combos.length > 0 ? (
                <Card variant="muted" padding={4}>
                  <VStack gap={3}>
                    <Text weight="semibold">¿Qué combinaciones están agotadas?</Text>
                    <Grid columns={{ minWidth: 140 }} gap={2}>
                      {combos.map((c) => <Check key={c.key} name="soldout" value={c.key} label={c.label} initial={prev.get(c.key)?.SoldOut ?? false} />)}
                    </Grid>
                  </VStack>
                </Card>
              ) : (
                <Check name="sold_out" label="Agotado" description="No se puede pedir hasta que lo desmarques." initial={f.SoldOut} />
              )}
            </VStack>
          </Card>
          </section>

          <Card padding={4} className="save-bar">
            <HStack gap={2} wrap="wrap" hAlign="end">
              <Button type="submit" label="Guardar y agregar otro" onClick={() => { if (then.current) then.current.value = "nuevo"; }} />
              <Button type="submit" variant="primary" label="Guardar producto" onClick={() => { if (then.current) then.current.value = ""; }} />
            </HStack>
          </Card>
        </VStack>
      </form>

      <section hidden={tab !== "fotos"} id="p-fotos" role="tabpanel" aria-label="Fotos">
        {isNew ? (
          <Card padding={5}><EmptyState title="Primero guarda el producto" description="Después de guardarlo puedes subir hasta 10 fotos." icon={<Package aria-hidden size={32} />} isCompact /></Card>
        ) : <Photos f={f} />}
      </section>

      {!isNew && tab === "datos" && (
        <Card padding={5} className="rise">
          <HStack gap={3} hAlign="between" vAlign="center" wrap="wrap">
            <VStack gap={1}>
              <Heading level={2}>Borrar producto</Heading>
              <Text color="secondary">Se borra para siempre, con sus fotos. Sus ventas pasadas se conservan.</Text>
            </VStack>
            <Button variant="destructive" label="Borrar producto" icon={<Trash2 aria-hidden size={16} />} onClick={() => setDeleting(true)} />
          </HStack>
          <Dialog isOpen={deleting} onOpenChange={setDeleting} width={420} padding={5}>
            <VStack gap={4}>
              <Heading level={2}>{`¿Borrar ${f.Name}?`}</Heading>
              <Text>Se borra para siempre, con sus fotos. No se puede deshacer.</Text>
              <HStack gap={2} hAlign="end">
                <Button variant="ghost" label="Cancelar" onClick={() => setDeleting(false)} />
                <PostButton action={`/panel/productos/${f.ID}/borrar`} variant="destructive" label="Sí, borrar" />
              </HStack>
            </VStack>
          </Dialog>
        </Card>
      )}
    </PanelShell>
  );
}

function Photos({ f }: { f: ProductForm }) {
  const photos = f.Photos ?? [];
  return (
    <Card padding={5} className="rise">
      <VStack gap={4}>
        <HStack gap={2} vAlign="center">
          <Heading level={2}>Fotos</Heading>
          <Text color="secondary">{`${photos.length} de 10`}</Text>
        </HStack>
        <Msg error={f.PhotoError} ok={f.PhotoOK ? "Foto agregada." : undefined} />
        {photos.length > 0 && (
          <Grid columns={{ minWidth: 140 }} gap={3}>
            {photos.map((p, i) => (
              <Card key={p.ID} padding={0} className="rise" style={{ "--i": i } as React.CSSProperties}>
                <VStack gap={0}>
                  <AspectRatio ratio={4 / 5} fit="cover">
                    <img src={p.Src} srcSet={p.SrcSet} sizes="(min-width: 576px) 200px, 45vw" alt={`Foto ${i + 1} de ${f.Name}`} loading="lazy" decoding="async" width={400} height={500} />
                  </AspectRatio>
                  <VStack padding={2}>
                    <PostButton action={`/panel/fotos/${p.ID}/borrar`} variant="ghost" size="sm" label={`Quitar foto ${i + 1}`} width="100%">Quitar</PostButton>
                  </VStack>
                </VStack>
              </Card>
            ))}
          </Grid>
        )}
        {photos.length < 10 && (
          <Upload action={`/panel/productos/${f.ID}/fotos`} field="foto" label="Agregar foto" submit="Subir foto" shrink
            description="JPG, PNG o WebP. La reducimos en tu celular antes de subirla para ahorrar datos." />
        )}
      </VStack>
    </Card>
  );
}

// Upload: sube una imagen con fetch (el servidor valida y recodifica todo) y sigue la redirección del servidor.
// shrink reduce la foto en el celular (lado mayor 1600 px, WebP) antes de subirla, como hacía fotos.js.
function Upload({ action, field, label, submit, description, shrink }: { action: string; field: string; label: string; submit: string; description?: string; shrink?: boolean }) {
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function send(ev: React.FormEvent<HTMLFormElement>) {
    ev.preventDefault();
    if (!file) return setError("Elige una imagen primero.");
    setBusy(true);
    setError("");
    try {
      const data = new FormData();
      data.set(field, shrink ? await reduce(file) : file, shrink ? "foto.webp" : file.name);
      const res = await fetch(action, { method: "POST", body: data });
      location.assign(res.url);
    } catch {
      setBusy(false);
      setError("No se pudo subir. Revisa tu conexión y vuelve a intentar.");
    }
  }
  return (
    <form method="post" action={action} encType="multipart/form-data" onSubmit={send} noValidate>
      <VStack gap={3}>
        <FileInput label={label} description={description} accept="image/jpeg,image/png,image/webp" value={file}
          onChange={(x) => setFile(Array.isArray(x) ? x[0] ?? null : x)} mode="dropzone"
          status={error ? { type: "error", message: error } : undefined} />
        <HStack><Button type="submit" variant="primary" label={busy ? "Subiendo…" : submit} isLoading={busy} icon={<UploadIcon aria-hidden size={16} />} /></HStack>
        {busy && <Text role="status" color="secondary">Preparando y subiendo la foto…</Text>}
      </VStack>
    </form>
  );
}

async function reduce(file: File): Promise<Blob> {
  if (!("createImageBitmap" in window)) return file;
  try {
    const bmp = await createImageBitmap(file, { imageOrientation: "from-image" });
    const scale = Math.min(1, 1600 / Math.max(bmp.width, bmp.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.round(bmp.width * scale);
    canvas.height = Math.round(bmp.height * scale);
    canvas.getContext("2d")!.drawImage(bmp, 0, 0, canvas.width, canvas.height);
    bmp.close();
    const blob = await new Promise<Blob | null>((ok) => canvas.toBlob(ok, "image/webp", 0.85));
    // Safari viejo no codifica WebP y devuelve PNG; si sale más pesado, mandamos el original.
    return blob && blob.size < file.size ? blob : file;
  } catch {
    return file;
  }
}

// ——— Categorías ———

function Categories({ categories, error, ...b }: Base & { categories: Opt[]; error: string }) {
  return (
    <PanelShell {...b} title="Categorías" current="/panel/categorias">
      <Card padding={5} className="rise">
        <Form action="/panel/categorias" gap={3}>
          <Msg error={error} />
          <HStack gap={2} vAlign="end">
            <VStack width="100%"><Txt name="name" label="Nueva categoría" placeholder="Ej. Blusas" /></VStack>
            <Button type="submit" variant="primary" label="Agregar" icon={<Plus aria-hidden size={16} />} />
          </HStack>
        </Form>
      </Card>
      {categories.length === 0 ? (
        <Card className="rise"><EmptyState title="Sin categorías" description="Agrúpalas como en tu mostrador: blusas, vestidos, accesorios…" icon={<Tags aria-hidden size={32} />} /></Card>
      ) : (
        <Card padding={0} className="rise">
          <List hasDividers>
            {categories.map((c, i) => (
              <ListItem key={c.value} label={<CategoryRow c={c} first={i === 0} last={i === categories.length - 1} />} />
            ))}
          </List>
        </Card>
      )}
    </PanelShell>
  );
}

function CategoryRow({ c, first, last }: { c: Opt; first: boolean; last: boolean }) {
  const [name, setName] = useState(c.label);
  const base = `/panel/categorias/${c.value}`;
  return (
    <HStack gap={2} vAlign="end" wrap="wrap" paddingBlock={1}>
      <form method="post" action={`${base}/renombrar`} className="grow-form">
        <HStack gap={2} vAlign="end">
          <VStack width="100%"><TextInput label="Nombre de la categoría" isLabelHidden htmlName="name" value={name} onChange={setName} /></VStack>
          <Button type="submit" label="Guardar" isDisabled={name === c.label} />
        </HStack>
      </form>
      <HStack gap={1}>
        <PostButton action={`${base}/mover`} fields={{ dir: "up" }} label={`Subir ${c.label}`} isIconOnly icon={<ArrowUp aria-hidden size={16} />} variant="ghost" isDisabled={first} />
        <PostButton action={`${base}/mover`} fields={{ dir: "down" }} label={`Bajar ${c.label}`} isIconOnly icon={<ArrowDown aria-hidden size={16} />} variant="ghost" isDisabled={last} />
        <PostButton action={`${base}/borrar`} label={`Borrar ${c.label}`} isIconOnly icon={<Trash2 aria-hidden size={16} />} variant="ghost" />
      </HStack>
    </HStack>
  );
}

// ——— Mi tienda ———

type Style = { ID: string; Name: string; Hint: string; Brand: string };
type StoreForm = {
  Name: string; ShopURL: string; Whatsapp: string; Color: string; Applied: string; FAQ: string; Payment: string; Style: string;
  Logo: Photo; Error: string; LogoError: string; Saved: boolean; LogoOK: boolean;
};

function MyStore({ form: f, adjusted, styles, ...b }: Base & { form: StoreForm; adjusted: boolean; styles: Style[] }) {
  const [name, setName] = useState(f.Name);
  const [color, setColor] = useState(f.Color);
  const [style, setStyle] = useState(f.Style);
  const [preview, setPreview] = useState("/panel/tienda/vista");
  const first = useRef(true);
  useEffect(() => {
    if (first.current) return void (first.current = false);
    const t = setTimeout(() => setPreview("/panel/tienda/vista?" + new URLSearchParams({ nombre: name, color, estilo: style })), 300);
    return () => clearTimeout(t);
  }, [name, color, style]);

  const warn = adjusted ? (
    <VStack gap={2}>
      <Text>Ajustamos un poco tu color para que los botones se lean bien en cualquier celular.</Text>
      <HStack gap={4} wrap="wrap">
        <Swatch color={f.Color} label={`Antes ${f.Color}`} />
        <Swatch color={f.Applied} label={`Ahora ${f.Applied}`} />
      </HStack>
    </VStack>
  ) : undefined;

  return (
    <PanelShell {...b} title="Mi tienda" current="/panel/tienda" width={1080}
      actions={<Button href={f.ShopURL} label="Ver mi tienda" icon={<ExternalLink aria-hidden size={16} />} target="_blank" rel="noopener" />}>
      <Text color="secondary">Tu tienda: <Link href={f.ShopURL}>{f.ShopURL}</Link></Text>
      <Grid columns={{ minWidth: 340 }} gap={5}>
        <Card padding={5} className="rise">
          <Form action="/panel/tienda" gap={5}>
            <Msg error={f.Error} ok={f.Saved ? "Datos guardados." : undefined} warn={warn} />
            <TextInput htmlName="name" label="Nombre de la tienda" value={name} onChange={setName} />
            <Txt name="whatsapp" label="WhatsApp para recibir pedidos" type="tel" inputMode="tel" autoComplete="tel" initial={f.Whatsapp}
              description="10 dígitos si es de México, por ejemplo 55 1234 5678. Si es de otro país, empieza con + y la lada." />
            <VStack gap={1}>
              <label htmlFor="color"><Text type="label" weight="semibold">Color de tu marca</Text></label>
              <Text type="supporting" color="secondary" id="color-help">Lo usamos en los botones. Si no se lee bien, lo ajustamos un poco.</Text>
              <input id="color" name="color" type="color" value={color} onChange={(e) => setColor(e.target.value)} aria-describedby="color-help" className="color-input" />
            </VStack>
            <RadioList label="Estilo de tu tienda" description="Elige el que va con tu giro. Al elegirlo te sugerimos su color; puedes cambiarlo."
              htmlName="style" value={style} onChange={(v) => { setStyle(v); setColor(styles.find((s) => s.ID === v)?.Brand ?? color); }}>
              {styles.map((s) => <RadioListItem key={s.ID} value={s.ID} label={s.Name} description={s.Hint} endContent={<Swatch color={s.Brand} />} />)}
            </RadioList>
            <Area name="payment" label="Datos para pagar por transferencia (opcional)" initial={f.Payment} rows={3} maxLength={1000}
              description="Banco, CLABE y a nombre de quién. Se muestran en las preguntas frecuentes de tu tienda." />
            <Area name="faq" label="Tus preguntas frecuentes (opcional)" initial={f.FAQ} rows={6} maxLength={4000}
              description="Escribe la pregunta en una línea y la respuesta abajo. Deja una línea vacía entre preguntas." />
            <HStack><Button type="submit" variant="primary" size="lg" label="Guardar datos" /></HStack>
          </Form>
        </Card>

        <VStack gap={5}>
          <Card padding={5} className="rise">
            <VStack gap={3}>
              <Heading level={2}>Vista previa</Heading>
              <Text color="secondary">Cambia el nombre, el color o el estilo y aquí ves cómo queda, antes de guardar.</Text>
              <iframe src={preview} title="Vista previa de tu tienda" loading="lazy" className="preview-frame" />
            </VStack>
          </Card>

          <Card padding={5} className="rise">
            <VStack gap={4}>
              <Heading level={2}>Logo</Heading>
              <Msg error={f.LogoError} ok={f.LogoOK ? "Logo guardado." : undefined} />
              {f.Logo.Src && (
                <HStack gap={3} vAlign="center">
                  <img src={f.Logo.Src} alt={`Logo de ${f.Name}`} width={80} height={80} className="logo-preview" />
                  <PostButton action="/panel/tienda/logo/borrar" variant="ghost" label="Quitar logo" icon={<Trash2 aria-hidden size={16} />} />
                </HStack>
              )}
              <Upload action="/panel/tienda/logo" field="logo" label={f.Logo.Src ? "Cambiar logo" : "Subir logo"} submit="Subir logo" />
            </VStack>
          </Card>

          <Card padding={5} className="rise">
            <VStack gap={3}>
              <Heading level={2}>QR de tu tienda</Heading>
              <Text color="secondary">Imprímelo o compártelo; al escanearlo abre tu tienda.</Text>
              <img src="/panel/tienda/qr.png" alt={`Código QR que abre ${f.ShopURL}`} width={200} height={200} loading="lazy" className="qr" />
              <HStack><Button href="/panel/tienda/qr.png?descargar" label="Descargar QR" icon={<Download aria-hidden size={16} />} /></HStack>
            </VStack>
          </Card>
        </VStack>
      </Grid>
    </PanelShell>
  );
}

function Swatch({ color, label }: { color: string; label?: string }) {
  const dot = <span className="swatch" style={{ background: color }} aria-hidden />;
  return label ? <HStack gap={2} vAlign="center">{dot}<Text>{label}</Text></HStack> : dot;
}

function Messages({ shopURL, payment, ...b }: Base & { shopURL: string; payment: string }) {
  return (
    <PanelShell {...b} title="Mensajes para tus clientas" current="/panel/mensajes">
      <MessagesBody store={b.store} shopURL={shopURL} payment={payment} />
    </PanelShell>
  );
}

// ——— Ventas ———

type OrderItem = { id: number; name: string; variant: string; qty: string; step: string; unit: string; price: string; line: string; stock: string; short: boolean };
type OrderView = { id: number; code: string; created: string; total: string; reason: string; items: OrderItem[] | null };

const REASONS = [
  { value: "1", label: "Sin existencia", description: "No tenía lo que pidió." },
  { value: "2", label: "La clienta no respondió", description: "No contestó o no confirmó." },
  { value: "3", label: "Precio", description: "Le pareció caro o buscaba otro precio." },
  { value: "4", label: "Otro", description: "Cualquier otra razón." },
];

function SalesTabs({ current, pending }: { current: string; pending: number }) {
  return (
    <TabList value={current} onChange={() => {}} hasDivider>
      <Tab value="resumen" label="Resumen" href="/panel/ventas/resumen" />
      <Tab value="pendientes" label="Pendientes" href="/panel/ventas?estado=pendientes" endContent={pending > 0 ? <Badge label={pending} variant="warning" /> : undefined} />
      <Tab value="vendidos" label="Vendidos" href="/panel/ventas?estado=vendidos" />
      <Tab value="no-vendidos" label="No vendidos" href="/panel/ventas?estado=no-vendidos" />
    </TabList>
  );
}

function LoseDialog({ order, onClose }: { order: OrderView | null; onClose: () => void }) {
  const [reason, setReason] = useState("2");
  return (
    <Dialog isOpen={!!order} onOpenChange={(o) => { if (!o) onClose(); }} width={440} padding={5}>
      {order && (
        <Form action={`/panel/ventas/${order.id}/no-vender`}>
          <Heading level={2}>{`¿Por qué no se vendió el pedido ${order.code}?`}</Heading>
          <Text color="secondary">Queda en "No vendidos" para que veas qué oportunidades se fueron. No cambia tus existencias.</Text>
          <RadioList label="Motivo" htmlName="reason" value={reason} onChange={setReason}>
            {REASONS.map((r) => <RadioListItem key={r.value} value={r.value} label={r.label} description={r.description} />)}
          </RadioList>
          <HStack gap={2} hAlign="end">
            <Button variant="ghost" label="Cancelar" onClick={onClose} />
            <Button type="submit" variant="primary" label="Guardar como no vendido" />
          </HStack>
        </Form>
      )}
    </Dialog>
  );
}

function OrderCard({ o, tab, focus, onLose }: { o: OrderView; tab: string; focus: boolean; onLose: () => void }) {
  const [editing, setEditing] = useState(false);
  const items = o.items ?? [];
  const short = items.some((i) => i.short);
  const ref = useRef<HTMLElement>(null);
  useEffect(() => { if (focus) ref.current?.scrollIntoView({ block: "center" }); }, [focus]);
  return (
    <Card padding={4} className={"rise" + (focus ? " order-focus" : "")}>
      <article ref={ref} aria-labelledby={`o-${o.id}`}>
      <VStack gap={3}>
        <HStack gap={3} hAlign="between" vAlign="start" wrap="wrap">
          <VStack gap={0.5}>
            <Heading level={2} id={`o-${o.id}`}>{`Pedido ${o.code}`}</Heading>
            <Text type="supporting" color="secondary">{o.created}</Text>
          </VStack>
          <VStack gap={0.5} hAlign="end">
            <Text type="large" weight="semibold" hasTabularNumbers>{o.total}</Text>
            {o.reason && <Token size="sm" label={o.reason} color="gray" />}
          </VStack>
        </HStack>

        {editing ? (
          <Form action={`/panel/ventas/${o.id}/ajustar`} gap={3}>
            <Text color="secondary">Cambia las cantidades como quedaron en la plática. Pon 0 para quitar un producto.</Text>
            {items.map((i) => (
              <Txt key={i.id} name={`qty-${i.id}`} label={`${i.name}${i.variant ? ` · ${i.variant}` : ""} (${i.unit})`} initial={i.qty} inputMode="decimal"
                description={`De ${i.step} en ${i.step}. ${i.price} c/u.${i.stock ? ` Hay ${i.stock} ${i.unit}.` : ""}`} />
            ))}
            <HStack gap={2} hAlign="end">
              <Button variant="ghost" label="Cancelar" onClick={() => setEditing(false)} />
              <Button type="submit" variant="primary" label="Guardar cambios" />
            </HStack>
          </Form>
        ) : (
          <List>
            {items.map((i) => (
              <ListItem key={i.id} label={`${i.name}${i.variant ? ` · ${i.variant}` : ""}`}
                description={`${i.qty} ${i.unit} × ${i.price}${tab === "pendientes" && i.stock ? ` · hay ${i.stock} ${i.unit}` : ""}`}
                endContent={<HStack gap={2} vAlign="center">{i.short && tab === "pendientes" && <Token size="sm" label="No alcanza" color="red" />}<Text hasTabularNumbers>{i.line}</Text></HStack>} />
            ))}
          </List>
        )}

        {!editing && (
          <HStack gap={2} wrap="wrap">
            {tab === "pendientes" ? (
              <>
                <PostButton action={`/panel/ventas/${o.id}/vender`} variant="primary" label="Marcar como vendido" icon={<Check2 aria-hidden size={16} />} isDisabled={short || items.length === 0} />
                <Button label="No se vendió" onClick={onLose} />
                <Button variant="ghost" label="Ajustar cantidades" onClick={() => setEditing(true)} />
              </>
            ) : (
              <PostButton action={`/panel/ventas/${o.id}/deshacer`} variant="ghost" label="Regresar a pendiente"
                icon={<RotateCcw aria-hidden size={16} />} />
            )}
          </HStack>
        )}
        {!editing && tab === "pendientes" && short && <Text type="supporting" color="secondary">Para venderlo, ajusta el pedido o sube la existencia del producto.</Text>}
      </VStack>
      </article>
    </Card>
  );
}

function Sales({ tab, orders, error, focus, pending, ...b }: Base & { tab: string; orders: OrderView[]; error: string; focus: number }) {
  const [losing, setLosing] = useState<OrderView | null>(null);
  const empty = {
    pendientes: ["No tienes pedidos pendientes", "Cuando una clienta envíe su pedido por WhatsApp, aparece aquí para que lo marques como vendido o no vendido."],
    vendidos: ["Aún no hay ventas registradas", "Los pedidos que marques como vendidos aparecen aquí y descuentan tus existencias."],
    "no-vendidos": ["Sin oportunidades perdidas", "Aquí quedan los pedidos que no se cerraron, con su motivo."],
  }[tab] ?? ["", ""];
  return (
    <PanelShell {...b} pending={pending} title="Ventas" current="/panel/ventas" width={880}>
      <SalesTabs current={tab} pending={pending} />
      <Msg error={error} />
      {orders.length === 0 ? (
        <Card className="rise"><EmptyState title={empty[0]} description={empty[1]} icon={<Receipt aria-hidden size={32} />} /></Card>
      ) : (
        <VStack gap={3}>
          {orders.map((o) => <OrderCard key={o.id} o={o} tab={tab} focus={o.id === focus} onLose={() => setLosing(o)} />)}
        </VStack>
      )}
      <LoseDialog order={losing} onClose={() => setLosing(null)} />
    </PanelShell>
  );
}

type ItemRow = { id?: number; name: string; variant: string; qty: string; unit: string; cents: number };
type Summary = {
  period: string; periods: Opt[]; revenue: number; lostCents: number; ticket: number; sold: number; lost: number; pending: number; conversion: number;
  series: { label: string; sold: number; lost: number }[] | null; top: ItemRow[] | null; topLost: ItemRow[] | null; categories: ItemRow[] | null;
  reasons: { label: string; orders: number; cents: number }[] | null; lowStock: ItemRow[] | null;
};

const mxn = new Intl.NumberFormat("es-MX", { style: "currency", currency: "MXN", maximumFractionDigits: 0 });
const pesos = (cents: number) => mxn.format(Math.round(cents / 100));

function Kpi({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <Card className="rise">
      <VStack gap={1}>
        <Text type="supporting" color="secondary">{label}</Text>
        <Text type="display-3" hasTabularNumbers>{value}</Text>
        {hint && <Text type="supporting" color="secondary">{hint}</Text>}
      </VStack>
    </Card>
  );
}

// Barras verticales de una sola serie; el valor sale al pasar el dedo o el foco, y hay tabla equivalente.
function BarChart({ title, points }: { title: string; points: { label: string; value: number }[] }) {
  const [hover, setHover] = useState<number | null>(null);
  // El SVG mide lo mismo que su caja: así las letras no se encogen en el celular ni crecen en escritorio.
  const box = useRef<HTMLElement>(null);
  const [W, setW] = useState(640);
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setW(Math.max(200, Math.round(e.contentRect.width))));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const max = Math.max(1, ...points.map((p) => p.value));
  const H = 200, pad = 24, gap = 2;
  // Barras delgadas aunque haya pocas (3 meses); el grupo queda centrado.
  const bw = Math.min(48, (W - gap * (points.length - 1)) / points.length);
  const x0 = (W - (bw * points.length + gap * (points.length - 1))) / 2;
  const every = Math.ceil(points.length / Math.max(1, Math.floor(W / 48))); // etiquetas del eje sin encimarse
  const cur = hover !== null ? points[hover] : null;
  return (
    <VStack gap={2}>
      <HStack gap={2} hAlign="between" vAlign="center">
        <Heading level={2}>{title}</Heading>
        <Text type="supporting" color="secondary" aria-live="polite">{cur ? `${cur.label}: ${pesos(cur.value)}` : `Máximo ${pesos(max)}`}</Text>
      </HStack>
      <figure ref={box} className="chart-box">
      <svg viewBox={`0 0 ${W} ${H + pad}`} className="chart" role="img" aria-label={`${title}. Detalle en la tabla de abajo.`} onMouseLeave={() => setHover(null)}>
        <line x1={0} x2={W} y1={H} y2={H} className="chart-axis" />
        {points.map((p, i) => {
          const h = (p.value / max) * (H - 8);
          const x = x0 + i * (bw + gap);
          return (
            <g key={i} onMouseEnter={() => setHover(i)}>
              <rect x={x} y={0} width={bw} height={H} className="chart-hit" />
              {p.value > 0 && <path className={"chart-bar" + (hover === i ? " on" : "")} d={roundTop(x, H - h, bw, h, Math.min(4, bw / 2))} />}
              {i % every === 0 && <text x={x + bw / 2} y={H + 16} textAnchor="middle" className="chart-label">{p.label}</text>}
            </g>
          );
        })}
      </svg>
      </figure>
      <details>
        <summary>Ver como tabla</summary>
        <Table data={points.map((p, i) => ({ ...p, i }))} idKey="i" density="compact"
          columns={[{ key: "label", header: "Periodo" }, { key: "value", header: "Ventas", align: "end", renderCell: (p) => <Text hasTabularNumbers>{pesos(p.value)}</Text> }]} />
      </details>
    </VStack>
  );
}

// Rectángulo con las esquinas de arriba redondeadas (el extremo del dato), pegado a la base.
function roundTop(x: number, y: number, w: number, h: number, r: number) {
  r = Math.min(r, h);
  return `M${x},${y + h}V${y + r}Q${x},${y} ${x + r},${y}H${x + w - r}Q${x + w},${y} ${x + w},${y + r}V${y + h}Z`;
}

// Barras horizontales: lista ordenada con la barra detrás del texto (se lee igual sin colores).
function Ranking({ title, rows, empty, money = true }: { title: string; rows: ItemRow[]; empty: string; money?: boolean }) {
  const max = Math.max(1, ...rows.map((r) => r.cents));
  return (
    <Card padding={5} className="rise">
      <VStack gap={3}>
        <Heading level={2}>{title}</Heading>
        {rows.length === 0 ? <Text color="secondary">{empty}</Text> : (
          <ol className="ranking">
            {rows.map((r, i) => (
              <li key={i} style={{ "--w": `${(r.cents / max) * 100}%` } as React.CSSProperties}>
                <span className="ranking-name">{r.name}{r.variant && <span className="ranking-sub"> · {r.variant}</span>}</span>
                <span className="ranking-val">{r.qty && `${r.qty} ${r.unit} · `}{money ? pesos(r.cents) : r.cents}</span>
              </li>
            ))}
          </ol>
        )}
      </VStack>
    </Card>
  );
}

function LowList({ rows }: { rows: ItemRow[] }) {
  return (
    <List>
      {rows.map((l, i) => (
        <ListItem key={i} label={`${l.name}${l.variant ? ` · ${l.variant}` : ""}`} href={`/panel/productos/${l.id}`}
          endContent={<Token size="sm" label={`${l.qty} ${l.unit}`} color={l.qty === "0" ? "red" : "orange"} />} />
      ))}
    </List>
  );
}

function SalesSummary({ summary: s, pending, ...b }: Base & { summary: Summary }) {
  const wide = useWide();
  const go = (v: string) => location.assign(`/panel/ventas/resumen?periodo=${v}`);
  const series = (s.series ?? []).map((p) => ({ label: p.label, value: p.sold }));
  const reasons = (s.reasons ?? []).map((r) => ({ name: r.label, variant: "", qty: "", unit: "", cents: r.cents, orders: r.orders }));
  return (
    <PanelShell {...b} pending={pending} title="Ventas" current="/panel/ventas" width={1040}
      actions={<Button href={`/panel/ventas/ventas.csv?periodo=${s.period}`} label="Descargar Excel (CSV)" icon={<Download aria-hidden size={16} />} />}>
      <SalesTabs current="resumen" pending={pending} />
      {wide ? (
        <SegmentedControl label="Periodo" value={s.period} onChange={go}>
          {s.periods.map((p) => <SegmentedControlItem key={p.value} value={p.value} label={p.label} />)}
        </SegmentedControl>
      ) : (
        <Selector label="Periodo" options={s.periods} value={s.period} onChange={(v) => v && go(v)} width="100%" />
      )}
      <Grid columns={{ minWidth: 136, repeat: "fit" }} gap={3}>
        <Kpi label="Vendido" value={pesos(s.revenue)} hint={`${s.sold} ${s.sold === 1 ? "pedido" : "pedidos"}`} />
        <Kpi label="Ticket promedio" value={pesos(s.ticket)} />
        <Kpi label="Pedidos que se cerraron" value={`${s.conversion} %`} hint={`${s.sold} de ${s.sold + s.lost}`} />
        <Kpi label="No vendido" value={pesos(s.lostCents)} hint={`${s.lost} ${s.lost === 1 ? "oportunidad" : "oportunidades"}`} />
        {s.pending > 0 && <Kpi label="Pendientes" value={String(s.pending)} hint="Sin marcar todavía" />}
      </Grid>
      <Card padding={5} className="rise">
        <BarChart title={s.period === "mes" || s.period === "mes-pasado" ? "Ventas por día" : "Ventas por mes"} points={series} />
      </Card>
      <Grid columns={{ minWidth: 300, repeat: "fit" }} gap={4}>
        <Ranking title="Lo más vendido" rows={s.top ?? []} empty="Aún no hay ventas en este periodo." />
        <Ranking title="Por categoría" rows={s.categories ?? []} empty="Aún no hay ventas en este periodo." />
        <Ranking title="Lo que no se vendió" rows={s.topLost ?? []} empty="Ninguna oportunidad perdida en este periodo." />
        <Card padding={5} className="rise">
          <VStack gap={3}>
            <Heading level={2}>Por qué no se vendió</Heading>
            {reasons.length === 0 ? <Text color="secondary">Nada que reportar.</Text> : (
              <List>
                {reasons.map((r) => <ListItem key={r.name} label={r.name} description={`${r.orders} ${r.orders === 1 ? "pedido" : "pedidos"}`} endContent={<Text hasTabularNumbers>{pesos(r.cents)}</Text>} />)}
              </List>
            )}
          </VStack>
        </Card>
      </Grid>
      <Card padding={5} className="rise">
        <VStack gap={3}>
          <Heading level={2}>Por surtir</Heading>
          <Text color="secondary">Lo que lleva inventario y está por acabarse.</Text>
          {(s.lowStock ?? []).length === 0 ? <Text color="secondary">Todo tiene existencia suficiente.</Text> : (
            <>
              <LowList rows={(s.lowStock ?? []).slice(0, 8)} />
              {(s.lowStock ?? []).length > 8 && (
                <details>
                  <summary>{`Ver ${(s.lowStock ?? []).length - 8} más`}</summary>
                  <LowList rows={(s.lowStock ?? []).slice(8)} />
                </details>
              )}
            </>
          )}
        </VStack>
      </Card>
    </PanelShell>
  );
}

mount({ sales: Sales, salesSummary: SalesSummary, messages: Messages, login: Login, forgot: Forgot, reset: Reset, resetDone: ResetDone, resetExpired: ResetExpired, products: Products, productEdit: ProductEdit, categories: Categories, myStore: MyStore }, neutralTheme);
