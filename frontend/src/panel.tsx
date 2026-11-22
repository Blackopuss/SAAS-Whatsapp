// Panel de la vendedora (/panel): productos, categorías, mi tienda y acceso. Datos desde views/panel.templ.
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Package, Tags, Store, ExternalLink, Plus, ArrowUp, ArrowDown, Trash2, Download, Upload as UploadIcon } from "lucide-react";
import { mount } from "./boot";
import { Shell, AuthCard, type NavLink } from "./shell";
import { Txt, Area, Check, Pick, Msg, PostButton, Form } from "./ui";
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
import { ToggleButton } from "@astryxdesign/core/ToggleButton";
import { Token } from "@astryxdesign/core/Token";
import { Dialog } from "@astryxdesign/core/Dialog";
import { FileInput } from "@astryxdesign/core/FileInput";
import { RadioList, RadioListItem } from "@astryxdesign/core/RadioList";
import { AspectRatio } from "@astryxdesign/core/AspectRatio";
import { TextInput } from "@astryxdesign/core/TextInput";

const NAV: NavLink[] = [
  { href: "/panel/", label: "Productos", icon: Package },
  { href: "/panel/categorias", label: "Categorías", icon: Tags },
  { href: "/panel/tienda", label: "Mi tienda", icon: Store },
  { href: "/", label: "Ver mi tienda", icon: ExternalLink },
];

type Base = { store: string; notice: string };

function PanelShell({ store, notice, ...p }: Base & { title: string; current: string; back?: { href: string; label: string }; actions?: ReactNode; width?: number; children: ReactNode }) {
  const banner = notice ? <Banner status="warning" title={notice} container="section" /> : undefined;
  return <Shell app={store} sub="Tu panel" nav={NAV} logout="/panel/logout" banner={banner} {...p} />;
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

type ProductRow = { id: number; name: string; price: string; was: string; active: boolean; soldOut: boolean; isNew: boolean; onSale: boolean };

// Flag: botón que alterna una marca del producto con POST (el servidor la invierte y regresa al listado).
function Flag({ id, flag, label, on }: { id: number; flag: string; label: string; on: boolean }) {
  return (
    <form method="post" action={`/panel/productos/${id}/marcar/${flag}`} className="inline-form">
      <ToggleButton size="sm" label={label} isPressed={on}
        onPressedChange={(_, e) => (e.currentTarget as HTMLElement).closest("form")?.requestSubmit()}>{on ? `✓ ${label}` : label}</ToggleButton>
    </form>
  );
}

function Products({ products, ...b }: Base & { products: ProductRow[] }) {
  const add = <Button href="/panel/productos/nuevo" variant="primary" label="Agregar producto" icon={<Plus aria-hidden size={16} />} />;
  return (
    <PanelShell {...b} title="Productos" current="/panel/" actions={products.length > 0 ? add : undefined}>
      {products.length === 0 ? (
        <Card className="rise">
          <EmptyState title="Aún no tienes productos" description="Agrega el primero; luego puedes ordenarlos por categoría." icon={<Package aria-hidden size={32} />} actions={add} />
        </Card>
      ) : (
        <VStack gap={3}>
          {products.map((p, i) => (
            <Card key={p.id} padding={4} className="rise" style={{ "--i": i } as React.CSSProperties}>
              <VStack gap={3}>
                <HStack gap={3} hAlign="between" vAlign="start">
                  <VStack gap={1}>
                    <Link href={`/panel/productos/${p.id}`} weight="semibold" color="primary" type="large">{p.name}</Link>
                    <HStack gap={2} vAlign="center" wrap="wrap">
                      <Text hasTabularNumbers weight="medium">{p.price}</Text>
                      {p.was && <Text hasTabularNumbers hasStrikethrough color="secondary">{p.was}</Text>}
                      {!p.active && <Token size="sm" label="Oculto" color="gray" />}
                      {p.soldOut && <Token size="sm" label="Agotado" color="red" />}
                    </HStack>
                  </VStack>
                  <Button href={`/panel/productos/${p.id}`} variant="ghost" size="sm" label="Editar" />
                </HStack>
                <HStack gap={2} wrap="wrap">
                  <Flag id={p.id} flag="active" label="Visible" on={p.active} />
                  <Flag id={p.id} flag="sold_out" label="Agotado" on={p.soldOut} />
                  <Flag id={p.id} flag="is_new" label="Nuevo" on={p.isNew} />
                  <Flag id={p.id} flag="on_sale" label="Oferta" on={p.onSale} />
                </HStack>
              </VStack>
            </Card>
          ))}
        </VStack>
      )}
    </PanelShell>
  );
}

type Opt = { value: string; label: string };
type Photo = { ID: number; Src: string; SrcSet: string };
type ProductForm = {
  ID: number; Name: string; Description: string; Price: string; SalePrice: string; Sizes: string; Colors: string;
  Variants: { Key: string; Label: string; SoldOut: boolean }[] | null; Error: string; Saved: boolean;
  Photos: Photo[] | null; PhotoOK: boolean; PhotoError: string;
};

function ProductEdit({ form: f, category, categories, ...b }: Base & { form: ProductForm; category: string; categories: Opt[] }) {
  const isNew = f.ID === 0;
  const [deleting, setDeleting] = useState(false);
  return (
    <PanelShell {...b} title={isNew ? "Nuevo producto" : "Editar producto"} current="/panel/" back={{ href: "/panel/", label: "Productos" }}>
      <Card padding={5} className="rise">
        <Form action={isNew ? "/panel/productos" : `/panel/productos/${f.ID}`} gap={5}>
          <Msg error={f.Error} ok={f.Saved ? "Producto guardado." : undefined} />
          <Txt name="name" label="Nombre" initial={f.Name} />
          <Grid columns={{ minWidth: 200, repeat: "fit" }} gap={4}>
            <Txt name="price" label="Precio" initial={f.Price} inputMode="decimal" placeholder="289" />
            <Txt name="sale_price" label="Oferta" isOptional initial={f.SalePrice} inputMode="decimal" placeholder="249" />
          </Grid>
          <Pick name="category_id" label="Categoría" initial={category} placeholder="Sin categoría" options={[{ value: "", label: "Sin categoría" }, ...categories]} />
          <Txt name="sizes" label="Tallas" initial={f.Sizes} placeholder="S, M, L" description="Sepáralas con comas. Déjalo vacío si es talla única." />
          <Txt name="colors" label="Colores" initial={f.Colors} placeholder="Negro, Blanco" description="Sepáralos con comas. Déjalo vacío si hay un solo color." />
          {f.Variants && f.Variants.length > 0 && (
            <Card variant="muted" padding={4}>
              <VStack gap={3}>
                <Text weight="semibold">¿Qué combinaciones están agotadas?</Text>
                <Grid columns={{ minWidth: 140 }} gap={2}>
                  {f.Variants.map((v) => <Check key={v.Key} name="soldout" value={v.Key} label={v.Label} initial={v.SoldOut} />)}
                </Grid>
              </VStack>
            </Card>
          )}
          <Area name="description" label="Descripción" initial={f.Description} rows={5} maxLength={4000} />
          <HStack><Button type="submit" variant="primary" size="lg" label="Guardar producto" /></HStack>
        </Form>
      </Card>

      {!isNew && <Photos f={f} />}

      {!isNew && (
        <Card padding={5} className="rise">
          <HStack gap={3} hAlign="between" vAlign="center" wrap="wrap">
            <VStack gap={1}>
              <Heading level={2}>Borrar producto</Heading>
              <Text color="secondary">Se borra para siempre, con sus fotos.</Text>
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

mount({ login: Login, forgot: Forgot, reset: Reset, resetDone: ResetDone, resetExpired: ResetExpired, products: Products, productEdit: ProductEdit, categories: Categories, myStore: MyStore });
