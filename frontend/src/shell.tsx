// Marco común del panel y del superadmin: navegación lateral (cajón en móvil), encabezado y columna de contenido.
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { ChevronLeft, LogOut } from "lucide-react";
import { AppShell } from "@astryxdesign/core/AppShell";
import { SideNav, SideNavHeading, SideNavItem, SideNavSection } from "@astryxdesign/core/SideNav";
import { Layout, LayoutContent } from "@astryxdesign/core/Layout";
import { HStack, VStack } from "@astryxdesign/core/Stack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Center } from "@astryxdesign/core/Center";
import { Card } from "@astryxdesign/core/Card";
import { Badge } from "@astryxdesign/core/Badge";
// No está en los exports del paquete; se importa por ruta.
import { slideRulesCss } from "../node_modules/@astryxdesign/core/dist/Layer/layerSlideRules.js";

// Las capas flotantes de Astryx meten estas reglas en un <style> sin nonce, que nuestra CSP bloquea.
// Las damos como hoja construida (la CSP no aplica) y el marcador con su id evita ese <style>.
if (!document.getElementById("astryx-layer-slide-rules")) {
  const sheet = new CSSStyleSheet();
  sheet.replaceSync(slideRulesCss());
  document.adoptedStyleSheets = [...document.adoptedStyleSheets, sheet];
  const mark = document.createElement("meta");
  mark.id = "astryx-layer-slide-rules";
  document.head.append(mark);
}

export type NavLink = { href: string; label: string; icon: LucideIcon; badge?: number };

export function Shell({ app, sub, nav, current, logout, banner, title, description, back, actions, width = 760, children }: {
  app: string;
  sub?: string;
  nav: NavLink[];
  current: string; // href de la sección activa
  logout: string;
  banner?: ReactNode;
  title: string;
  description?: string; // una línea bajo el título que dice para qué sirve la página
  back?: { href: string; label: string };
  actions?: ReactNode;
  width?: number;
  children: ReactNode;
}) {
  const side = (
    <SideNav
      header={<SideNavHeading heading={app} subheading={sub} headingHref={nav[0]?.href} />}
      footer={
        <form method="post" action={logout}>
          <Button type="submit" variant="ghost" label="Salir" icon={<LogOut aria-hidden size={16} />} width="100%" />
        </form>
      }
    >
      <SideNavSection title="Menú" isHeaderHidden>
        {nav.map((n) => (
          <SideNavItem key={n.href} label={n.label} href={n.href} icon={n.icon} isSelected={n.href === current}
            endContent={n.badge ? <Badge label={n.badge} variant="warning" /> : undefined} />
        ))}
      </SideNavSection>
    </SideNav>
  );
  return (
    <AppShell sideNav={side} banner={banner} height="auto" variant="wash">
      <Layout height="auto" contentWidth={width} padding={4}
        content={
          <LayoutContent isScrollable={false}>
            <VStack gap={6} paddingBlockEnd={10}>
              <VStack gap={2} className="rise">
                {back && <HStack><Button href={back.href} label={back.label} variant="ghost" size="sm" icon={<ChevronLeft aria-hidden size={16} />} /></HStack>}
                <HStack gap={3} vAlign="center" hAlign="between" wrap="wrap">
                  <Heading level={1}>{title}</Heading>
                  {actions}
                </HStack>
                {description && <Text color="secondary">{description}</Text>}
              </VStack>
              {children}
            </VStack>
          </LayoutContent>
        }
      />
    </AppShell>
  );
}

// AuthCard: pantallas sueltas (entrar, recuperar contraseña) centradas en una tarjeta.
export function AuthCard({ title, sub, children, footer }: { title: string; sub?: string; children: ReactNode; footer?: ReactNode }) {
  return (
    <Center minHeight="100dvh" padding={4}>
      <VStack as="main" gap={4} width="100%" maxWidth={400} className="rise">
        <VStack gap={1}>
          <Heading level={1}>{title}</Heading>
          {sub && <Text color="secondary">{sub}</Text>}
        </VStack>
        <Card padding={5} elevation="low">{children}</Card>
        {footer}
      </VStack>
    </Center>
  );
}
