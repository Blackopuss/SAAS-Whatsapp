#!/usr/bin/env bash
set -euo pipefail

fail() { echo "ERROR: $*" >&2; exit 1; }
[[ ${EUID} -eq 0 ]] || fail "ejecuta como root mediante systemd."
[[ -r /etc/tiendas/backup.env ]] || fail "falta /etc/tiendas/backup.env."
# Este archivo lo administra root; no contiene shell externo ni se pasa como argumento.
set -a
# shellcheck disable=SC1091
source /etc/tiendas/backup.env
set +a
: "${RCLONE_CONFIG_R2_ENDPOINT:?ERROR: configura R2 en /etc/tiendas/backup.env}"
[[ -r /etc/tiendas/backup.pub ]] || fail "falta /etc/tiendas/backup.pub."
[[ -r ${PGPASSFILE:-} ]] || fail "falta PGPASSFILE legible (normalmente /etc/tiendas/pgpass)."
recipient=$(tr -d '\r\n' < /etc/tiendas/backup.pub)
[[ $recipient == age1* ]] || fail "la clave pública en /etc/tiendas/backup.pub no parece una clave age válida."
command -v pg_dump >/dev/null || fail "falta pg_dump (postgresql-client-16)."
command -v age >/dev/null || fail "falta age."
command -v rclone >/dev/null || fail "falta rclone."

umask 077
tmpdir=$(mktemp -d /var/lib/tiendas/backups/run.XXXXXX)
trap 'rm -rf "$tmpdir"' EXIT
stamp=$(TZ=America/Mexico_City date +%Y-%m-%dT%H-%M-%S)
dump="$tmpdir/tiendas-$stamp.dump"
encrypted="$tmpdir/tiendas-$stamp.dump.age"
pg_dump --no-password --format=custom --host=127.0.0.1 --username=tiendas_migrator --dbname=tiendas --file="$dump"
age --encrypt --recipient "$recipient" --output "$encrypted" "$dump"
rclone copyto "$encrypted" "r2:tiendas-backups/tiendas-$stamp.dump.age" --checksum

# Respaldo local de retención por si la regla de ciclo de vida de R2 no está lista.
rclone delete "r2:tiendas-backups" --min-age 30d --include 'tiendas-*.dump.age' || fail "no se pudo aplicar la retención remota; revisa la regla de ciclo de vida de R2."
echo "Respaldo cifrado subido: tiendas-$stamp.dump.age"
