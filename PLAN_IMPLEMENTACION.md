# Plan de implementación - SaaS tiendas WhatsApp

Nov 21, 2026 · @Axel

Este documento baja a tareas concretas el "Plan de desarrollo". Donde aquel dice qué construir, este dice en qué orden, con qué criterios de aceptación, cómo se ven las plantillas y qué revisar de seguridad en cada paso.

Lineamientos que se aplicaron al armarlo:

- **Diseño (frontend-design / impeccable / ui-ux):** cada plantilla con identidad propia, tipografía elegida a propósito, color derivado de un solo valor de la vendedora y piso de calidad fijo (contraste, foco visible, movimiento reducido, móvil primero).
- **Web moderna (modern-web-guidance):** usar lo que ya trae el navegador en 2026 en vez de JavaScript, para no salirse de los 60 KB.
- **Seguridad (OWASP Top 10:2025):** cada sprint tiene su checklist y hay una auditoría completa antes de producción.

---

## 1. Antes de escribir código (día 0)

1. Crear el repo `rulasheim/tiendas` privado, rama `main` protegida.
2. Instalar herramientas locales:
   - Go 1.23+, `templ`, `sqlc`, `goose`, `air` (recarga en caliente)
   - Tailwind CLI standalone v4 (binario, sin Node)
   - Docker Desktop para Postgres 16
   - `golangci-lint`, `govulncheck`, `gosec`
3. Registrar un dominio de pruebas barato o usar un subdominio tuyo con Cloudflare Tunnel.
4. Crear bucket de R2 `tiendas-dev` y uno separado `tiendas-backups`, con llaves de API de alcance mínimo (una de solo escritura para backups).
5. Definir el nombre comercial, aunque sea provisional, porque aparece en el panel y en el footer de cada tienda.

**Listo cuando:** `make dev` levanta Postgres, compila templ y Tailwind y sirve un "hola" en `localhost:8080`.

---

## 2. Sprints

Sprints de 1 semana a medio tiempo. Cada uno termina con algo que se puede enseñar.

### Sprint 0 · Esqueleto (semana 1)

1. Estructura de carpetas del plan de desarrollo y `Makefile` con `dev`, `build`, `gen`, `test`, `lint`, `audit`.
2. Config con variables de entorno validadas al arrancar (si falta una, el binario no inicia).
3. Logs con `slog` en JSON, con `request_id` y `store_id` en cada línea.
4. Migración 001: `stores`, `users`. Migración 002: `categories`, `products`, `product_variants`, `product_images`.
5. Middlewares base en este orden: recover, request_id, logger, security headers, tenant, sesión, CSRF.
6. Pipeline de CI en GitHub Actions: `templ generate` + `sqlc generate` sin diff, tests, `golangci-lint`, `govulncheck`, `gosec`.

**Seguridad del sprint:**
- [ ] Headers fijos: `Content-Security-Policy` con nonce, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy` cerrado, `frame-ancestors 'none'` en panel y admin.
- [ ] Secrets solo por entorno, `.env` en `.gitignore`, `gitleaks` en CI.
- [ ] Usuario de Postgres de la app sin permisos de `CREATE`/`DROP`; las migraciones corren con otro usuario.

**Listo cuando:** CI en verde y `curl -I` muestra todos los headers.

### Sprint 1 · Auth y tenant (semana 2)

1. Login de vendedora con email y contraseña, hash `argon2id`.
2. Sesiones con `scs` en Postgres: cookie `HttpOnly`, `Secure`, `SameSite=Lax`, rotación de ID al iniciar sesión, expiración por inactividad de 7 días.
3. CSRF en todos los `POST`/`PUT`/`DELETE` del panel. htmx manda el token en un header con `hx-headers` desde el `<body>`.
4. Rate limit en login: 5 intentos por 15 min por IP y por email.
5. Middleware de tenant: `Host` → `slug` → tienda en contexto. Caché en memoria de 60 s.
6. Recuperar contraseña por WhatsApp o email con token de un solo uso y 30 min de vida.

**Seguridad del sprint (A01 Broken Access Control, A07 Auth):**
- [ ] Todas las queries de sqlc llevan `store_id` como parámetro obligatorio, sacado del contexto, nunca del formulario.
- [ ] Test de aislamiento: vendedora A intenta leer, editar y borrar un producto de B por ID y recibe 404 (no 403, para no confirmar que existe).
- [ ] Mensaje de error de login idéntico para "no existe" y "contraseña mal".
- [ ] El superadmin vive en otro subdominio con su propia cookie y exige 2FA TOTP.

**Listo cuando:** pasan los tests de aislamiento y no se puede entrar a `ana.` con la sesión de `luz.`.

### Sprint 2 · Catálogo desde el panel (semana 3)

1. CRUD de categorías con orden por arrastre (`hx-post` al soltar).
2. CRUD de productos con variantes de talla y color en la misma pantalla.
3. Subida de fotos: el navegador comprime antes de subir (canvas a WebP a 1600 px) para ahorrar datos de la vendedora, el servidor vuelve a validar y re-codificar.
4. Job de River: genera 400, 800 y 1200 px en WebP y AVIF, sube a R2, guarda llaves y un color dominante para el placeholder.
5. Toggle de activo, agotado, nuevo y oferta desde la lista sin entrar al producto.

**Seguridad del sprint (A03 Injection, A05 Misconfiguration, subidas):**
- [ ] Validar tipo real por magic bytes, máximo 10 MB, máximo 10 fotos por producto.
- [ ] Re-codificar siempre: elimina EXIF con GPS y cualquier payload.
- [ ] Llaves de R2 con nombre aleatorio (`stores/{id}/{uuid}.webp`), nunca el nombre original del archivo.
- [ ] templ escapa por default; prohibido `templ.Raw` salvo en el JSON-LD, que se genera con `json.Marshal`.
- [ ] Descripción de producto en texto plano con saltos de línea, sin HTML ni markdown.

**Listo cuando:** se da de alta un producto con 3 fotos y 6 variantes desde un celular en menos de 2 minutos.

### Sprint 3 · Plantilla 1 y carrito (semanas 4 y 5)

1. Sistema de tokens de tema (sección 3) y la plantilla **Vitrina**.
2. Páginas: inicio, categoría, producto, carrito, preguntas frecuentes.
3. Carrito con Alpine en `localStorage`, contador en el header, cajón con `<dialog>`.
4. Mensaje de WhatsApp armado con nombre, talla, color, cantidad, subtotal, total y link al producto.
5. Tienda demo con 20 productos reales, medida con Lighthouse móvil.

**Seguridad del sprint:**
- [ ] El precio del mensaje sale de los datos que pintó el servidor; el texto final se codifica con `encodeURIComponent`.
- [ ] El número de WhatsApp se valida en formato E.164 al guardarlo.
- [ ] `localStorage` envuelto en `try/catch` y validado al leer; si el producto ya no existe o cambió de precio, el carrito lo avisa.

**Listo cuando:** Lighthouse móvil arriba de 90 en rendimiento y accesibilidad, y el pedido llega bien formateado a WhatsApp.

### Sprint 4 · Personalización (semana 6)

1. Pantalla "Mi tienda": logo, banner, color principal, plantilla.
2. Vista previa en vivo dentro de un `<iframe>` que recarga por htmx con los valores sin guardar.
3. Validación de contraste en el servidor (sección 3.3) con aviso amable si el color no se lee.
4. Preguntas frecuentes y datos de transferencia.
5. QR de la tienda y link por producto con imagen Open Graph para que se vea bonito en estados.

### Sprint 5 · Fundadoras y cobro (semana 7)

1. Superadmin: alta de tiendas, registro manual de pagos, suspensión.
2. Job diario de River: racha de lealtad, aviso 3 días antes de vencer, suspensión al día 5 de atraso.
3. Página de "tienda en pausa" que no revela datos de la vendedora.
4. Bitácora de acciones del admin (quién, qué, cuándo).

### Sprint 6 · Producción (semana 8)

1. VPS con Ubuntu LTS, usuario sin root, SSH solo con llave, `ufw` con 80/443, `unattended-upgrades`, `fail2ban`.
2. Caddy con wildcard por DNS-01 de Cloudflare. Origen cerrado a IPs de Cloudflare.
3. systemd con `ProtectSystem=strict`, `NoNewPrivileges=yes`, `PrivateTmp=yes`.
4. Backups `pg_dump` cifrados con `age` a R2, retención 30 días, prueba de restauración documentada.
5. Uptime Kuma y alerta a tu WhatsApp o Telegram.
6. **Auditoría de seguridad completa** (sección 5) antes de abrir la primera tienda real.

---

## 3. Diseño de plantillas

### 3.1 Para quién diseñamos

Compradoras que llegan desde un estado o un grupo de WhatsApp, con un Android de gama baja, datos limitados y poca paciencia. Abren un link, ven fotos, eligen talla y mandan mensaje. Todo lo que no ayude a eso estorba.

Por eso:

- La foto del producto es la protagonista; la interfaz se hace a un lado.
- El color de la vendedora es su marca, pero el sistema garantiza que se lea bien, elija lo que elija.
- Cada plantilla tiene personalidad propia por tipografía y composición, no por adornos.
- Nada de carruseles automáticos, popups de bienvenida, ni animaciones al hacer scroll.

### 3.2 Tokens compartidos

Todas las plantillas leen las mismas variables. La vendedora solo escoge `--brand`; el resto se deriva con `oklch` y `color-mix`, así cualquier color queda armonioso.

```css
@layer tokens {
  :root {
    --brand: oklch(0.62 0.19 350);          /* lo elige la vendedora */
    --brand-ink: white;                      /* lo decide el servidor por contraste */
    --brand-soft: color-mix(in oklch, var(--brand) 12%, var(--paper));
    --brand-strong: oklch(from var(--brand) calc(l - 0.12) c h);

    --paper: oklch(0.985 0.004 90);
    --ink: oklch(0.22 0.01 280);
    --ink-muted: oklch(0.48 0.01 280);
    --line: oklch(0.90 0.005 280);
    --sale: oklch(0.58 0.2 25);
    --ok: oklch(0.6 0.13 155);

    --space-1: 0.25rem; --space-2: 0.5rem; --space-3: 0.75rem;
    --space-4: 1rem;    --space-6: 1.5rem; --space-8: 2rem;

    --step--1: clamp(0.83rem, 0.8rem + 0.15vw, 0.9rem);
    --step-0:  clamp(1rem, 0.96rem + 0.2vw, 1.06rem);
    --step-1:  clamp(1.2rem, 1.1rem + 0.5vw, 1.4rem);
    --step-2:  clamp(1.44rem, 1.25rem + 0.9vw, 1.9rem);
    --step-3:  clamp(1.73rem, 1.4rem + 1.6vw, 2.6rem);
  }
}
```

Escala tipográfica 1.2 (tercera menor) siguiendo Bringhurst; tamaño base nunca debajo de 16 px para que el celular no haga zoom en los inputs.

### 3.3 Contraste garantizado

1. Al guardar el color, el servidor calcula el contraste APCA/WCAG del `--brand` contra blanco y contra `--ink`.
2. Elige automáticamente `--brand-ink` (blanco o tinta) para el texto encima de botones.
3. Si ninguno pasa 4.5:1, oscurece o aclara el color en `oklch` hasta pasar y le muestra a la vendedora: "Ajustamos un poco tu color para que los botones se lean bien en cualquier celular." con el antes y después.
4. Se guarda el color original y el ajustado, para que pueda regresar si quiere.

### 3.4 Las tres plantillas

Cada una se distingue por la forma de mostrar el catálogo y por su tipografía. Fuentes self-hosted en WOFF2 con subset latino básico más acentos y ñ, una sola familia variable por plantilla para no pasar de 25 KB.

#### Vitrina (MVP)

Para la vendedora con buenas fotos. Cuadrícula de 2 columnas en celular, 3 en tablet, fotos a sangre en proporción 4:5 como un perfil de Instagram, pero con precio y talla disponibles a la vista.

- **Tipografía:** Bricolage Grotesque variable. Nombre de la tienda en peso 800 y ancho condensado, grande, como letrero de tienda de barrio. Productos en peso 500.
- **Color por defecto:** rosa mexicano `oklch(0.62 0.19 350)` sobre papel casi blanco.
- **Lo memorable:** el nombre de la tienda ocupa todo el ancho del header y se encoge con `animation-timeline: scroll()` al bajar, dejando una barra compacta. Es el único movimiento de la plantilla y se desactiva con `prefers-reduced-motion`.
- **Etiquetas:** "Nuevo" y "Oferta" como pestaña recortada en la esquina de la foto, no como píldora flotante. "Agotado" baja la saturación de la foto con `filter: grayscale(.8)`.

```
+--------------------------------+
| ANA MODA                       |  nombre a todo lo ancho
| Blusas  Vestidos  Ofertas  ... |  categorías con scroll horizontal
+---------------+----------------+
| [foto 4:5]    | [foto 4:5]     |
| Blusa lino    | Vestido midi   |
| $289  S M L   | $450  M L      |
+---------------+----------------+
| [foto 4:5]    | [foto 4:5]     |
                    ( Carrito 2 )   botón fijo abajo a la derecha
```

#### Lista (fase 4)

Para catálogos grandes o fotos regulares: tianguis, mayoreo, ropa por paca. Fila compacta con miniatura cuadrada, nombre, precio y botón de agregar sin abrir el producto.

- **Tipografía:** Familjen Grotesk, más estrecha, deja caber nombres largos en una línea.
- **Color por defecto:** verde botella `oklch(0.45 0.09 160)`.
- **Lo memorable:** el precio alineado a la derecha en cifras tabulares (`font-variant-numeric: tabular-nums`) y grande, para comparar rápido de un vistazo como en una lista de precios de mercado.
- Encabezados de categoría pegajosos con `position: sticky` para saber dónde vas.

```
+--------------------------------+
| Paca Luz        [buscar]       |
+--------------------------------+
| Pantalones ------------------- |  sticky
| [img] Mezclilla recto     $180 |
|       28 30 32          [ + ]  |
| [img] Cargo beige         $220 |
|       30 32             [ + ]  |
| Blusas ----------------------- |
```

#### Boutique (fase 4)

Para quien vende piezas más caras o de diseño propio. Banner grande arriba, colecciones como bloques editoriales y producto destacado alternando foto y texto.

- **Tipografía:** Fraunces variable con eje `SOFT` alto para títulos, del mismo archivo en peso ligero para cuerpo; un poco más de interlineado (1.6) por ser serif.
- **Color por defecto:** ciruela `oklch(0.38 0.08 330)` sobre un blanco frío, no crema.
- **Lo memorable:** el banner usa la foto de la vendedora con el nombre de la tienda sobrepuesto con `mix-blend-mode: difference`, así siempre se lee sin importar la foto.
- Más aire: márgenes de `--space-8`, una sola columna de lectura de 34 caracteres en las descripciones.

### 3.5 Componentes base (`themes/base/`)

Compartidos por las 3 plantillas, cambian solo por tokens:

| Componente | Detalle que lo hace sentir bien |
| --- | --- |
| Tarjeta de producto | Placeholder del color dominante de la foto mientras carga, sin saltos de layout gracias a `aspect-ratio` |
| Selector de talla | Botones de radio grandes (48 px), tallas agotadas tachadas pero visibles |
| Selector de color | Círculos con el color real y el nombre debajo, no solo el círculo |
| Galería del producto | `scroll-snap` horizontal con puntos indicadores, sin librería |
| Cajón del carrito | `<dialog>` nativo que sube desde abajo, se cierra con Escape, con gesto atrás del sistema y tocando afuera |
| Botón de WhatsApp | Siempre el mismo texto: "Enviar pedido por WhatsApp", verde de WhatsApp solo en este botón |
| Estado vacío | "Tu carrito está vacío. Ve las novedades" con link a la categoría con más productos nuevos |
| Errores | "No pudimos cargar las fotos. Revisa tu conexión y vuelve a intentar." con botón de reintentar |

### 3.6 Panel de la vendedora

Lo usa en el celular entre una venta y otra, así que manda la rapidez sobre lo bonito, sin dejar de ser agradable.

1. Navegación inferior fija con 4 destinos: Productos, Pedidos (fase 4), Mi tienda, Cuenta.
2. Botón grande "Agregar producto" siempre visible en la lista de productos.
3. Formularios de una columna, labels arriba del campo, teclado numérico en precio (`inputmode="decimal"`) y en WhatsApp (`inputmode="tel"`).
4. Guardado con confirmación que usa el mismo verbo: el botón dice "Guardar producto" y el aviso dice "Producto guardado".
5. Primera vez: lista vacía con tres pasos claros ("Sube tu logo", "Agrega tu primer producto", "Comparte tu tienda") que se van palomeando.
6. Tipografía del panel: la del sistema (`system-ui`) para que cargue al instante; el color de la marca del SaaS solo en acciones principales.

### 3.7 Piso de calidad (todas las pantallas)

- [ ] Contraste mínimo 4.5:1 en texto, 3:1 en íconos y bordes de inputs.
- [ ] Foco visible con `:focus-visible` y anillo de 2 px en `--brand-strong`.
- [ ] Áreas táctiles de al menos 44 × 44 px.
- [ ] `prefers-reduced-motion` apaga el header animado y las transiciones de vista.
- [ ] Funciona sin JavaScript para ver productos; el carrito es lo único que lo necesita.
- [ ] Probado en 360 px de ancho y con fuente del sistema al 130 %.
- [ ] `lang="es-MX"`, precios con `Intl.NumberFormat('es-MX', { style: 'currency', currency: 'MXN' })`.

---

## 4. Web moderna que aprovechamos

Todo esto ya tiene soporte amplio en Chrome Android, que es casi todo tu público, y degrada bien en lo demás.

| Necesidad | Solución nativa | Ahorro |
| --- | --- | --- |
| Cajón del carrito y modales | `<dialog>` y `popover` | Sin librería de modales |
| Transición entre lista y producto | View Transitions entre documentos (`@view-transition { navigation: auto; }`) | Se siente como app sin SPA |
| Abrir el producto más rápido | Speculation Rules con `prerender` en `moderate` solo para links de producto | Navegación casi instantánea |
| Imagen principal | `fetchpriority="high"` en la primera foto, `loading="lazy"` en el resto | Mejor LCP |
| Formatos | `<picture>` con AVIF, WebP de respaldo y `srcset` por ancho | 30 a 50 % menos peso |
| Layout que se adapta al contenedor | Container queries en la tarjeta de producto | Una sola tarjeta para las 3 plantillas |
| Estilos por estado | `:has()` (ej. resaltar talla seleccionada, deshabilitar botón sin talla) | Menos Alpine |
| Orden de CSS | `@layer reset, tokens, base, theme, utilities` | Sin peleas de especificidad |
| Colores derivados | `oklch`, `color-mix`, colores relativos | Paletas armoniosas automáticas |
| Header que se encoge | `animation-timeline: scroll()` | Sin listener de scroll |

**Reglas que salen de las guías de modern-web-guidance:**

1. **Carrito en `<dialog>`:** abrir con `showModal()` y `closedby="any"` para cerrar tocando afuera. No agregar `role`, `aria-modal`, trampa de foco ni `overflow: hidden` en el `body`, el navegador ya lo hace. Donde no exista `closedby`, igual cierra con Escape y el gesto atrás.
2. **View Transitions:** las dos páginas (lista y producto) deben declarar `@view-transition`, y siempre dentro de `@media (prefers-reduced-motion: no-preference)`. Si el navegador no lo soporta, la navegación es normal.
3. **Speculation Rules:** solo links de producto, con `eagerness: "moderate"`. Excluir explícitamente links de WhatsApp, `/logout` y cualquier URL que cambie estado. El navegador limita a 2 especulaciones no-eager, así que no se especula toda la página.
4. **Header que se encoge:** es decorativo, así que va como mejora progresiva dentro de `@supports ((animation-timeline: scroll()) and (animation-range: 0% 100%))`, sin polyfill ni listener de respaldo. Declarar `animation-timeline` después del shorthand `animation`. Poner `scroll-padding-top` igual a la altura del header pegajoso para que el foco no quede tapado.
5. **Fotos:** la primera foto visible lleva `fetchpriority="high"` y nunca `loading="lazy"`. Las fotos ocultas de la galería del producto llevan `fetchpriority="low"` y `loading="lazy"`. Las de más abajo solo `loading="lazy"`.

**Presupuesto de peso por página de tienda (sin fotos):**

| Recurso | Máximo |
| --- | --- |
| HTML | 15 KB gzip |
| CSS del tema | 12 KB gzip |
| Fuente WOFF2 | 25 KB |
| htmx + Alpine | 0 KB en la tienda pública si no hay carrito abierto; Alpine 15 KB diferido |
| **Total** | **menos de 60 KB** |

Este presupuesto se revisa en CI con un script que hace `curl` de la tienda demo y falla si se pasa.

---

## 5. Auditoría de seguridad (OWASP Top 10:2025)

Se corre al cierre del sprint 3 (primera versión pública de la demo) y otra vez antes de producción. Los hallazgos van en `SECURITY.md` con severidad, evidencia y estado.

| # | Riesgo | Qué revisar en este proyecto | Cómo se verifica |
| --- | --- | --- | --- |
| A01 | Control de acceso roto | Aislamiento por `store_id`, IDOR en productos, fotos y pagos; superadmin separado | Tests de integración cruzando tiendas, revisión de cada query de sqlc |
| A02 | Configuración insegura | Headers, CSP con nonce, CORS cerrado, errores sin stack trace en producción, puertos del VPS | `curl -I`, Mozilla Observatory, `nmap` desde fuera |
| A03 | Cadena de suministro | Dependencias de Go, versiones fijas de htmx y Alpine servidas desde tu dominio con SRI | `govulncheck`, Dependabot, hash SRI en el HTML |
| A04 | Fallas criptográficas | argon2id, cookies `Secure`, TLS 1.2+, backups cifrados, secretos fuera del repo | Revisión de config, `testssl.sh` |
| A05 | Inyección | SQL solo por sqlc parametrizado, escape de templ, link `wa.me` codificado, nombres de archivo | `gosec`, búsqueda de `templ.Raw` y `fmt.Sprintf` con SQL |
| A06 | Diseño inseguro | Reglas de negocio: precio y stock vienen del servidor, límites de productos y fotos por plan | Revisión del flujo de carrito y subida |
| A07 | Fallas de autenticación | Rate limit, rotación de sesión, recuperación de contraseña, 2FA del admin | Pruebas manuales y tests de fuerza bruta |
| A08 | Integridad de software y datos | Deploy firmado o con checksum, migraciones revisadas, jobs de River idempotentes | Script de deploy verifica SHA256 |
| A09 | Fallas de logging y alertas | Logs de login fallido, cambios de admin, suspensiones; sin contraseñas ni tokens en logs | Revisar una muestra de logs, alerta de 10 logins fallidos |
| A10 | Manejo de condiciones excepcionales | Panics recuperados, timeouts en HTTP y DB, límites de tamaño de body, fallo de R2 no tumba la tienda | Tests con R2 caído y Postgres lento |

**Extras propios del modelo multi-tenant:**

1. Subdominios reservados (`www`, `panel`, `admin`, `api`, `mail`, `static`) que no se pueden usar como slug.
2. Slugs solo `a-z0-9-`, de 3 a 30 caracteres, sin homoglifos.
3. Purga de caché de Cloudflare por tienda con token de alcance mínimo.
4. Cuando se active el dominio propio, `on_demand_tls` de Caddy solo pregunta a un endpoint interno que confirma que el dominio está registrado y verificado, para que nadie te haga emitir certificados sin control.

---

## 6. Definición de terminado (para cada tarea)

1. Código con tests, incluido el de aislamiento si toca datos de tienda.
2. CI en verde: lint, `govulncheck`, `gosec`, tests, presupuesto de peso.
3. Probado en un Android real o en Chrome con throttling "Slow 4G" y CPU 4x.
4. Revisado contra el piso de calidad de la sección 3.7.
5. Checklist de seguridad del sprint palomeado.

---

## 7. Skills para usar mientras programas

Ya quedaron instaladas en `.claude/skills/` del repo `impeccable` y `ui-ux-pro-max`. En Claude Code úsalas así:

1. **Al maquetar cada plantilla:** `ui-ux-pro-max` para paleta y patrones, luego `impeccable` para pulir y quitar lo genérico.
2. **Antes de cerrar cada sprint con UI:** `vectorlab-ux-skills:ux-audit` sobre las pantallas nuevas.
3. **Antes de escribir HTML/CSS/JS de cliente:** `modern-web-guidance` para confirmar el patrón y su respaldo.
4. **Al cierre del sprint 3 y antes de producción:** `owasp-security-audit:security-audit` sobre el repo, resultados a `SECURITY.md`.
5. **Ponytail:** se instala desde tu terminal de Claude Code con `/plugin marketplace add DietrichGebert/ponytail` y `/plugin install ponytail@ponytail`.

---

## 8. Primeros pasos para mañana

1. Crear el repo y la estructura de carpetas.
2. `docker compose up` con Postgres 16 y escribir la migración 001.
3. Configurar templ, Tailwind standalone y `air` en el `Makefile`.
4. Escribir el middleware de headers de seguridad y su test.
5. Maquetar en HTML estático la tarjeta de producto de Vitrina con los tokens de la sección 3.2 y medirla en Lighthouse antes de pasarla a templ.
