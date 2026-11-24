# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users
- **Compradora**: llega desde un enlace de WhatsApp, Facebook o un QR, casi siempre en celular con datos. Quiere ver qué hay, cuánto cuesta y pedir sin crear cuenta.
- **Vendedora (dueña de la tienda)**: pequeño negocio mexicano (ropa, carnicería, abarrotes, barbería, papelería, etc.). Hay de todo: unas apenas usan WhatsApp y Facebook, otras ya venden en redes y quieren rapidez. Opera desde el celular. El panel debe ser obvio para la novata sin estorbar a la experta.
- **Superadmin**: da de alta tiendas y registra pagos manuales.

## Product Purpose
Catálogo en línea por subdominio (`ana.<dominio>`) donde la compradora arma un carrito y lo manda como pedido por WhatsApp. La vendedora administra productos, existencias y ventas desde su panel. Éxito: la compradora confía y pide; la vendedora sabe qué hacer en su panel sin ayuda.

## Positioning
Tienda profesional sin pasarela ni comisiones: el cierre sigue siendo la conversación de WhatsApp que la vendedora ya domina, pero con catálogo, existencias y registro de pedidos.

## Operating Context
- Pedido = mensaje de WhatsApp con código de 6 letras; pago y entrega se acuerdan por chat.
- UI en español de México, móvil primero (360 px mínimo).
- Cada tienda tiene un giro (vitrina, boutique, barbería, carnicería, abarrotes, papelería) y un color de marca elegido por la vendedora.

## Capabilities and Constraints
- Venta por pieza o a granel (kg, g, litro, metro) con paso; existencias opcionales por producto o por talla × color.
- Sin cuentas de compradora, sin pagos en línea, sin reseñas.
- CSP estricta con nonce (sin estilos en línea fuera de `<style nonce>` o atributo `style` de React).
- Panel y superadmin usan componentes Astryx; la tienda es React con CSS propio.

## Brand Commitments
- Base visual premium y limpia, común a todos los giros; cada giro cambia acento (color, fuente de títulos, esquinas), no la estructura. Referencias de calidad citadas por el dueño: Apple, Shopify, Amazon (decisión del 2026-11-23).

## Evidence on Hand
- Solo datos reales: nada de descuentos, reseñas, existencias, urgencia ni testimonios inventados. Ofertas y "Nuevo" salen de lo que captura la vendedora.
- Fotos de demo en `demo/fotos/` (fuera de git).

## Product Principles
1. La compradora debe poder pedir en tres toques desde cualquier producto.
2. Lo único que grita es la acción principal; todo lo demás se calla.
3. La vendedora siempre ve qué sigue (siguiente paso, pedidos pendientes).
4. Nada de datos inventados para vender más.

## Accessibility & Inclusion
WCAG AA; axe sin hallazgos y Lighthouse accesibilidad 100 en la e2e. Objetivos táctiles de 44 px. Respeta `prefers-reduced-motion`.
