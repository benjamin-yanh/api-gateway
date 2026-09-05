#!/usr/bin/env bash
# Run on the destination host after uploading the release directory.
set -Eeuo pipefail
role=${1:?role required}
release_id=${2:?release ID required}
[[ "$role" =~ ^(control|relay|frontend)$ ]]
[[ "$release_id" =~ ^[0-9]+-[0-9]+-[a-f0-9]{40}$ ]]
root=${DEPLOY_ROOT:-/opt/new-api}
release="$root/releases/$release_id"
mkdir -p "$release/backups"
exec 9>"$root/deploy.lock"
flock -n 9 || { echo 'Another deployment is active'; exit 1; }
cd "$release"
# Each host receives only the artifacts that it needs.
sha256sum -c SHA256SUMS
backup="$release/backups/$role"
[[ ! -e "$backup" ]] || { echo 'Release phase already attempted; use a new run'; exit 1; }
changed=0
rollback() {
  trap - ERR INT TERM HUP
  if [[ "$changed" == 1 ]]; then
    if [[ "$role" == frontend ]]; then
      [[ ! -d "$root/web" ]] || mv "$root/web" "$release/web.failed"
      mv "$backup" "$root/web"
    else
      install -m 0755 "$backup" "$root/bin/new-api-$role.rollback"
      mv -f "$root/bin/new-api-$role.rollback" "$root/bin/new-api-$role"
      systemctl restart "new-api-$role" || true
    fi
    echo "ROLLED_BACK role=$role backup=$backup (check service readiness)"
  fi
  exit 1
}
trap rollback ERR INT TERM HUP
if [[ "$role" == frontend ]]; then
  nginx -t
  mkdir "$release/web.staged"
  tar -xzf frontend.tar.gz -C "$release/web.staged"
  test -s "$release/web.staged/index.html"
  chmod 755 "$release/web.staged"
  find "$release/web.staged" -type d -exec chmod 755 {} +
  find "$release/web.staged" -type f -exec chmod 644 {} +
  mv "$root/web" "$backup"
  changed=1
  mv "$release/web.staged" "$root/web"
  # Nginx serves static files directly; no backend restart is needed.
  curl -fsS --max-time 20 "${PUBLIC_URL:?public HTTPS URL required}/pricing" -o "$release/served-index.html"
  cmp "$root/web/index.html" "$release/served-index.html"
else
  cp -p "$root/bin/new-api-$role" "$backup"
  install -m 0755 "new-api-$role" "$root/bin/new-api-$role.staged"
  changed=1
  mv -f "$root/bin/new-api-$role.staged" "$root/bin/new-api-$role"
  systemctl restart "new-api-$role"
  port=3001
  [[ "$role" != relay ]] || port=3002
  deadline=$((SECONDS + 900))
  until curl -fsS --max-time 3 "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; do
    (( SECONDS < deadline )) || { echo 'Readiness timeout'; false; }
    systemctl is-active --quiet "new-api-$role"
    echo "Waiting for $role readiness..."
    sleep 10
  done
  systemctl is-active --quiet "new-api-$role"
fi
trap - ERR INT TERM HUP
printf 'DEPLOYED role=%s release=%s backup=%s\n' "$role" "$release_id" "$backup"
