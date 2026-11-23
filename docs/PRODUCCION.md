# Guía de producción

Esta guía instala una instancia en Ubuntu 24.04 LTS, con PostgreSQL local, Caddy delante de la app y archivos estáticos para cada release. Haz los pasos en orden y conserva la llave SSH, la llave privada age y los datos de R2 fuera del servidor.

## 1. Contrata el VPS y prepara DNS

1. Renta un VPS Ubuntu 24.04 LTS con al menos 2 GB de RAM y una IPv4 pública. Conserva su IP.
2. En Cloudflare, crea `A` para el dominio raíz hacia la IP y activa el proxy naranja. Crea `A` para `*` hacia la misma IP y activa también el proxy. El proxy DNS de Cloudflare debe estar activo para ambos.
3. Antes de cerrar la sesión root, añade la llave pública SSH que usarás para administrar el servidor en `/root/.ssh/authorized_keys`. Comprueba que puedes entrar con ella.
4. En Cloudflare crea un API token con permiso mínimo **Zone / DNS / Edit**, limitado exclusivamente a la zona de este dominio. No uses Global API Key. Se usa para el desafío DNS-01 de Caddy; el secreto se guardará en `/etc/caddy/caddy.env`.
5. Crea en R2 el bucket de imágenes `tiendas-prod` y el bucket privado `tiendas-backups`. Genera credenciales S3 de alcance mínimo: la app solo necesita el bucket de imágenes; la credencial de respaldo debe poder escribir y listar/borrar objetos únicamente en `tiendas-backups`.
6. En tu computadora instala `age`, `rclone`, `pg_dump`/`pg_restore` cliente PostgreSQL 16 y `ssh`, `scp`, `make`, Go según `CLAUDE.md`. Genera una pareja de respaldo **en tu computadora**:

   ```bash
   age-keygen -o tiendas-backup.agekey
   age-keygen -y tiendas-backup.agekey > tiendas-backup.pub
   ```

   Guarda `tiendas-backup.agekey` en un gestor de contraseñas o almacenamiento cifrado. Nunca lo copies al VPS. El contenido de `tiendas-backup.pub` sí se copiará al VPS.

## 2. Instala el VPS

Conéctate como root y ejecuta el instalador del repo desde tu computadora:

```bash
scp deploy/setup-vps.sh root@IP_DEL_VPS:/root/setup-vps.sh
ssh root@IP_DEL_VPS 'bash /root/setup-vps.sh'
```

El script crea `tiendas` (cuenta sin login) y `deploy` (sudo, llave SSH copiada desde la llave autorizada de root y contraseña solo para sudo), desactiva login SSH por contraseña y root, activa UFW, actualizaciones automáticas y fail2ban, instala PostgreSQL 16 desde PGDG y crea la base/roles/extensión `citext`. Los passwords DB se generan con OpenSSL. El de la app queda en `/etc/tiendas/env`, `root:tiendas`, modo `0640`; el de migración queda en `/etc/tiendas/pgpass` (`root:root`, `0600`, para el respaldo) y en `/etc/tiendas-migrate/pgpass` (`tiendas-migrate`, `0600`, para las migraciones), para que el proceso normal no reciba esa clave. El usuario de sistema `tiendas-migrate` solo corre goose.

El instalador permite repetirse: conserva los passwords iniciales en `/root/.tiendas-setup-secrets` (root, modo `0600`) y no reemplaza los archivos de entorno ya editados. Si modificas los passwords manualmente, actualiza los roles de PostgreSQL, `/etc/tiendas/pgpass` y las URLs juntas.

Completa `/etc/tiendas/env` por SSH (`sudoedit /etc/tiendas/env`) con las variables de la aplicación descritas en `CLAUDE.md` y la dirección de escucha:

```dotenv
APP_ENV=prod
ADDR=127.0.0.1:8080
BASE_DOMAIN=ejemplo.mx
# statement_timeout: una consulta lenta se corta a los 5 s en vez de colgar la petición (SECURITY.md, A10)
DATABASE_URL=postgres://tiendas_app:...@127.0.0.1:5432/tiendas?sslmode=disable&statement_timeout=5000
R2_ACCOUNT_ID=...
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET=tiendas-prod
R2_PUBLIC_URL=https://...tu-dominio-publico-de-R2...
TOTP_KEY=...64-caracteres-hex-generados-con-openssl-rand-hex-32...
```

Los valores `R2_*` de este archivo son las credenciales solo para imágenes. En `/etc/tiendas/backup.env` (`root:root`, modo `0600`) completa por separado los valores usados solo por el respaldo:

```dotenv
PGPASSFILE=/etc/tiendas/pgpass
RCLONE_CONFIG_R2_TYPE=s3
RCLONE_CONFIG_R2_PROVIDER=Cloudflare
RCLONE_CONFIG_R2_ENV_AUTH=true
RCLONE_CONFIG_R2_ENDPOINT=https://ACCOUNT_ID.r2.cloudflarestorage.com
RCLONE_CONFIG_R2_ACCESS_KEY_ID=...credencial-solo-para-respaldos...
RCLONE_CONFIG_R2_SECRET_ACCESS_KEY=...
RCLONE_CONFIG_R2_ACL=private
```

El token Cloudflare DNS va solo en `/etc/caddy/caddy.env`, junto con `BASE_DOMAIN=ejemplo.mx`; protégelo como `root:caddy`, modo `0640`. Así el proceso de la app no puede leer el token DNS ni las credenciales S3 de los respaldos. `PGPASSFILE` contiene la ruta al archivo, no una clave. Usa `openssl rand -hex 32` para `TOTP_KEY`. El ejemplo contiene marcadores falsos: no dejes ninguno al iniciar. Los passwords URL generados por el instalador son hexadecimales y seguros.

Copia la llave pública age a `/etc/tiendas/backup.pub` y confirma `root:tiendas`, modo `0640`:

```bash
scp tiendas-backup.pub deploy@IP_DEL_VPS:/tmp/backup.pub
ssh deploy@IP_DEL_VPS 'sudo install -o root -g tiendas -m 0640 /tmp/backup.pub /etc/tiendas/backup.pub'
```

Desde la raíz del repo copia los archivos de operación al VPS. Usa `/tmp/tiendas-deploy` como origen en los siguientes pasos:

```bash
scp -r deploy deploy@IP_DEL_VPS:/tmp/tiendas-deploy
```

Los bloques de administración con `sudo` que siguen se ejecutan en una terminal conectada al VPS (`ssh deploy@IP_DEL_VPS`). El `setup-vps.sh` inicial se ejecuta como root y no se vuelve a correr desde `deploy`.

## 3. Instala y configura Caddy

El paquete oficial de Caddy instala el servicio y el usuario `caddy`, pero no incluye DNS de Cloudflare. En tu computadora descarga el binario con el módulo requerido:

```bash
curl -fL 'https://caddyserver.com/api/download?os=linux&arch=amd64&p=github.com/caddy-dns/cloudflare' -o caddy
```

Verifica el SHA-256 en el origen y después de copiarlo (la segunda comprobación confirma que el archivo no cambió durante el traslado). Para una verificación de procedencia reproducible, compílalo con `xcaddy` desde los tags/versiones fijados de Caddy y del módulo Cloudflare, y conserva el comando y el hash del artefacto. No copies un binario sin verificar. Súbelo y sustituye el ejecutable del paquete usando `dpkg-divert`, para que una actualización del paquete no lo sobrescriba:

```bash
sha256sum caddy
scp caddy deploy@IP_DEL_VPS:/tmp/caddy
ssh deploy@IP_DEL_VPS
sha256sum /tmp/caddy
chmod +x /tmp/caddy
/tmp/caddy version
/tmp/caddy list-modules | grep '^dns.providers.cloudflare$'
sudo install -o root -g root -m 0755 /tmp/caddy /usr/local/sbin/caddy-cloudflare
sudo dpkg-divert --local --add --rename --divert /usr/bin/caddy.distrib /usr/bin/caddy
sudo ln -sfn /usr/local/sbin/caddy-cloudflare /usr/bin/caddy
```

Guarda `deploy/Caddyfile` como `/etc/caddy/Caddyfile`, revisa que `BASE_DOMAIN` coincida y valida antes de reiniciar:

```bash
sudo install -o root -g root -m 0644 /tmp/tiendas-deploy/Caddyfile /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
sudo systemctl daemon-reload
sudo systemctl enable --now caddy
sudo systemctl restart caddy
sudo systemctl status caddy
```

El Caddyfile confía en los rangos oficiales de Cloudflare y toma la IP de la compradora de `CF-Connecting-IP` (`client_ip_headers`; el `X-Forwarded-For` que manda el cliente no cuenta); mantenlos actualizados al mismo tiempo que las reglas UFW. Los logs JSON se guardan en `/var/log/caddy/tiendas-access.json` y Caddy conserva 14 rotaciones de hasta 100 MB.

## 4. Restringe el origen al proxy Cloudflare

UFW inicialmente permite 22 (limitado), 80 y 443. Desde el VPS, ejecuta la sincronización y comprueba SSH antes de cerrar la conexión actual:

```bash
sudo install -o root -g root -m 0755 /tmp/tiendas-deploy/cloudflare-ufw.sh /usr/local/sbin/cloudflare-ufw
sudo /usr/local/sbin/cloudflare-ufw
sudo ufw status numbered
```

El script descarga `ips-v4` e `ips-v6`, agrega las redes oficiales de Cloudflare a 80/443, retira redes descontinuadas y elimina las reglas globales. Instala tarea semanal:

```bash
echo '17 4 * * 1 root /usr/local/sbin/cloudflare-ufw' | sudo tee /etc/cron.d/cloudflare-ufw
```

`deploy/Caddyfile` también contiene la lista de rangos para `trusted_proxies`; cuando la lista oficial cambie, actualiza esa línea y reinicia Caddy. No cierres 80/443 a Cloudflare antes de confirmar que DNS proxy está activo y que Caddy emitió los certificados.

## 5. Instala servicio y realiza el primer deploy

Instala los archivos systemd y scripts como administrador:

```bash
sudo install -o root -g root -m 0644 /tmp/tiendas-deploy/tiendas.service /etc/systemd/system/tiendas.service
sudo install -o root -g root -m 0644 /tmp/tiendas-deploy/tiendas-migrate.service /etc/systemd/system/tiendas-migrate.service
sudo install -o root -g root -m 0755 /tmp/tiendas-deploy/backup.sh /usr/local/sbin/tiendas-backup
sudo install -o root -g root -m 0644 /tmp/tiendas-deploy/tiendas-backup.service /etc/systemd/system/tiendas-backup.service
sudo install -o root -g root -m 0644 /tmp/tiendas-deploy/tiendas-backup.timer /etc/systemd/system/tiendas-backup.timer
sudo systemctl daemon-reload
sudo systemctl enable tiendas
sudo systemctl enable --now tiendas-backup.timer
```

En el equipo de despliegue, define host SSH y dominio. El usuario `deploy` debe poder escribir bajo `/opt/tiendas/releases`:

```bash
export DEPLOY_HOST=deploy@IP_DEL_VPS
export BASE_DOMAIN=ejemplo.mx
make deploy
```

`build-prod` compila Linux/amd64 sin CGO, crea server, `createadmin`, `createstore` y goose, calcula `SHA256SUMS` e incluye migraciones. El deploy verifica el checksum remoto, mueve atómicamente `current`, reinicia y comprueba `/healthz` local y vía HTTPS; ante fallo recupera el symlink anterior y reinicia el servicio. Requiere DNS/Caddy funcionando desde la primera publicación. Para volver manualmente:

```bash
ssh deploy@IP_DEL_VPS 'ls -1 /opt/tiendas/releases'
ssh deploy@IP_DEL_VPS 'ln -sfn /opt/tiendas/releases/SHA_ANTERIOR /opt/tiendas/current && sudo systemctl restart tiendas'
```

El release contiene goose y las migraciones. Al arrancar o reiniciar `tiendas`, systemd corre antes `tiendas-migrate.service` (goose como `tiendas-migrate`, con `/etc/tiendas-migrate/pgpass`); si falla, la app no arranca. Nada del release corre como root: `deploy` controla `/opt/tiendas`, pero lo que hay ahí solo corre como `tiendas` o `tiendas-migrate`. El respaldo (root) usa la copia instalada en `/usr/local/sbin/tiendas-backup`; si cambias `deploy/backup.sh`, reinstálala con el comando de arriba. systemd limita filesystem, capacidades y llamadas al sistema de ambos servicios.

## 6. Crea el primer superadmin y verifica

El primer deploy debe dejar la base migrada. Ejecuta `createadmin` desde el VPS para que use `DATABASE_URL` y `TOTP_KEY`:

```bash
ssh deploy@IP_DEL_VPS
sudo -u tiendas bash -lc 'set -a; source /etc/tiendas/env; set +a; /opt/tiendas/current/bin/createadmin'
```

Sigue las preguntas del comando; guarda la semilla TOTP que imprime en el autenticador del dueño y luego elimínala de cualquier nota temporal. Comprueba servicio y dominio:

```bash
sudo systemctl status tiendas
sudo journalctl -u tiendas -n 100 --no-pager
curl -fsS https://ejemplo.mx/healthz
curl -fsSI https://admin.ejemplo.mx/
```

Entra a `https://admin.ejemplo.mx/` y crea la primera tienda desde el superadmin. Después revisa `https://ana.ejemplo.mx/healthz` (sustituye `ana` por su slug) y el catálogo.

## 7. Respaldos R2 y prueba de restauración

El timer ejecuta el respaldo diario a las 03:30 de `America/Mexico_City`. El servicio ejecuta `pg_dump -Fc` con el rol migrador y usa `/etc/tiendas/pgpass` mediante `PGPASSFILE`, sin contraseña en los argumentos. Cifra el dump con `age -r` usando la llave pública de `/etc/tiendas/backup.pub` (la llave privada permanece fuera del VPS) y sube `tiendas-AAAA-MM-DDTHH-MM-SS.dump.age` al bucket privado `tiendas-backups`. Retención de respaldo en el bucket:

1. En Cloudflare Dashboard abre **R2 → tiendas-backups → Settings → Object lifecycle rules**.
2. Crea una regla para todo el bucket que elimine objetos después de 30 días y guarda.
3. Confirma la regla; el script también elimina objetos con más de 30 días como respaldo adicional.

Inspecciona ejecución con `sudo systemctl start tiendas-backup.service` y `sudo journalctl -u tiendas-backup.service`. Programa una prueba de restauración tras el primer respaldo y periódicamente:

1. En tu computadora configura las variables `RCLONE_CONFIG_R2_*` con las credenciales de solo lectura (o de respaldo, guardadas localmente) y `AGE_IDENTITY` con la ruta a `tiendas-backup.agekey`.
2. Baja y descifra el respaldo más reciente:

   ```bash
   export AGE_IDENTITY="$PWD/tiendas-backup.agekey"
   export RCLONE_CONFIG_R2_TYPE=s3 RCLONE_CONFIG_R2_PROVIDER=Cloudflare RCLONE_CONFIG_R2_ENV_AUTH=true
   export RCLONE_CONFIG_R2_ENDPOINT=https://ACCOUNT_ID.r2.cloudflarestorage.com
   export RCLONE_CONFIG_R2_ACCESS_KEY_ID=... RCLONE_CONFIG_R2_SECRET_ACCESS_KEY=...
   bash ./deploy/restore.sh
   ```

   El comando informa el nombre del respaldo si necesitas conservar evidencia. `restore.sh [nombre.dump.age] [salida.dump]` permite elegir un respaldo y archivo local.
3. En una instalación local de PostgreSQL 16 vacía, crea una base aislada y restaura:

   ```bash
   createdb tiendas_restore_test
   pg_restore --no-owner -d tiendas_restore_test tiendas-restore.dump
   psql -d tiendas_restore_test -c 'SELECT count(*) AS tiendas FROM stores;'
   psql -d tiendas_restore_test -c 'SELECT count(*) AS productos FROM products;'
   psql -d tiendas_restore_test -c 'SELECT count(*) AS pagos FROM payments;'
   ```

   Comprueba que los conteos coinciden razonablemente con la producción y que no hay errores de restauración. `payments` puede ser cero si aún no hay pagos registrados.
4. Borra la base temporal y el dump descifrado (contiene datos personales):

   ```bash
   dropdb tiendas_restore_test
   shred -u tiendas-restore.dump
   ```

No guardes la llave privada en el VPS ni dejes el dump descifrado en una carpeta sincronizada.

## 8. Monitoreo y operación diaria

Uptime Kuma puede vivir en otro VPS o en un contenedor en este VPS. Para el mismo servidor instala Docker Engine siguiendo su guía oficial y arranca el contenedor con volumen persistente y puerto solo local (la imagen oficial documenta esta forma de instalación y el volumen `/app/data`):

```bash
sudo docker volume create uptime-kuma
sudo docker run -d --restart=unless-stopped --pull=always \
  -p 127.0.0.1:3001:3001 -v uptime-kuma:/app/data \
  --name uptime-kuma louislam/uptime-kuma:2
ssh -L 3001:127.0.0.1:3001 deploy@IP_DEL_VPS
```

Abre `http://127.0.0.1:3001` en tu computadora mientras el túnel SSH está activo. No publiques el puerto 3001 en `0.0.0.0` ni abras ese puerto en UFW. Crea monitores HTTP para `https://ejemplo.mx/healthz` y para `https://ana.ejemplo.mx/` (usa slug real). Añade contacto de alerta Telegram con bot creado mediante BotFather y chat ID. Para WhatsApp, CallMeBot requiere activar el servicio desde el teléfono y aceptar su mensaje de activación; guarda la API key solo en Uptime Kuma, prueba el contacto y no la incluyas en comandos o repo. Consulta [la instalación oficial de Uptime Kuma](https://github.com/louislam/uptime-kuma/wiki/%F0%9F%94%A7-How-to-Install).

```bash
sudo journalctl -u tiendas -f
sudo journalctl -u caddy -f
sudo systemctl list-timers tiendas-backup.timer
sudo systemctl status fail2ban postgresql caddy tiendas
```

La app registra JSON en journald. Caddy escribe acceso JSON en `/var/log/caddy/tiendas-access.json`. No copies logs que incluyan cookies, URLs sensibles o tokens a canales públicos.

## 9. Revisión de seguridad antes de abrir tiendas reales

Ejecuta desde un equipo externo una vez que DNS/TLS estén activos. Registra fecha, comando y salida en el registro de auditoría (`SECURITY.md` del plan):

| Control | Verificación concreta |
| --- | --- |
| A01 Control de acceso | `go test ./internal/web -run 'Isolation|Recovery' -count=1` con `TEST_DATABASE_URL` de una base de pruebas; en producción revisa por separado las queries SQL con `rg -n 'store_id|WHERE' internal/db` y confirma acceso al admin con TOTP. |
| A02 Configuración | `curl -fsSI https://ejemplo.mx/` y `curl -fsSI https://ana.ejemplo.mx/`; `nmap -Pn -sV -p- IP_DEL_VPS` desde fuera (el IP de origen debe mostrar SSH y filtrar 80/443 para una IP que no sea de Cloudflare; los dominios sí deben responder por HTTPS). |
| A03 Cadena de suministro | `govulncheck ./...`; `go list -m all`; revisa versiones fijadas y SRI/localidad de JS con `rg -n 'htmx|alpine|integrity=' web`. |
| A04 Criptografía | `testssl.sh --fast ejemplo.mx`; `sudo stat -c '%a %U:%G %n' /etc/tiendas/env /etc/tiendas/backup.env /etc/tiendas/pgpass /etc/tiendas/backup.pub /etc/caddy/caddy.env`; confirma que solo la clave pública age está en el VPS. |
| A05 Inyección | `gosec -exclude-generated ./...`; `rg -n 'templ\.Raw|fmt\.Sprintf.*(SELECT|INSERT|UPDATE|DELETE)' internal cmd`. Revisa cada coincidencia manualmente. |
| A06 Diseño inseguro | `go test ./internal/web -run 'SalePrice|Options' -count=1`; prueba carrito con precio/stock alterados desde el navegador y confirma que servidor rechaza valores inventados. |
| A07 Autenticación | `go test ./internal/web -run 'Auth|Recovery|Admin' -count=1`; prueba límite de login, rotación de sesión, reset de un uso y TOTP del admin en staging. |
| A08 Integridad | `sha256sum --check SHA256SUMS` dentro del release; realiza un deploy de prueba y simula un healthcheck fallido en staging para confirmar rollback. |
| A09 Logs y alertas | `sudo journalctl -u tiendas --since today`; revisa login fallido y acciones admin, busca que no aparezcan passwords, cookies ni tokens; fuerza alerta de prueba desde Uptime Kuma. |
| A10 Excepciones | `go test ./...`; prueba Postgres/R2 no disponibles en staging, carga superior al límite, timeout y healthcheck. No ejecutes fallas destructivas en producción. |

Requiere además verificar en el firewall del proveedor que los puertos de Postgres (5432) y Uptime Kuma (si está instalado) no estén publicados. `nmap` externo debe confirmar la exposición real, no solo la configuración local. Corrige hallazgos antes de poner tiendas reales.

## Límites conocidos de esta entrega

- `clientIP` de la app usa `X-Real-IP` solo si la conexión viene de loopback (Caddy). Caddy lo llena con `CF-Connecting-IP` y solo para conexiones de Cloudflare. Los límites de intentos cuentan IPv6 por bloque /64.
- Los límites de intentos viven en memoria de un proceso (un solo VPS). Con varias réplicas hay que moverlos a Postgres.
- Alerta de logins fallidos: más de 30 en 10 minutos deja `"event":"login_alert"` (nivel ERROR) en `journalctl -u tiendas`. Para que avise, conecta ese evento a tu monitoreo (p. ej. un monitor de palabra clave o un `journalctl -f | grep` que mande WhatsApp/correo).
- `ADDR` permite fijar el bind y el servicio establece `127.0.0.1:8080`; su valor por defecto en la app es `:8080`.
- `R2_PUBLIC_URL` debe apuntar al dominio público real del bucket de imágenes. El bucket de respaldo permanece privado.
- `deploy/backup.sh` guarda un dump temporal solo durante el proceso y lo borra al salir; el cifrado ocurre antes de subirlo.
