#!/usr/bin/env bash
set -euo pipefail

# Sincroniza los rangos oficiales; poner semanalmente en cron. Requiere UFW activo.
[[ ${EUID} -eq 0 ]] || { echo "ERROR: ejecuta como root." >&2; exit 1; }
command -v ufw >/dev/null || { echo "ERROR: ufw no está instalado." >&2; exit 1; }
state=/var/lib/tiendas/cloudflare-ranges
install -d -m 0750 -o root -g root "$(dirname "$state")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSLo "$tmp/ips-v4" https://www.cloudflare.com/ips-v4
curl -fsSLo "$tmp/ips-v6" https://www.cloudflare.com/ips-v6
[[ -s $tmp/ips-v4 && -s $tmp/ips-v6 ]] || { echo "ERROR: Cloudflare devolvió una lista vacía." >&2; exit 1; }
cat "$tmp/ips-v4" "$tmp/ips-v6" | sort -u > "$tmp/new"

# Quita rangos ya retirados y añade los vigentes en ambos puertos.
if [[ -f $state ]]; then
  while IFS= read -r old; do
    [[ -n $old ]] || continue
    if ! grep -Fxq "$old" "$tmp/new"; then
      ufw --force delete allow from "$old" to any port 80 proto tcp >/dev/null 2>&1 || true
      ufw --force delete allow from "$old" to any port 443 proto tcp >/dev/null 2>&1 || true
    fi
  done < "$state"
fi
while IFS= read -r cidr; do
  [[ -n $cidr ]] || continue
  ufw allow from "$cidr" to any port 80 proto tcp comment 'Cloudflare HTTP'
  ufw allow from "$cidr" to any port 443 proto tcp comment 'Cloudflare HTTPS'
done < "$tmp/new"
# Al terminar la primera ejecución, elimina las reglas globales creadas por setup-vps.
ufw --force delete allow 80/tcp >/dev/null 2>&1 || true
ufw --force delete allow 443/tcp >/dev/null 2>&1 || true
install -m 0640 -o root -g root "$tmp/new" "$state"
echo "UFW limitado a los rangos oficiales de Cloudflare para puertos 80/443."
