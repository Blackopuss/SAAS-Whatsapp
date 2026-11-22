# Seguridad

Auditoría OWASP Top 10:2025 (plan, sección 5). Primera pasada: 2026-11-22, sobre `desarrollo` antes de producción. Lo que depende del servidor real (TLS, puertos, headers vistos desde fuera) se verifica con la lista de `docs/PRODUCCION.md` el día del primer deploy.

¿Encontraste un problema? Escribe al dueño del repositorio por un canal privado; no abras un issue público.

| # | Riesgo | Estado | Evidencia |
| --- | --- | --- | --- |
| A01 | Control de acceso | Cumple | Toda query lleva `store_id` del Host; FKs compuestas `(store_id, id)`; lo ajeno da 404. `isolation_test.go` cruza tiendas en productos, fotos, tienda pública y pagos; superadmin en otro host con cookie propia SameSite=Strict. |
| A02 | Configuración | Cumple en código; verificar en prod | CSP con nonce sin `unsafe-inline`, `frame-ancestors 'none'`, errores 500 sin detalle al usuario. HSTS lo pone Caddy. Pendiente al desplegar: `curl -I`, Mozilla Observatory, `nmap` desde fuera. |
| A03 | Cadena de suministro | Cumple | `govulncheck`: sin vulnerabilidades (2026-11-22) y corre en CI. Sin JS de terceros: carrito y panel son JS propio servido desde el dominio; fuentes self-hosted. |
| A04 | Criptografía | Cumple | argon2id; cookies `Secure` en prod; secreto TOTP cifrado AES-GCM; tokens de recuperación solo como hash; respaldos cifrados con `age` (llave privada fuera del servidor). TLS lo termina Caddy/Cloudflare: verificar con `testssl.sh`. |
| A05 | Inyección | Cumple | SQL solo por sqlc parametrizado (sin `Sprintf` en `internal/db`). templ escapa todo; el único `templ.Raw` es el `<style nonce>` y solo recibe colores validados por `isHex`/`theme.Fit`. Link `wa.me` con `url.QueryEscape`. Llaves de foto aleatorias, nunca el nombre del archivo. `gosec` limpio. |
| A06 | Diseño | Cumple | Precio y existencias se validan contra `/carrito.json` del servidor; el mensaje de WhatsApp no confía en el `localStorage`. Límite de 10 fotos por producto, 10 MB y 40 MP por foto. |
| A07 | Autenticación | Cumple | 5 intentos / 15 min por IP y por email (IP real detrás de Caddy, `clientip_test.go`); token de sesión nuevo al entrar; cambiar contraseña invalida sesiones; TOTP sin reuso de código; mismo mensaje exista o no la cuenta. |
| A08 | Integridad | Cumple con `deploy/deploy.sh` | El deploy verifica SHA256 del binario en el servidor antes de cambiar de versión y regresa a la anterior si `/healthz` falla. Migraciones con goose versionadas en el repo. |
| A09 | Logging y alertas | Parcial | Se registran logins fallidos y bloqueados, acciones del admin y pausas automáticas (`admin_audit`); el enlace de recuperación solo sale en el log en dev. **Pendiente:** alerta automática al pasar de 10 logins fallidos (hoy solo queda en el log). |
| A10 | Condiciones excepcionales | Parcial | `recover` en middleware, timeouts de lectura/escritura del servidor, `MaxBytesHandler`, límite de 2 fotos procesándose a la vez. **Pendiente:** `statement_timeout` en Postgres (se agrega en `DATABASE_URL` de producción, ver `docs/PRODUCCION.md`) y prueba con R2 caído. |

## Extras multi-tenant

1. Subdominios reservados: `ValidSlug` rechaza `www`, `panel`, `admin`, `api`, `mail` y `static` (`internal/web/tenant.go`).
2. Slugs solo `a-z0-9-`, de 3 a 30 caracteres (sin homoglifos porque no se acepta Unicode).
3. Purga de caché de Cloudflare por tienda: no aplica todavía (el HTML no se cachea en Cloudflare).
4. Dominio propio con `on_demand_tls`: no activo; el bloque comentado en `deploy/Caddyfile` exige un endpoint `ask` interno antes de habilitarlo.
