# Tiendas por WhatsApp (SaaS)

Catálogo en línea por subdominio (`ana.<dominio>`) para vendedoras; la compradora arma el carrito y manda el pedido por WhatsApp. UI en español (es-MX), móvil primero. El plan completo está en `PLAN_IMPLEMENTACION.md`; este archivo dice qué ya está hecho, qué decidimos distinto y cómo se corre.

## Estado (2026-11-22)

| Sprint | Estado | Notas |
| --- | --- | --- |
| 0 Esqueleto | Hecho | CI verde en `main` |
| 1 Auth y tenant | Hecho | Incluye recuperación de contraseña y superadmin con 2FA (adelantado del 5) |
| 2 Catálogo en el panel | Hecho | Categorías, productos, variantes talla × color, marcas, fotos |
| 3 Vitrina y carrito | Hecho | Inicio, categoría, producto, preguntas, carrito, mensaje de WhatsApp |
| 4 Personalización | Hecho | Mi tienda, color con contraste, vista previa, logo, QR, Open Graph, 6 estilos por giro |
| 5 Fundadoras y cobro | Hecho | Alta de tiendas, pagos manuales, racha, pausa automática, aviso en panel, bitácora |
| 6 Producción | Listo en el repo | `deploy/` y `docs/PRODUCCION.md`; falta rentar el VPS y seguir la guía. Auditoría en `SECURITY.md` |
| Rediseño React | Hecho | Admin, panel y tienda en React; tienda con portada, mosaico de categorías, destacado, "agregar rápido"; página Mensajes (plantillas de WhatsApp) en el panel |
| Inventario y Ventas | Hecho | Existencias por producto o talla × color, venta a granel por pasos, pedidos desde el carrito, Ventas con gráficas y no ventas; productos en tabla con filtros y formulario en pestañas; superadmin con búsqueda y filtros |
| Rediseño premium | Hecho (2026-11-23) | Tienda con una sola estructura limpia (Apple/Shopify/Amazon como referencia de calidad); el giro solo da acento. Panel con página Inicio (pasos para dejar lista la tienda, pedidos, ventas del mes, compartir) y tema propio `panel`. Reemplaza el "diseño por giro" con estructura propia |

Ramas: se trabaja en `desarrollo`; pasa a `main` solo con CI verde. El repo es público (CI gratis). Sin `Co-Authored-By` ni atribución a Claude en commits o PRs.

## Cómo correrlo

- `docker compose up -d app` levanta Postgres 16 y el servidor con recarga en caliente (air en modo polling) en http://ana.localhost:8080. Se corre en Docker porque Windows Defender bloquea los binarios recién compilados. El servicio `web` compila `frontend/` una vez al arrancar; para editar el frontend en vivo corre `cd frontend && npm run watch` en Windows (el polling de Docker no ve los cambios).
- `.env` (copia de `.env.example`) lo lee el contenedor; dentro de Docker la base se llama `db`.
- Tiendas demo: `ana@demo.mx` / `demo-ana-2026`, `luz@demo.mx` / `demo-luz-2026`. Superadmin: http://admin.localhost:8080, `admin@demo.mx` / `admin-dev-2026-largo` + TOTP. Su clave está en `e2e/.admin-totp` (y el enlace `otpauth://` en `e2e/.admin-totp-uri`), fuera de git; si se pierde: `go run ./cmd/createadmin -reset -email admin@demo.mx`.
- `go run ./cmd/createstore`, `./cmd/createadmin` (imprime la clave TOTP), `./cmd/seeddemo -slug ana [-fotos carpeta]` (borra y llena 20 productos; usa R2 si hay variables); `-catalogo fotos` siembra un producto por foto de `demo/fotos/` (ropa, pan y carne, con datos acordes; las de más de 24 MP van reducidas en `demo/fotos/chicas/`).
- Tests: `TEST_DATABASE_URL=postgres://tiendas_app:app_dev@localhost:5432/tiendas?sslmode=disable go test ./...`. `-race` solo en CI (necesita cgo).
- Revisión móvil: `cd e2e && ADMIN_TOTP=<clave de createadmin> npm run check` (sin `ADMIN_TOTP` se salta el superadmin; con él pausa y reactiva a luz y espera 61 s por el caché). (Playwright 360 px + axe-core + Lighthouse; falla si axe encuentra algo o accesibilidad < 100). Capturas en `e2e/out/`. Rápido, sin Lighthouse: `cd e2e && node shots.mjs` (tienda a 360 px, axe, desborde lateral), `node panel-shots.mjs` (Inicio y Productos del panel a 360 y 1280 px con axe; no escribe en la base) y `node giros-shots.mjs [giro,…]` (inicio con cada giro sin tocar la base). `node giros-full.mjs [giro,…]`: los seis giros con los datos de ana sin guardar nada (reescribe el HTML como la vista previa): portada, categoría, catálogo y ficha a 1440 y 390 px, axe, desborde, fotos rotas, consola, buscar, categoría, agregar sin y con talla, carrito, total, enlace de WhatsApp (sin abrirlo) y quitar; también la vista previa del panel. Capturas `out/g-*`. Si Lighthouse falla con `Network.enable timed out`, quedó un Chrome viejo en el puerto 9333.
- Generado: `go tool templ generate`, `sqlc generate` (CI revisa que no haya diff). Lint: `golangci-lint run`; `gosec -exclude-generated ./...`. Frontend: `cd frontend && npm run typecheck && npm run build && npm run budget` (lo mismo que `make web` y CI).

## Stack y configuración

- Go 1.27 con `net/http` ServeMux, templ, sqlc (pgx/v5, `emit_interface`), goose, scs + pgxstore. Frontend en `frontend/` (React 19 + Vite, salida en `web/static/app/`, no se versiona). Go sigue haciendo rutas, sesión, validación y POST de formularios; cada página es un `reactPage` de templ con los datos en `#page-data` (JSON con nonce). Panel y admin usan componentes Astryx con tema neutral; la tienda es React sin componentes Astryx (pesaban ~190 KB) con `shop.css` a mano sobre los tokens del tema.
- Postgres con dos roles: `tiendas_migrator` (dueño del esquema, corre migraciones) y `tiendas_app` (solo DML). `citext` lo crea el superusuario en `docker/postgres/init.sql`.
- Variables: `APP_ENV`, `BASE_DOMAIN`, `DATABASE_URL`, `MIGRATIONS_URL`, `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET`, `R2_PUBLIC_URL`, `TOTP_KEY` (64 hex; no cambiarla o los admins no entran). En prod R2 y TOTP_KEY son obligatorias.
- Cloudflare: buckets R2 `tiendas-dev` (público por r2.dev) y `tiendas-backups`. El MCP de Cloudflare no puede crear tokens de API; las llaves S3 las crea el dueño en el dashboard.

## Seguridad (lo que ya cumple)

- Cada query lleva `store_id` del Host, nunca del formulario; FKs compuestas `(store_id, id)`. Lo ajeno responde 404. `isolation_test.go` y `recovery_test.go` lo prueban contra Postgres real.
- Sesión: cookie `tiendas_sid` Path=/panel, HttpOnly, SameSite=Lax, Secure en prod; token nuevo al entrar; cambiar contraseña invalida sesiones viejas (`password_changed_at` vs `auth_at`).
- Admin: `admin.<dominio>`, cookie `tiendas_admin` SameSite=Strict, 30 min de inactividad, TOTP RFC 6238 con secreto cifrado AES-GCM y sin reuso de código.
- argon2id; límite 5 intentos / 15 min por IP (IPv6 por /64) y 5 fallos / 15 min por cuenta; con la cuenta bloqueada entra solo un navegador donde ya había entrado (cookie `tiendas_disp`/`tiendas_admin_disp`, HMAC de `TOTP_KEY` + hash de la contraseña, en `guard.go`); mismo mensaje exista o no la cuenta. Límites en memoria con tope de 50 000 llaves; el de cuentas (`accounts`) nunca suelta un bloqueo vivo. Más de 30 fallos en 10 min: `event=login_alert` en el log.
- `createadmin -reset` cierra las sesiones del admin (`admins.credentials_changed_at`); recuperar la contraseña revoca los otros enlaces. La app solo puede INSERT en `admin_audit` (migración 00010).
- Precio normal: la vendedora lo pone al crear el producto; después solo lo cambia el superadmin (`POST /tiendas/{id}/productos/{pid}/precio`, pestaña Precios, queda en `admin_audit`). `UpdateProduct` no escribe `price_cents`. La oferta (`sale_price_cents`, `on_sale`) sigue siendo de la vendedora.
- Auditoría 2026-11-24 (informe fuera del repo, `~/security-audit-skill/Saas-Whatsapp/run-1/`): JPEG con más de 64 scans se rechaza; las tarjetas publican existencias con el mismo tope que `/carrito.json`; recuperar la contraseña recuerda ese navegador; `/panel/*` y admin van con `Cache-Control: no-store`; los 429 llevan `Retry-After`.
- Estado `paused` (pausa manual del superadmin) distinto de `suspended` (atraso): registrar un pago solo reabre `suspended`.
- Despliegue: Caddy toma la IP de `CF-Connecting-IP`; migraciones en `tiendas-migrate.service` con su propio usuario; nada del release corre como root. Acciones de CI fijadas por SHA, Dependabot activo.
- CSRF con `http.NewCrossOriginProtection` (stdlib). CSP con nonce por petición, sin `unsafe-inline` (los colores por producto van en `<style nonce>`); `frame-ancestors 'none'` salvo la vista previa del panel.
- Fotos: tipo real por magic bytes, 10 MB, 24 MP (se decodifica dentro del límite de 2 a la vez), recodificadas a WebP 400/800/1200 (quita EXIF), llave aleatoria por tienda.
- Recuperación: token de 32 bytes, solo su hash en la base, 30 min, un uso, ligado a la tienda; va en `?t=` porque el log guarda solo la ruta.

## Decisiones que se apartan del plan

- **CSRF**: stdlib (Origin/Sec-Fetch-Site) en vez de token en `hx-headers`.
- **Orden de categorías**: botones Subir/Bajar en vez de arrastrar (accesible, sin JS).
- **Fotos**: solo WebP (sin AVIF, a pedido); se procesan en la petición con 2 a la vez, no en River (`shortcut:` en `photos.go`). El celular las reduce a 1600 px antes de subir (`shrink` en `panel.tsx`).
- **Carrito**: en `shop.tsx`, localStorage `carrito-2` (y `ultimo-pedido-2` para "Pedir de nuevo"); precios y existencias se validan contra `/carrito.json`. No hay página de carrito, solo el `<dialog>`. El botón + de cada tarjeta abre "agregar rápido" con tallas y colores sacados de `/carrito.json`.
- **htmx**: no se usa; la vista previa es un iframe en `panel.tsx`.
- **Recuperación sin proveedor**: en dev el enlace sale en el log; en prod lo genera el superadmin y lo manda por WhatsApp. Pendiente: Cloudflare Email cuando haya dominio.
- **Cobro**: pagos manuales registrados por el superadmin (sin pasarela). Racha = meses pagados a tiempo; pagar tarde la reinicia y cuenta desde el día del pago. Al 6.º día de atraso la tienda se pausa (proceso cada hora en el mismo servidor, `shortcut:` en `billing.go`, no River). Pausada: 503 sin datos de la vendedora; el panel sigue abierto. Aviso en el panel desde 3 días antes de vencer. Fechas en America/Mexico_City.
- **Alta de tiendas**: la hace el superadmin; la dueña recibe por WhatsApp el enlace para crear su contraseña.
- **Estilos por giro**: columna `stores.style` (vitrina, boutique, barberia, carniceria, abarrotes, papeleria). Desde el 2026-11-23 (segunda vuelta) **cada giro tiene portada y composición propias** (decisión del dueño, reemplaza "base premium + acento"); el marco (encabezado, carrito, ficha, agregar rápido) es común para no reaprender a comprar. Portadas en `shop.tsx` (`VitrinaHome`, `BoutiqueHome`, `BarberiaHome`, `CarniceriaHome`, `AbarrotesHome`, `PapeleriaHome`; `giro` decide además detalles de tarjeta: muestras de color en boutique/papelería, "Por kg/Por pieza" en carnicería/abarrotes, botón "Agregar" ancho en abarrotes; y de ficha: tabla de datos en barbería/carnicería, total cantidad × precio en lo que va por kilo). Composición en `frontend/src/giros/<giro>.css`, todo dentro de `[data-astryx-theme="<giro>"]` (nada de `:root`). Paleta y letra de cuerpo/títulos salen del tema hecho con el editor (`npx astryx experimental theme editor set <giro> …`: accent, neutral, `--color-background-body` = papel, `--color-text-primary` = tinta, `font … --on body` Bricolage salvo vitrina con letra del sistema); `shop.css` los toma como `--paper`, `--ink`, `--sans` y el color del giro como `--decor` (`--color-icon-accent`, que la marca de la tienda no pisa). Todos son claros; barbería usa bandas carbón, no fondo negro. `seeddemo -slug luz -giro <giro>` siembra catálogo de ese giro. Catálogo en `internal/web/views/styles.go`. Fuentes self-hosted OFL (`web/static/fonts/OFL-estilos.txt`).
- **Codex**: hace diseño y archivos de despliegue en worktrees (`../Saas-Whatsapp-visual`, `../Saas-Whatsapp-prod`) con `codex exec -m gpt-6-luna -s workspace-write` (el modelo por defecto de su config no sirve con cuenta ChatGPT). Su sandbox no puede hacer commit: los hace Claude tras revisar. Se integra solo si e2e sigue en 100.
- **Open Graph**: cada foto y logo guarda además `<base>-og.jpg` (1200 px) porque WhatsApp no siempre muestra WebP.
- **IP real**: la app confía en `X-Real-IP` solo si la conexión viene de loopback (Caddy); Caddy lo pone solo para IPs de Cloudflare.
- **Plantillas**: solo Vitrina. Selector de plantilla y banner llegan con Lista/Boutique.
- **Fuente**: Bricolage Grotesque recortada a latín + español y peso 400–800 (24 KB). No tiene eje de ancho.
- **WAVE** no corre local; se usa axe-core (mismo motor de Lighthouse).
- **Diseño de la tienda** (`shop.tsx`, `shop.css`, `giros/`; sistema en `DESIGN.md`): portada propia por giro (ver Estilos por giro). Tarjeta: foto, "Nuevo" arriba del nombre, nombre, precio grande, opciones ("Tallas 26 a 32", "2 colores" o muestras) y existencia real ("Quedan 3" si son 3 pasos o menos, "8 disponibles"; nada si no lleva inventario); sobre la foto solo "Agotado" o "−N %" (si redondea a 1 % o más). "Nuevo" se apaga en toda la página si más de la mitad es nuevo (`tame`). Ficha: barra fija de compra en el celular (`.buybar`). Solo datos reales: nada de descuentos, reseñas, existencias ni urgencia inventados. Un solo momento de movimiento: el título de la portada.
- **Panel**: `/panel/` es **Inicio** (`home.go`, `Home` en `panel.tsx`): 3 cifras (pedidos por atender, vendido este mes, productos con agotados/poca existencia), lista "Deja lista tu tienda" (WhatsApp, categorías, 3 productos con foto, logo, datos de pago; se oculta al completarse) y "Comparte tu tienda" (copiar, WhatsApp, QR). Productos se movió a `/panel/productos`. En el celular cada producto lleva menú ⋯ (Editar, Ajustar existencias, Ocultar/Mostrar). Cada página puede llevar `description` bajo el título (`Shell`). Panel y superadmin usan el tema `panel` (acento verde `#0f7a4a`, neutro cálido, esquinas `large`, botones píldora), hecho con `npx astryx experimental theme editor … panel` (fuente en `frontend/themes/panel/`, copia en `src/themes/panel/`); se cambia con el editor, no a mano.
- **Inventario**: cantidades y existencias en enteros de milésimas (1 pza = 1000, 0.25 kg = 250); `products.unit` (pieza, kg, g, litro, metro) y `step` (de cuánto en cuánto). `stock` NULL = no lleva inventario (se usa el `sold_out` manual); en queries sale como -1. Agotado = manual o `stock < step`. Con tallas/colores la existencia va en `product_variants.stock`.
- **Pedidos**: en el carrito la clienta escribe "Tu nombre para el pedido" (único dato; 2–60 caracteres, letras/números/espacios/`'.-`, sin espacios de sobra, validado igual en `customerName` de `orders.go`) y toca "Preparar pedido para WhatsApp": `POST /pedido` (código de 6 letras hecho en el celular) responde el pedido registrado (nombre, renglones con subtotal y total) y con eso se arma el mensaje (sin enlaces; `orderText` en `shop.tsx`). "Pedir solo este" usa el mismo carrito sin tocar el guardado. `orders.customer` (migración 00011) es '' en pedidos viejos: Ventas dice "Cliente sin nombre registrado". `e2e/pedido-check.mjs` lo revisa con `/pedido` simulado. Precios salen de la base; 10 pedidos/hora por tienda+IP; idempotente por código. Pendiente no aparta nada; **Vendido** descuenta existencia (UPDATE condicionado, nunca negativa; si no alcanza, 409 y no cambia nada), **Deshacer** la regresa. **No vendido** guarda motivo (1 sin existencia, 2 no respondió, 3 precio, 4 otro); pendientes de más de 7 días pasan solos a motivo 5 en el proceso de cada hora (`billing.go`). Los renglones copian nombre, categoría, talla y color para que el historial no cambie. De la clienta solo se guarda el nombre que escribe.
- **Ventas** (`/panel/ventas`): pestañas Resumen / Pendientes / Vendidos / No vendidos; resumen por periodo (este mes, mes pasado, 3 meses, año; la gráfica dice el mes y llega solo hasta hoy; "Lo más vendido" se ordena por número de pedidos, el dinero desempata) calculado con GROUP BY al vuelo, sin tablas de resumen; CSV con BOM para Excel. Gráficas SVG a mano (Astryx no trae), una serie, con tabla equivalente.
- **CSP y Astryx**: las capas flotantes meten un `<style>` sin nonce; `shell.tsx` da esas reglas como hoja construida (`adoptedStyleSheets`) y pone el marcador con su id para que no lo intenten.
- **Mensajes**: `/panel/mensajes` trae 9 plantillas de WhatsApp (bienvenida a clientas inactivas) para copiar; no se envían solas.

## Pendientes conocidos

- **De la auditoría del 2026-11-24** (detalle en `~/security-audit-skill/Saas-Whatsapp/run-1/REPORT.md` y `NEEDS-VALIDATION.md`, fuera del repo):
  - Cloudflare: regla de caché que no guarde `/panel/*` ni `admin.*`. Validar solo con una tienda de prueba: pedir `/panel/ventas/ventas.csv?periodo=zz-audit` dos veces sin cookie y ver `cf-cache-status`.
  - Revisar en el navegador la pestaña Precios del admin y el precio de solo lectura al editar un producto en el panel (no se vio en pantalla; las pruebas de Go sí pasan). Correr la e2e completa.
  - Recomendaciones sin aplicar:
    - `ADDR` por defecto en loopback.
    - Limitadores en memoria: separar los de tienda y admin; el estricto falla cerrado con 50 000 llaves.
    - `issueReset` no revoca los enlaces anteriores.
    - AAD en el cifrado del TOTP.
    -
    - `X-Frame-Options`.
    - `.env.*`, `*.pem` y `*.key` en `.gitignore`.
    - Tope de `SetOrderTotal`: desborda int32 con cantidades enormes.
    - `productDelete` sin transacción: puede dejar fotos huérfanas.
    - Reparto de `photoSlots` por tienda.
    - Reintento de `/pedido` ligado al mismo contenido.

- Rentar VPS (contratado: 2 vCPU, 8 GB, 100 GB NVMe) y seguir `docs/PRODUCCION.md`, incluida la sección 10 (2FA en cuentas, Cloudflare Access y regla WAF para `admin.`, Authenticated Origin Pulls con certificado propio, SSH solo por Tailscale). Tras cambiar el estado de una tienda, otras réplicas tardan hasta 60 s en verlo (caché de tiendas).
- Giros nuevos propuestos (panadería, florería, ferretería, antojitos, mascotas): columna `style`, tema, `giros/<giro>.css` y catálogo en `seeddemo`. Hay fotos de pan en `demo/fotos/panaderia/`.
- Antes de pasar a `main`: dejar que CI corra la e2e completa con Lighthouse (el rediseño del 2026-11-23 solo se revisó con axe y capturas a 360 y 1280–1366 px; `check.mjs` ya apunta a `/panel/productos`).
- SECURITY.md A09/A10: alerta automática de logins fallidos y prueba con R2 caído.
- La e2e escribe el WhatsApp de prueba en "Mi tienda" de ana en cada corrida (pisa el número que tenga).
- El presupuesto de peso de la tienda (200 KB gzip desde 2026-11-22, antes 130: JS + `shop.css` + el tema/giro más pesado + la fuente más pesada) lo revisa `npm run budget` en CI. React solo ya son ~58 KB. Cada giro tiene su CSS de estructura en `frontend/src/giros/<giro>.css` (importa su tema; solo carga el de la tienda).
- Fotos reales para la demo: `demo/fotos/` (fuera de git).

<!-- ASTRYX:START -->
Astryx v0.7.17 · 174 components
CLI: run every command as `npx astryx <cmd>` (shown below as `astryx ...`).

SETUP (once, in app entry e.g. main.tsx) — without these, components render unstyled:
  import "@astryxdesign/core/reset.css";
  import "@astryxdesign/core/astryx.css";

WORKFLOW — start every page from a template, never from scratch:
1. `astryx build "<idea>"` — names the template to scaffold from and the parts it lacks.
2. `astryx template <name> <path>` — scaffold it; keep its frame, gap and padding; replace the content.
3. `astryx component <Name>` to read props before using one; `astryx template <Block>` prints a block example to read for a part it lacks. Astryx names differ from other libraries: buttons, badges, tokens and tabs take their text in `label` (Text, Heading and Link take children), booleans start with `is` or `has` (`isOpen`, `isDisabled`), and every input takes `value` and `onChange`.
Changing an existing page? Keep it: skip step 2 and add components inside its sections; read a block example when it helps.
Before you deliver a build, run `astryx doctor --built <dir>`; don't deliver it if it fails.

RULES:
- No <div> — components handle all layout and spacing.
- Read `astryx docs layout` before changing a template's frame.
- Dense data = rows (Table, List/Item), not Card-wrapped lists. Badge = counts only.
- Style with component props first, then tokens (var(--color-*|--spacing-*|--radius-*)). No raw hex/px. No StyleX/Tailwind compiler here — don't use xstyle/utility classes directly. Some templates use StyleX and need a compiler plugin; see `astryx docs styling-overview`.
- THEMES: palettes and custom colors go through the theme, never :root overrides; so does a denser or roomier app (`editor set <name> spacing tight`), never gap and padding changes page by page. A change to one kind of part (the sidebar, its selected item, the gaps in a form) is a theme change too, not props on one page: `editor how "<ask>"` names the `set … --on <part>` line. No app yet: `astryx experimental theme preview app <dir>` makes a starter app that shows the theme. To make or change a theme (colors, fonts, corners, spacing, a look from a picture), prefer the experimental theme editor over hand-written theme files: `astryx experimental theme editor new <name> --like neutral`, then `editor set <name> <thing> <value>` or `editor how "<what you want>"`; wire it with `astryx theme add <name>`; after each change look at it with `astryx experimental theme editor show <name> --png` (takes pictures of real components) and open the PNGs it writes; a theme change is not done until you have looked at the pictures. In code, extend an imported theme to customize it; `theme eject <slug>` only forks source. See `astryx docs theme`.
- SELF-CHECK: re-read the file; replace any raw <div>/<span>, imported .css/@apply, or hardcoded #hex/px with the component or a token.

`astryx help` lists every command. Key ones beyond the workflow:
  search "<query>"   find any component / hook / doc / template
  discover <words>   integrations you could add
  docs <topic>       getting-started, principles, tokens, theme … (`astryx docs` lists all)
  docs cli           commands and API reference
  docs authoring     build an integration package, and every field it holds
  upgrade --from <old version> --apply   run after a dependency bump
To remove this block, run `astryx init --remove-agents`, which removes it from the agent files at the project root and in the folder you run it in; from any other file it leaves, delete everything from ASTRYX:START to ASTRYX:END.
<!-- ASTRYX:END -->
