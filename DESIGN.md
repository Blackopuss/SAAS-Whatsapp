# Design

Sistema visual de la tienda pública y del panel (2026-11-23). Producto y principios en `PRODUCT.md`.

## Tienda (`frontend/src/shop.css`)

Marco común (encabezado, carrito, ficha) y portada propia por giro; referencias de calidad: Apple (aire, fotos grandes), Shopify (tarjeta y ficha), Amazon (claridad de compra).

**Color (Restrained).** Fondo `--bg` blanco (`#121212` en oscuro), `--surface` gris piedra `#f4f4f2` para fotos y bloques suaves, `--line` `#e6e5e1` para separadores, `--ink` `#161616` y `--ink-2` `#5b5a55` (6.5:1). El color de la marca (`--color-accent`, lo pone Go por tienda) solo en: botón principal, contador del carrito, `−N %` y selección. Precio en oferta en tinta, no en color (contraste seguro con cualquier marca). WhatsApp: `--wa` `#0b7a3e`.

**Tipo.** Texto y títulos de sección en la letra del sistema (`--sans`: SF / Segoe / Roboto), peso 650, tracking −0.015 a −0.03 em. La letra del giro (`--display` = `--font-family-heading` del tema) solo en el nombre de la tienda, título de portada y de categoría. Escala: portada `clamp(2.5rem, 11vw, 5.25rem)`, página `clamp(2.25rem, 9vw, 3.75rem)`, sección `clamp(1.375rem, 4.5vw, 1.875rem)`, producto `clamp(1.625rem, 6vw, 2.375rem)`, cuerpo 1rem, tarjeta 0.9375rem, meta 0.8125rem. Números tabulares en precios.

**Forma.** Esquinas `--r` (tarjeta/foto) y `--r-sm` (controles) del tema; botones píldora. Sin sombras en tarjetas; sombra solo en lo que flota (+ de agregar rápido, cajón, hoja, barra de compra). Separadores de 1 px en vez de cajas.

**Componentes.** Encabezado blanco fijo con pestañas de categorías subrayadas. Tarjeta: foto 4:5, nombre (2 renglones), precio y precio tachado, meta ("Nuevo" · tallas); sobre la foto solo "Agotado" o "−N %"; botón + circular en la esquina inferior de la foto (posición con `125cqi`). Rieles deslizables con asomo; rejilla 2/3/4 columnas. Categorías con foto cuadrada. Franja "así compras" de 3 columnas con separadores. Opciones talla/color: recuadro con borde, elegido = borde doble de tinta, agotado = tachado sobre `--surface`. Barra de compra fija en el celular.

**Movimiento.** Un momento: el título de la portada sube palabra por palabra; fotos aparecen con opacidad al cargar; hover con zoom de 3 % en escritorio. `prefers-reduced-motion` lo apaga todo.

**Giros (`frontend/src/giros/`, portadas en `shop.tsx`).** Cada giro tiene composición propia; paleta y letras del tema (editor de Astryx):

| Giro | Concepto | Paleta | Letras | Portada | Catálogo y tarjeta | Ficha |
|---|---|---|---|---|---|---|
| Vitrina | Almacén general claro | porcelana, carbón, coral | Bricolage + sistema | Buscador grande + colección destacada (1 grande, 2 chicas) | Categorías redondas, bandas porcelana/blanco, rejilla 2/3/4 | Base |
| Boutique | Revista de moda | marfil, espresso, vino, blush | Cormorant + Bricolage | Foto de campaña dominante asimétrica, titular serif, segunda foto desplazada | Colecciones en filas imagen/texto alternas; fotos 3:4 sin marco con tallas y muestras | Miniaturas en columna, selectores amplios |
| Barbería | Señalética | hueso, carbón, cobre | Big Shoulders + Bricolage | Banda carbón con titular condensado y producto en placa clara; franja cobre | Familias numeradas tipo letrero; ofertas en banda carbón; fotos 1:1 con marco | Tabla de datos (familia, se vende por, presentación) |
| Carnicería | Mostrador | papel de carnicería, carbón, rojo carne, verde hierba | Alfa Slab + Bricolage | Pizarra "Precios de hoy" (primero lo que va por kilo) + producto destacado | Catálogo en banda blanca por corte; tarjetas en renglón con "Por kg"/"Por pieza" | Cantidad × precio = total; tabla de datos |
| Abarrotes | Pasillos | crema, verde hoja, tomate | Lilita + Bricolage | Banda verde con buscador grande y accesos a pasillos | Un riel por pasillo; tarjeta compacta con precio primero y "Agregar" ancho; ofertas en banda tomate | Stepper grande |
| Papelería | Cuaderno y carpetas | marfil, marino, azul escolar, amarillo, coral | Fredoka + Bricolage | Hoja cuadriculada con etiquetas de categoría y mosaico de fotos con cinta | Carpetas con pestaña de color por categoría; tarjeta redondeada con muestras | Chips con muestra, descripción en renglones |

## Panel y superadmin (Astryx, tema `panel`)

Tema hecho con el editor de Astryx sobre neutral: acento `#0f7a4a`, neutro cálido `#78716c`, esquinas `large` (12 px), botones píldora. Fuente en `frontend/themes/panel/`; se cambia con `npx astryx experimental theme editor set panel …`. Cada página: título, una línea de para qué sirve (`description`) y acción principal arriba a la derecha. Inicio con 3 cifras enlazadas, lista de pasos con `ProgressBar` y tarjeta de compartir con QR.
