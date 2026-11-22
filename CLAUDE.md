# Tiendas por WhatsApp (SaaS)

Catálogo en línea por subdominio (`ana.<dominio>`) para vendedoras; la compradora arma el carrito y manda el pedido por WhatsApp. UI en español (es-MX), móvil primero. El plan completo está en `PLAN_IMPLEMENTACION.md`; este archivo dice qué ya está hecho, qué decidimos distinto y cómo se corre.

## Estado (2026-11-22)

| Sprint | Estado | Notas |
| --- | --- | --- |
| 0 Esqueleto | Hecho | CI verde en `main` |
| 1 Auth y tenant | Hecho | Incluye recuperación de contraseña y superadmin con 2FA (adelantado del 5) |
| 2 Catálogo en el panel | Hecho | Categorías, productos, variantes talla × color, marcas, fotos |
| 3 Vitrina y carrito | Hecho | Inicio, categoría, producto, preguntas, carrito, mensaje de WhatsApp |
| 4 Personalización | Hecho | Mi tienda, color con contraste, vista previa, logo, QR, Open Graph |
| 5 Fundadoras y cobro | Hecho | Alta de tiendas, pagos manuales, racha, pausa automática, aviso en panel, bitácora |
| 6 Producción | Pendiente | |

Ramas: se trabaja en `desarrollo`; pasa a `main` solo con CI verde. El repo es público (CI gratis). Sin `Co-Authored-By` ni atribución a Claude en commits o PRs.

## Cómo correrlo

- `docker compose up -d app` levanta Postgres 16 y el servidor con recarga en caliente (air en modo polling) en http://ana.localhost:8080. Se corre en Docker porque Windows Defender bloquea los binarios recién compilados.
- `.env` (copia de `.env.example`) lo lee el contenedor; dentro de Docker la base se llama `db`.
- Tiendas demo: `ana@demo.mx` / `demo-ana-2026`, `luz@demo.mx` / `demo-luz-2026`. Superadmin: http://admin.localhost:8080, `admin@demo.mx` / `admin-dev-2026-largo` + TOTP.
- `go run ./cmd/createstore`, `./cmd/createadmin` (imprime la clave TOTP), `./cmd/seeddemo -slug ana [-fotos carpeta]` (borra y llena 20 productos; usa R2 si hay variables).
- Tests: `TEST_DATABASE_URL=postgres://tiendas_app:app_dev@localhost:5432/tiendas?sslmode=disable go test ./...`. `-race` solo en CI (necesita cgo).
- Revisión móvil: `cd e2e && ADMIN_TOTP=<clave de createadmin> npm run check` (sin `ADMIN_TOTP` se salta el superadmin; con él pausa y reactiva a luz y espera 61 s por el caché). (Playwright 360 px + axe-core + Lighthouse; falla si axe encuentra algo o accesibilidad < 100). Capturas en `e2e/out/`.
- Generado: `go tool templ generate`, `sqlc generate` (CI revisa que no haya diff). Lint: `golangci-lint run`; `gosec -exclude-generated ./...`.

## Stack y configuración

- Go 1.27 con `net/http` ServeMux, templ, sqlc (pgx/v5, `emit_interface`), goose, scs + pgxstore. Tailwind v4 standalone solo para panel/admin; la tienda usa `web/static/css/vitrina.css` a mano.
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
- **Fotos**: solo WebP (sin AVIF, a pedido); se procesan en la petición con 2 a la vez, no en River (`shortcut:` en `photos.go`). El celular las reduce a 1600 px antes de subir (`fotos.js`).
- **Carrito**: JS propio (~2.5 KB) en vez de Alpine; precios y existencias se validan contra `/carrito.json`. No hay página de carrito, solo el `<dialog>`.
- **htmx**: no se usa; la vista previa recarga el iframe con JS mínimo (`mitienda.js`).
- **Recuperación sin proveedor**: en dev el enlace sale en el log; en prod lo genera el superadmin y lo manda por WhatsApp. Pendiente: Cloudflare Email cuando haya dominio.
- **Cobro**: pagos manuales registrados por el superadmin (sin pasarela). Racha = meses pagados a tiempo; pagar tarde la reinicia y cuenta desde el día del pago. Al 6.º día de atraso la tienda se pausa (proceso cada hora en el mismo servidor, `shortcut:` en `billing.go`, no River). Pausada: 503 sin datos de la vendedora; el panel sigue abierto. Aviso en el panel desde 3 días antes de vencer. Fechas en America/Mexico_City.
- **Alta de tiendas**: la hace el superadmin; la dueña recibe por WhatsApp el enlace para crear su contraseña.
- **Diseño visual**: se delega a Codex en la rama `visual` (worktree `../Saas-Whatsapp-visual`); se integra solo si e2e sigue en 100.
- **Plantillas**: solo Vitrina. Selector de plantilla y banner llegan con Lista/Boutique.
- **Fuente**: Bricolage Grotesque recortada a latín + español y peso 400–800 (24 KB). No tiene eje de ancho.
- **WAVE** no corre local; se usa axe-core (mismo motor de Lighthouse).
- **Diseño**: la UI es funcional y básica a propósito; se pule después. Prioridad actual: funcionalidad y UX.

## Pendientes conocidos

- Sprint 6 completo. Tras cambiar el estado de una tienda, otras réplicas tardan hasta 60 s en verlo (caché de tiendas). `clientIP` usa RemoteAddr (`shortcut:` en `auth.go`): detrás de Cloudflare hay que leer el header del proxy.
- Revisar que WhatsApp muestre `og:image` en WebP; si no, generar un JPG para Open Graph.
- El presupuesto de peso (60 KB) se midió a mano (~33 KB), no está en CI.
- Fotos reales para la demo: `demo/fotos/` (fuera de git).
