#!/usr/bin/env bash
set -euo pipefail

fail() { echo "ERROR: $*" >&2; exit 1; }
: "${DEPLOY_HOST:?ERROR: define DEPLOY_HOST (alias o usuario@host SSH)}"
: "${BASE_DOMAIN:?ERROR: define BASE_DOMAIN (por ejemplo tiendas.ejemplo.mx)}"
command -v git >/dev/null || fail "falta git."
command -v ssh >/dev/null || fail "falta ssh."
command -v scp >/dev/null || fail "falta scp."
command -v sha256sum >/dev/null || fail "falta sha256sum."
command -v tar >/dev/null || fail "falta tar."
git diff --quiet || fail "hay cambios sin commit; despliega una revisión confirmada."
git diff --cached --quiet || fail "hay cambios staged sin commit; despliega una revisión confirmada."
make build-prod
sha=$(git rev-parse --short=12 HEAD)
[[ $sha =~ ^[0-9a-f]{12}$ ]] || fail "no se pudo determinar el SHA de Git."
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/release/bin" "$work/release/migrations" "$work/release/deploy" "$work/release/web"
cp bin/server bin/goose bin/createadmin bin/createstore "$work/release/bin/"
cp migrations/*.sql "$work/release/migrations/"
# La app sirve /static desde web/static (CSS de Tailwind ya compilado por build-prod, JS y fuentes).
cp -R web/static "$work/release/web/static"
cp deploy/backup.sh "$work/release/deploy/backup.sh"
chmod 0755 "$work/release/bin/"* "$work/release/deploy/backup.sh"
(cd "$work/release" && find bin migrations deploy web -type f -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS)
tar -C "$work" -czf "$work/release-$sha.tar.gz" release
scp "$work/release-$sha.tar.gz" "$DEPLOY_HOST:/tmp/tiendas-$sha.tar.gz"

ssh "$DEPLOY_HOST" bash -s -- "$sha" "$BASE_DOMAIN" <<'REMOTE'
set -euo pipefail
sha=$1
base_domain=$2
root=/opt/tiendas
archive=/tmp/tiendas-$sha.tar.gz
release=$root/releases/$sha
staging=$root/releases/.staging-$sha
[[ $sha =~ ^[0-9a-f]{12}$ ]] || { echo 'ERROR: SHA inválido' >&2; exit 1; }
mkdir -p "$root/releases"
rm -rf "$staging"
mkdir "$staging"
tar -xzf "$archive" -C "$staging" --strip-components=1
(cd "$staging" && sha256sum --check --status SHA256SUMS) || { echo 'ERROR: checksum de release inválido' >&2; exit 1; }
chmod -R a+rX "$staging"
if [[ -e $release ]]; then rm -rf "$staging"; else mv "$staging" "$release"; fi
previous=$(readlink -f "$root/current" 2>/dev/null || true)
ln -sfn "$release" "$root/current.new"
mv -Tf "$root/current.new" "$root/current"
rollback() {
  if [[ -n $previous && -d $previous ]]; then ln -sfn "$previous" "$root/current.rollback"; mv -Tf "$root/current.rollback" "$root/current"; fi
  sudo -n /usr/bin/systemctl restart tiendas || true
  echo 'ERROR: deploy falló; se restauró el release anterior.' >&2
  exit 1
}
sudo -n /usr/bin/systemctl restart tiendas || rollback
for attempt in $(seq 1 20); do
  if curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null && curl -fsS --max-time 5 "https://$base_domain/healthz" >/dev/null; then exit 0; fi
  sleep 2
done
rollback
REMOTE
echo "Deploy $sha terminado; /healthz local y público responden correctamente."
