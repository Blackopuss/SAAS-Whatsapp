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

Ramas: se trabaja en `desarrollo`; pasa a `main` solo con CI verde. El repo es público (CI gratis). Sin `Co-Authored-By` ni atribución a Claude en commits o PRs.

## Cómo correrlo

- `docker compose up -d app` levanta Postgres 16 y el servidor con recarga en caliente (air en modo polling) en http://ana.localhost:8080. Se corre en Docker porque Windows Defender bloquea los binarios recién compilados. El servicio `web` compila `frontend/` una vez al arrancar; para editar el frontend en vivo corre `cd frontend && npm run watch` en Windows (el polling de Docker no ve los cambios).
- `.env` (copia de `.env.example`) lo lee el contenedor; dentro de Docker la base se llama `db`.
- Tiendas demo: `ana@demo.mx` / `demo-ana-2026`, `luz@demo.mx` / `demo-luz-2026`. Superadmin: http://admin.localhost:8080, `admin@demo.mx` / `admin-dev-2026-largo` + TOTP. Su clave está en `e2e/.admin-totp` (y el enlace `otpauth://` en `e2e/.admin-totp-uri`), fuera de git; si se pierde: `go run ./cmd/createadmin -reset -email admin@demo.mx`.
- `go run ./cmd/createstore`, `./cmd/createadmin` (imprime la clave TOTP), `./cmd/seeddemo -slug ana [-fotos carpeta]` (borra y llena 20 productos; usa R2 si hay variables).
- Tests: `TEST_DATABASE_URL=postgres://tiendas_app:app_dev@localhost:5432/tiendas?sslmode=disable go test ./...`. `-race` solo en CI (necesita cgo).
- Revisión móvil: `cd e2e && ADMIN_TOTP=<clave de createadmin> npm run check` (sin `ADMIN_TOTP` se salta el superadmin; con él pausa y reactiva a luz y espera 61 s por el caché). (Playwright 360 px + axe-core + Lighthouse; falla si axe encuentra algo o accesibilidad < 100). Capturas en `e2e/out/`. Rápido, sin Lighthouse: `cd e2e && node shots.mjs` (tienda a 360 px, axe, desborde lateral). Si Lighthouse falla con `Network.enable timed out`, quedó un Chrome viejo en el puerto 9333.
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
- argon2id; límite 5 intentos / 15 min por IP y por email; mismo mensaje exista o no la cuenta.
- CSRF con `http.NewCrossOriginProtection` (stdlib). CSP con nonce por petición, sin `unsafe-inline` (los colores por producto van en `<style nonce>`); `frame-ancestors 'none'` salvo la vista previa del panel.
- Fotos: tipo real por magic bytes, 10 MB, 40 MP, recodificadas a WebP 400/800/1200 (quita EXIF), llave aleatoria por tienda.
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
- **Estilos por giro**: columna `stores.style` (vitrina, boutique, barberia, carniceria, abarrotes, papeleria); mismo marcado; cada giro es un tema Astryx (`frontend/src/themes/`, compilado a `theme-<giro>.css`) puesto en `<html data-astryx-theme>`; barbería es oscuro. El color de marca sobrescribe `--color-accent` en un `<style nonce>`. En bloques de fondo invertido hay que poner `color: inherit` a `h2`/`p`, porque el tema les fija el color de texto. Catálogo en `internal/web/views/styles.go` (nombre, fuente que se precarga, color sugerido). Fuentes self-hosted OFL (`web/static/fonts/OFL-estilos.txt`). Exploración con 3 propuestas por giro y 5 giros nuevos (panadería, florería, ferretería, antojitos, mascotas) en el artifact "Estilos por giro".
- **Codex**: hace diseño y archivos de despliegue en worktrees (`../Saas-Whatsapp-visual`, `../Saas-Whatsapp-prod`) con `codex exec -m gpt-6-luna -s workspace-write` (el modelo por defecto de su config no sirve con cuenta ChatGPT). Su sandbox no puede hacer commit: los hace Claude tras revisar. Se integra solo si e2e sigue en 100.
- **Open Graph**: cada foto y logo guarda además `<base>-og.jpg` (1200 px) porque WhatsApp no siempre muestra WebP.
- **IP real**: la app confía en `X-Real-IP` solo si la conexión viene de loopback (Caddy); Caddy lo pone solo para IPs de Cloudflare.
- **Plantillas**: solo Vitrina. Selector de plantilla y banner llegan con Lista/Boutique.
- **Fuente**: Bricolage Grotesque recortada a latín + español y peso 400–800 (24 KB). No tiene eje de ancho.
- **WAVE** no corre local; se usa axe-core (mismo motor de Lighthouse).
- **Diseño de la tienda**: inspirado en catálogos impresos sin copiarlos: portada con collage, etiquetas de precio en bloque (oferta con % y precio tachado), mosaico de categorías, producto destacado (la mayor oferta real o la novedad). Solo datos reales: nada de descuentos, reseñas, existencias ni urgencia inventados.
- **Inventario**: cantidades y existencias en enteros de milésimas (1 pza = 1000, 0.25 kg = 250); `products.unit` (pieza, kg, g, litro, metro) y `step` (de cuánto en cuánto). `stock` NULL = no lleva inventario (se usa el `sold_out` manual); en queries sale como -1. Agotado = manual o `stock < step`. Con tallas/colores la existencia va en `product_variants.stock`.
- **Pedidos**: al tocar "Enviar pedido por WhatsApp" el carrito manda `POST /pedido` (fetch keepalive, código de 6 letras hecho en el celular, va en el mensaje). Precios salen de la base; 10 pedidos/hora por tienda+IP; idempotente por código. Pendiente no aparta nada; **Vendido** descuenta existencia (UPDATE condicionado, nunca negativa; si no alcanza, 409 y no cambia nada), **Deshacer** la regresa. **No vendido** guarda motivo (1 sin existencia, 2 no respondió, 3 precio, 4 otro); pendientes de más de 7 días pasan solos a motivo 5 en el proceso de cada hora (`billing.go`). Los renglones copian nombre, categoría, talla y color para que el historial no cambie. No se guardan datos de la clienta.
- **Ventas** (`/panel/ventas`): pestañas Resumen / Pendientes / Vendidos / No vendidos; resumen por periodo (este mes, mes pasado, 3 meses, año) calculado con GROUP BY al vuelo, sin tablas de resumen; CSV con BOM para Excel. Gráficas SVG a mano (Astryx no trae), una serie, con tabla equivalente.
- **CSP y Astryx**: las capas flotantes meten un `<style>` sin nonce; `shell.tsx` da esas reglas como hoja construida (`adoptedStyleSheets`) y pone el marcador con su id para que no lo intenten.
- **Mensajes**: `/panel/mensajes` trae 9 plantillas de WhatsApp (bienvenida a clientas inactivas) para copiar; no se envían solas.

## Pendientes conocidos

- Rentar VPS y seguir `docs/PRODUCCION.md`. Tras cambiar el estado de una tienda, otras réplicas tardan hasta 60 s en verlo (caché de tiendas).
- Elegir propuestas del artifact de estilos y, si se quieren, agregar los giros nuevos (columna `style` + CSS).
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
