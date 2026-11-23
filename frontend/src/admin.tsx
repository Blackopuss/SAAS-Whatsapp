// Superadmin (admin.<dominio>): tiendas, alta, cobro y bitácora. Los datos llegan formateados desde views/admin.templ.
import { useState } from "react";
import { Store, Plus, ScrollText, Copy } from "lucide-react";
import { mount } from "./boot";
import { neutralTheme } from "@astryxdesign/theme-neutral/built";
import { Shell, AuthCard, type NavLink } from "./shell";
import { Txt, Check, Pick, Msg, PostButton, Form, useWide } from "./ui";
import { Button } from "@astryxdesign/core/Button";
import { Table } from "@astryxdesign/core/Table";
import { Link } from "@astryxdesign/core/Link";
import { Text } from "@astryxdesign/core/Text";
import { Heading } from "@astryxdesign/core/Heading";
import { StatusDot } from "@astryxdesign/core/StatusDot";
import { Token } from "@astryxdesign/core/Token";
import { HStack, VStack } from "@astryxdesign/core/Stack";
import { Card } from "@astryxdesign/core/Card";
import { Grid } from "@astryxdesign/core/Grid";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { List, ListItem } from "@astryxdesign/core/List";
import { MetadataList, MetadataListItem } from "@astryxdesign/core/MetadataList";
import { Dialog } from "@astryxdesign/core/Dialog";
import { Section } from "@astryxdesign/core/Section";
import { DateInput } from "@astryxdesign/core/DateInput";
import { TextInput } from "@astryxdesign/core/TextInput";
import { TabList, Tab } from "@astryxdesign/core/TabList";
import { Badge } from "@astryxdesign/core/Badge";
type ISODateString = Parameters<typeof DateInput>[0]["max"] & string;

const NAV: NavLink[] = [
  { href: "/", label: "Tiendas", icon: Store },
  { href: "/tiendas/nueva", label: "Nueva tienda", icon: Plus },
  { href: "/bitacora", label: "Bitácora", icon: ScrollText },
];

function AdminShell(p: { title: string; current: string; back?: { href: string; label: string }; actions?: React.ReactNode; children: React.ReactNode; width?: number }) {
  return <Shell app="Tiendas" sub="Superadmin" nav={NAV} logout="/logout" {...p} />;
}

function Login({ email, error }: { email: string; error: string }) {
  return (
    <AuthCard title="Superadmin" sub="Entra con tu contraseña y el código de tu app.">
      <Form action="/login">
        <Msg error={error} />
        <Txt name="email" label="Email" type="email" initial={email} autoComplete="username" hasAutoFocus={!email} />
        <Txt name="password" label="Contraseña" type="password" autoComplete="current-password" hasAutoFocus={!!email} />
        <Txt name="code" label="Código de tu app de autenticación" inputMode="numeric" autoComplete="one-time-code" />
        <Button type="submit" variant="primary" size="lg" label="Entrar" width="100%" />
      </Form>
    </AuthCard>
  );
}

type StoreRow = { id: number; name: string; slug: string; active: boolean; status: string; paidUntil: string; overdue: boolean; streak: number; founder: boolean; products: number; orders: number };

function status(s: { active: boolean; overdue: boolean; status: string }) {
  if (!s.active) return <StatusDot variant="error" label={s.status} tooltip={s.status} />;
  if (s.overdue) return <StatusDot variant="warning" label="Abierta, con pago vencido" tooltip="Pago vencido" />;
  return <StatusDot variant="success" label={s.status} tooltip={s.status} />;
}

const fold = (s: string) => s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();

function Stores({ stores }: { stores: StoreRow[] }) {
  const [q, setQ] = useState("");
  const [show, setShow] = useState("todas");
  const wide = useWide();
  const active = stores.filter((s) => s.active).length;
  const late = stores.filter((s) => s.active && s.overdue).length;
  const rows = stores.filter((s) =>
    (!q.trim() || fold(s.name + " " + s.slug).includes(fold(q.trim()))) &&
    (show === "todas" || (show === "abiertas" && s.active) || (show === "pausadas" && !s.active) || (show === "vencidas" && s.active && s.overdue)));
  return (
    <AdminShell title="Tiendas" current="/" width={1040} actions={<Button href="/tiendas/nueva" variant="primary" label="Nueva tienda" icon={<Plus aria-hidden size={16} />} />}>
      <Grid columns={{ minWidth: 160, repeat: "fit" }} gap={3}>
        <Stat label="Tiendas" value={stores.length} />
        <Stat label="Abiertas" value={active} />
        <Stat label="Con pago vencido" value={late} />
      </Grid>
      <TextInput label="Buscar tienda" placeholder="Nombre o subdominio" value={q} onChange={setQ} startIcon="search" />
      <TabList value={show} onChange={setShow} hasDivider role="tablist">
        <Tab value="todas" label="Todas" endContent={<Badge label={stores.length} />} />
        <Tab value="abiertas" label="Abiertas" endContent={<Badge label={active} />} />
        <Tab value="pausadas" label="Pausadas" endContent={<Badge label={stores.length - active} />} />
        <Tab value="vencidas" label="Pago vencido" endContent={<Badge label={late} />} />
      </TabList>
      <Card padding={0} className="rise">
        <Table
          data={rows}
          idKey="id"
          hasHover
          emptyState={<EmptyState title="Aún no hay tiendas" description="Da de alta la primera para empezar." actions={<Button href="/tiendas/nueva" variant="primary" label="Dar de alta" />} />}
          columns={!wide ? [
            { key: "name", header: "Tienda", renderCell: (s) => (
              <VStack gap={1}>
                <HStack gap={2} vAlign="center" wrap="wrap">
                  <Link href={`/tiendas/${s.id}`} weight="semibold">{s.name}</Link>
                  {s.founder && <Token label="Fundadora" color="purple" size="sm" />}
                </HStack>
                <HStack gap={2} vAlign="center">{status(s)}<Text type="supporting">{s.status}</Text></HStack>
                <Text type="supporting" color={s.overdue ? "accent" : "secondary"} hasTabularNumbers>
                  {`${s.paidUntil === "sin pagos" ? "Sin pagos" : `Pagado hasta ${s.paidUntil}`} · ${s.products} productos · ${s.orders} pedidos en 30 días`}
                </Text>
              </VStack>
            ) },
          ] : [
            { key: "name", header: "Tienda", renderCell: (s) => (
              <VStack gap={0.5}>
                <Link href={`/tiendas/${s.id}`} weight="semibold">{s.name}</Link>
                <Text type="supporting" color="secondary">{s.slug}</Text>
              </VStack>
            ) },
            { key: "status", header: "Estado", renderCell: (s) => <HStack gap={2} vAlign="center">{status(s)}<Text>{s.status}</Text></HStack> },
            { key: "paidUntil", header: "Pagado hasta", renderCell: (s) => <Text hasTabularNumbers color={s.overdue ? "accent" : "primary"}>{s.paidUntil}</Text> },
            { key: "streak", header: "Racha", align: "end", renderCell: (s) => <Text hasTabularNumbers>{s.streak > 0 ? `${s.streak} m` : "—"}</Text> },
            { key: "products", header: "Productos", align: "end", renderCell: (s) => <Text hasTabularNumbers>{s.products}</Text> },
            { key: "orders", header: "Pedidos 30 días", align: "end", renderCell: (s) => <Text hasTabularNumbers>{s.orders}</Text> },
            { key: "founder", header: "Plan", renderCell: (s) => (s.founder ? <Token label="Fundadora" color="purple" size="sm" /> : null) },
          ]}
        />
      </Card>
    </AdminShell>
  );
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <Card className="rise">
      <VStack gap={1}>
        <Text type="supporting" color="secondary">{label}</Text>
        <Text type="display-3" hasTabularNumbers>{String(value)}</Text>
      </VStack>
    </Card>
  );
}

type NewStore = { Slug: string; Name: string; Email: string; Whatsapp: string; Price: string; Error: string; Founder: boolean };

function StoreNew(f: NewStore) {
  return (
    <AdminShell title="Nueva tienda" current="/tiendas/nueva" back={{ href: "/", label: "Tiendas" }}>
      <Card padding={5} className="rise">
        <Form action="/tiendas" gap={5}>
          <Msg error={f.Error} />
          <Txt name="slug" label="Subdominio" initial={f.Slug} description="Minúsculas, números y guiones. Será la dirección de la tienda." autoCapitalize="none" hasSpellCheck={false} />
          <Txt name="name" label="Nombre de la tienda" initial={f.Name} />
          <Txt name="email" label="Email de la dueña" type="email" initial={f.Email} description="Con él entra a su panel." />
          <Txt name="whatsapp" label="WhatsApp de la dueña" isOptional type="tel" inputMode="tel" initial={f.Whatsapp} description="Para mandarle su enlace de acceso y recibir pedidos." />
          <Txt name="price" label="Precio mensual" inputMode="decimal" initial={f.Price} description="En pesos." />
          <Check name="founder" label="Es fundadora" initial={f.Founder} />
          <HStack><Button type="submit" variant="primary" size="lg" label="Dar de alta" /></HStack>
        </Form>
      </Card>
    </AdminShell>
  );
}

type Payment = { amount: string; months: number; paidOn: string; onTime: boolean; covers: string; note: string };
type StoreView = {
  id: number; name: string; shopURL: string; active: boolean; status: string; paidUntil: string; overdue: boolean; streak: number;
  price: string; priceInput: string; founder: boolean; notice: string; error: string; ok: string; today: string; payments: Payment[];
};

function StoreDetail(v: StoreView) {
  const [paying, setPaying] = useState(false);
  const [pausing, setPausing] = useState(false);
  const [tab, setTab] = useState("datos");
  const base = `/tiendas/${v.id}`;
  return (
    <AdminShell title={v.name} current="/" back={{ href: "/", label: "Tiendas" }}
      actions={<Button variant="primary" label="Registrar pago" onClick={() => setPaying(true)} />}>
      <Msg error={v.error} ok={v.ok} warn={v.notice ? `Lo que ve la dueña: ${v.notice}` : undefined} />
      <Card padding={5} className="rise">
        <VStack gap={4}>
          <HStack gap={2} vAlign="center">{status(v)}<Link href={v.shopURL} isExternalLink newTabLabel="(abre en otra pestaña)">{v.shopURL}</Link></HStack>
          <MetadataList columns="multi">
            <MetadataListItem label="Estado">{v.status}</MetadataListItem>
            <MetadataListItem label="Pagado hasta">{v.paidUntil || "Sin pagos"}</MetadataListItem>
            <MetadataListItem label="Racha">{`${v.streak} meses`}</MetadataListItem>
            <MetadataListItem label="Precio">{`${v.price} al mes${v.founder ? " · fundadora" : ""}`}</MetadataListItem>
          </MetadataList>
        </VStack>
      </Card>

      <TabList value={tab} onChange={setTab} hasDivider role="tablist">
        <Tab value="datos" label="Plan y acceso" panelId="t-datos" />
        <Tab value="pagos" label="Pagos" panelId="t-pagos" endContent={<Badge label={v.payments.length} />} />
      </TabList>

      <section hidden={tab !== "pagos"} id="t-pagos" role="tabpanel" aria-label="Pagos">
        <Card padding={0}>
          {v.payments.length === 0 ? (
            <EmptyState title="Aún no hay pagos" isCompact headingLevel={2} actions={<Button label="Registrar pago" onClick={() => setPaying(true)} />} />
          ) : (
            <List hasDividers>
              {v.payments.map((p, i) => (
                <ListItem key={i} label={`${p.amount} · ${p.months} mes(es)`}
                  description={`Pagado el ${p.paidOn} · cubre hasta ${p.covers}${p.note ? ` · ${p.note}` : ""}`}
                  endContent={<Token size="sm" label={p.onTime ? "A tiempo" : "Con atraso"} color={p.onTime ? "green" : "orange"} />} />
              ))}
            </List>
          )}
        </Card>
      </section>

      <section hidden={tab !== "datos"} id="t-datos" role="tabpanel" aria-label="Plan y acceso">
      <Grid columns={{ minWidth: 300 }} gap={4}>
        <Card padding={5} className="rise">
          <VStack gap={4}>
            <Heading level={2}>Plan</Heading>
            <Form action={`${base}/plan`}>
              <Txt name="price" label="Precio mensual" inputMode="decimal" initial={v.priceInput} description="En pesos." />
              <Check name="founder" label="Es fundadora" initial={v.founder} />
              <HStack><Button type="submit" label="Guardar plan" /></HStack>
            </Form>
          </VStack>
        </Card>
        <Card padding={5} className="rise">
          <VStack gap={4}>
            <Heading level={2}>Acceso y estado</Heading>
            <Text color="secondary">El enlace sirve una vez y vence en 30 minutos.</Text>
            <HStack gap={2} wrap="wrap">
              <PostButton action={`${base}/recuperar`} label="Enlace para nueva contraseña" />
              {v.active
                ? <Button variant="destructive" label="Pausar tienda" onClick={() => setPausing(true)} />
                : <PostButton action={`${base}/reactivar`} variant="primary" label="Reactivar tienda" />}
            </HStack>
          </VStack>
        </Card>
      </Grid>
      </section>

      <Dialog isOpen={paying} onOpenChange={setPaying} purpose="form" width={440} padding={5}>
        <VStack gap={4}>
          <Heading level={2}>Registrar pago</Heading>
          <Form action={`${base}/pagos`}>
            <Txt name="amount" label="Monto" inputMode="decimal" initial={v.priceInput} />
            <Pick name="months" label="Meses" initial="1" options={["1", "2", "3", "6", "12"].map((m) => ({ value: m, label: m }))} />
            <DateField name="paid_on" label="Fecha del pago" value={v.today} max={v.today} />
            <Txt name="note" label="Nota" isOptional description="Por ejemplo: transferencia BBVA, referencia 1234." />
            <HStack gap={2} hAlign="end">
              <Button label="Cancelar" variant="ghost" onClick={() => setPaying(false)} />
              <Button type="submit" variant="primary" label="Registrar pago" />
            </HStack>
          </Form>
        </VStack>
      </Dialog>

      <Dialog isOpen={pausing} onOpenChange={setPausing} purpose="form" width={440} padding={5}>
        <VStack gap={4}>
          <Heading level={2}>{`¿Pausar ${v.name}?`}</Heading>
          <Text>Las compradoras verán "Tienda en pausa". La dueña puede seguir entrando a su panel.</Text>
          <Form action={`${base}/pausar`}>
            <Txt name="reason" label="Motivo (queda en la bitácora)" isOptional />
            <HStack gap={2} hAlign="end">
              <Button label="Cancelar" variant="ghost" onClick={() => setPausing(false)} />
              <Button type="submit" variant="destructive" label={`Pausar ${v.name}`} />
            </HStack>
          </Form>
        </VStack>
      </Dialog>
    </AdminShell>
  );
}

// DateField: DateInput con calendario; el servidor recibe AAAA-MM-DD en un hidden.
function DateField({ name, label, value, max }: { name: string; label: string; value: string; max: string }) {
  const [v, setV] = useState<string | undefined>(value);
  return (
    <>
      <DateInput label={label} value={v as ISODateString} max={max as ISODateString} onChange={setV} />
      <input type="hidden" name={name} value={v ?? ""} />
    </>
  );
}

function ResetLink({ title, link, wa, storeID }: { title: string; link: string; wa: string; storeID: number }) {
  return (
    <AdminShell title={title} current="/" back={{ href: `/tiendas/${storeID}`, label: "Ir a la tienda" }}>
      <Card padding={5} className="rise">
        <VStack gap={4}>
          <Text>El enlace vence en 30 minutos y sirve una sola vez. Mándaselo solo a la dueña.</Text>
          <Txt name="link" label="Enlace" initial={link} isReadOnly />
          <HStack gap={2} wrap="wrap">
            <Button label="Copiar enlace" icon={<Copy aria-hidden size={16} />} onClick={() => navigator.clipboard?.writeText(link)} />
            {wa && <Button href={wa} variant="primary" label="Enviar por WhatsApp" />}
          </HStack>
        </VStack>
      </Card>
    </AdminShell>
  );
}

type AuditRow = { action: string; store: string; when: string; who: string; detail: string };

function Audit({ rows }: { rows: AuditRow[] }) {
  return (
    <AdminShell title="Bitácora" current="/bitacora">
      <Section padding={0} variant="transparent">
        <Card padding={0} className="rise">
          {rows.length === 0 ? (
            <EmptyState title="Aún no hay acciones registradas" isCompact headingLevel={2} />
          ) : (
            <List hasDividers>
              {rows.map((a, i) => (
                <ListItem key={i} label={a.store ? `${a.action} · ${a.store}` : a.action}
                  description={<Text type="supporting" color="secondary">{`${a.when} · ${a.who}${a.detail ? ` · ${a.detail}` : ""}`}</Text>} />
              ))}
            </List>
          )}
        </Card>
      </Section>
    </AdminShell>
  );
}

mount({ login: Login, stores: Stores, storeNew: StoreNew, store: StoreDetail, resetLink: ResetLink, audit: Audit }, neutralTheme);
