#!/usr/bin/env bash
set -euo pipefail

# Preparación inicial para Ubuntu 24.04 LTS. Ejecutar una vez como root.
if [[ ${EUID} -ne 0 ]]; then echo "ERROR: ejecuta este script como root (sudo -i)." >&2; exit 1; fi
if [[ ! -r /etc/os-release ]] || ! grep -q '^VERSION_ID="24.04"' /etc/os-release; then
  echo "ERROR: este instalador requiere Ubuntu 24.04 LTS." >&2; exit 1
fi
command -v openssl >/dev/null || { echo "ERROR: falta openssl." >&2; exit 1; }
[[ -s /root/.ssh/authorized_keys ]] || { echo "ERROR: agrega primero una llave en /root/.ssh/authorized_keys." >&2; exit 1; }

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl gnupg lsb-release openssh-server sudo ufw \
  unattended-upgrades fail2ban postgresql-common age rclone jq

# Cuentas separadas: el proceso no puede iniciar sesión y deploy solo entra con llave.
if ! id tiendas >/dev/null 2>&1; then useradd --system --create-home --home-dir /var/lib/tiendas --shell /usr/sbin/nologin tiendas; fi
if ! id deploy >/dev/null 2>&1; then useradd --create-home --shell /bin/bash deploy; passwd deploy; fi
usermod -aG sudo deploy
cat > /etc/sudoers.d/tiendas-deploy <<'EOF'
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart tiendas
EOF
chmod 0440 /etc/sudoers.d/tiendas-deploy
visudo -cf /etc/sudoers.d/tiendas-deploy
install -d -m 0700 -o deploy -g deploy /home/deploy/.ssh
install -m 0600 -o deploy -g deploy /root/.ssh/authorized_keys /home/deploy/.ssh/authorized_keys

install -d -m 0755 /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-tiendas-hardening.conf <<'EOF'
PasswordAuthentication no
PermitRootLogin no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
AuthenticationMethods publickey
EOF
sshd -t
systemctl reload ssh

# Solo SSH limitado y HTTP(S); el script cloudflare-ufw.sh restringe 80/443 al proxy.
ufw --force default deny incoming
ufw --force default allow outgoing
ufw limit 22/tcp comment 'SSH limitado'
ufw allow 80/tcp comment 'HTTP Cloudflare'
ufw allow 443/tcp comment 'HTTPS Cloudflare'
ufw --force enable
# Para cerrar 80/443 a las redes de Cloudflare configura deploy/cloudflare-ufw.sh
# en cron semanal; después de probarlo, se eliminan las reglas globales 80/443.

cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF
systemctl enable --now apt-daily.timer apt-daily-upgrade.timer

cat > /etc/fail2ban/jail.d/tiendas-sshd.local <<'EOF'
[sshd]
enabled = true
backend = systemd
maxretry = 5
findtime = 10m
bantime = 1h
EOF
systemctl enable --now fail2ban

# PostgreSQL 16 desde PGDG oficial.
install -d -m 0755 /usr/share/postgresql-common/pgdg
curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor --yes -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg] https://apt.postgresql.org/pub/repos/apt $(. /etc/os-release && echo "$VERSION_CODENAME")-pgdg main" > /etc/apt/sources.list.d/pgdg.list
apt-get update
apt-get install -y postgresql-16 postgresql-client-16
systemctl enable --now postgresql

if [[ -f /root/.tiendas-setup-secrets ]]; then
  # Archivo local generado por este script, legible solo por root para conservar claves al repetirlo.
  # shellcheck disable=SC1091
  source /root/.tiendas-setup-secrets
else
  APP_PASS=$(openssl rand -hex 32)
  MIG_PASS=$(openssl rand -hex 32)
  install -m 0600 /dev/null /root/.tiendas-setup-secrets
  printf 'APP_PASS=%q\nMIG_PASS=%q\n' "$APP_PASS" "$MIG_PASS" > /root/.tiendas-setup-secrets
fi
set_role_password() {
  local role=$1 password=$2 exists
  exists=$(su postgres -c "psql -tAc \"SELECT 1 FROM pg_roles WHERE rolname='$role'\"")
  if [[ $exists == 1 ]]; then
    su postgres -c 'psql -v ON_ERROR_STOP=1' <<SQL
ALTER ROLE $role WITH LOGIN PASSWORD '$password';
SQL
  else
    su postgres -c 'psql -v ON_ERROR_STOP=1' <<SQL
CREATE ROLE $role LOGIN PASSWORD '$password';
SQL
  fi
}
set_role_password tiendas_migrator "$MIG_PASS"
set_role_password tiendas_app "$APP_PASS"
su postgres -c "psql -tAc \"SELECT 1 FROM pg_database WHERE datname='tiendas'\"" | grep -q 1 || su postgres -c 'createdb -O tiendas_migrator tiendas'
su postgres -c "psql -v ON_ERROR_STOP=1 -d tiendas -c 'CREATE EXTENSION IF NOT EXISTS citext'"
su postgres -c "psql -v ON_ERROR_STOP=1 -d tiendas -c 'REVOKE ALL ON DATABASE tiendas FROM PUBLIC; GRANT CONNECT ON DATABASE tiendas TO tiendas_migrator, tiendas_app; REVOKE ALL ON SCHEMA public FROM PUBLIC; ALTER SCHEMA public OWNER TO tiendas_migrator; GRANT USAGE ON SCHEMA public TO tiendas_app; ALTER DEFAULT PRIVILEGES FOR ROLE tiendas_migrator IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO tiendas_app; ALTER DEFAULT PRIVILEGES FOR ROLE tiendas_migrator IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO tiendas_app'"
install -d -m 0750 -o root -g tiendas /etc/tiendas
if [[ ! -f /etc/tiendas/pgpass ]]; then
  install -o root -g root -m 0600 /dev/null /etc/tiendas/pgpass
  printf '127.0.0.1:5432:tiendas:tiendas_migrator:%s\n' "$MIG_PASS" > /etc/tiendas/pgpass
fi

install -d -m 0750 -o root -g tiendas /etc/tiendas /var/lib/tiendas/backups
install -d -m 2775 -o deploy -g tiendas /opt/tiendas/releases
BASE_DOMAIN=CAMBIAR-DOMINIO.example
if [[ ! -f /etc/tiendas/env ]]; then
cat > /etc/tiendas/env <<EOF
# Completa valores de R2, BASE_DOMAIN y TOTP_KEY antes del primer arranque.
APP_ENV=prod
ADDR=127.0.0.1:8080
BASE_DOMAIN=$BASE_DOMAIN
DATABASE_URL=postgres://tiendas_app:$APP_PASS@127.0.0.1:5432/tiendas?sslmode=disable
MIGRATIONS_URL=postgres://tiendas_migrator@127.0.0.1:5432/tiendas?sslmode=disable
R2_ACCOUNT_ID=CAMBIAR_ACCOUNT_ID
R2_ACCESS_KEY_ID=CAMBIAR_ACCESS_KEY_ID
R2_SECRET_ACCESS_KEY=CAMBIAR_SECRET_ACCESS_KEY
R2_BUCKET=tiendas-prod
R2_PUBLIC_URL=https://CAMBIAR-DOMINIO-R2.example
TOTP_KEY=CAMBIAR_POR_64_CARACTERES_HEX
EOF
chown root:tiendas /etc/tiendas/env
chmod 0640 /etc/tiendas/env
fi
unset APP_PASS MIG_PASS RCLONE_PASS

# Token de Caddy separado: el daemon de proxy no necesita leer credenciales de la app/DB.
install -d -m 0750 -o root -g caddy /etc/caddy
if [[ ! -f /etc/caddy/caddy.env ]]; then
cat > /etc/caddy/caddy.env <<'EOF'
CF_API_TOKEN=CAMBIAR_TOKEN_CLOUDFLARE_DNS_EDIT
BASE_DOMAIN=CAMBIAR-DOMINIO.example
EOF
chown root:caddy /etc/caddy/caddy.env
chmod 0640 /etc/caddy/caddy.env
fi

# Credenciales del respaldo solo son legibles por root, no por el usuario de la app.
if [[ ! -f /etc/tiendas/backup.env ]]; then
cat > /etc/tiendas/backup.env <<EOF
PGPASSFILE=/etc/tiendas/pgpass
RCLONE_CONFIG_R2_TYPE=s3
RCLONE_CONFIG_R2_PROVIDER=Cloudflare
RCLONE_CONFIG_R2_ENV_AUTH=true
RCLONE_CONFIG_R2_ENDPOINT=https://CAMBIAR_ACCOUNT_ID.r2.cloudflarestorage.com
RCLONE_CONFIG_R2_ACCESS_KEY_ID=CAMBIAR_BACKUP_ACCESS_KEY
RCLONE_CONFIG_R2_SECRET_ACCESS_KEY=CAMBIAR_BACKUP_SECRET
RCLONE_CONFIG_R2_ACL=private
EOF
chown root:root /etc/tiendas/backup.env
chmod 0600 /etc/tiendas/backup.env
fi

# age guarda únicamente la pública en el VPS; generar la pareja fuera del servidor.
if [[ ! -e /etc/tiendas/backup.pub ]]; then install -o root -g tiendas -m 0640 /dev/null /etc/tiendas/backup.pub; fi

# Caddy oficial, luego se sustituye el binario por el build con cloudflare DNS.
curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/gpg.key | gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt -o /etc/apt/sources.list.d/caddy-stable.list
chmod o+r /usr/share/keyrings/caddy-stable-archive-keyring.gpg /etc/apt/sources.list.d/caddy-stable.list
apt-get update
apt-get install -y caddy
install -d -m 0755 /etc/systemd/system/caddy.service.d
cat > /etc/systemd/system/caddy.service.d/tiendas-env.conf <<'EOF'
[Service]
EnvironmentFile=/etc/caddy/caddy.env
EOF
systemctl daemon-reload

echo "Caddy: instala un build con Cloudflare DNS según docs/PRODUCCION.md. URL oficial: https://caddyserver.com/api/download?os=linux&arch=amd64&p=github.com/caddy-dns/cloudflare"
echo "Siguiente: edita /etc/tiendas/env, /etc/caddy/caddy.env y /etc/tiendas/backup.pub; instala la configuración de deploy/."
