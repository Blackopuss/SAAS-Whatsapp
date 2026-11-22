#!/usr/bin/env bash
set -euo pipefail

fail() { echo "ERROR: $*" >&2; exit 1; }
for tool in rclone age pg_restore createdb psql; do command -v "$tool" >/dev/null || fail "falta el comando requerido: $tool"; done
: "${RCLONE_CONFIG_R2_ENDPOINT:?ERROR: configura las variables RCLONE_CONFIG_R2_* para el bucket de respaldos}"
: "${AGE_IDENTITY:?ERROR: define AGE_IDENTITY con la ruta local a la llave privada age (nunca en el VPS)}"
[[ -r "$AGE_IDENTITY" ]] || fail "no se puede leer AGE_IDENTITY=$AGE_IDENTITY."

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
object=${1:-}
if [[ -z $object ]]; then
  object=$(rclone lsf 'r2:tiendas-backups' --files-only | grep -E '^tiendas-[0-9T-]+\.dump\.age$' | sort | tail -n 1 || true)
fi
[[ -n $object ]] || fail "no se encontró respaldo; indica el nombre como primer argumento."
case "$object" in tiendas-*.dump.age) ;; *) fail "nombre de respaldo inválido: $object" ;; esac
output=${2:-./tiendas-restore.dump}
rclone copyto "r2:tiendas-backups/$object" "$work/$object"
age --decrypt --identity "$AGE_IDENTITY" --output "$output" "$work/$object"
chmod 0600 "$output"
echo "Respaldo descifrado en $output"
echo "Ahora sigue la restauración temporal y verificación descritas en docs/PRODUCCION.md."
