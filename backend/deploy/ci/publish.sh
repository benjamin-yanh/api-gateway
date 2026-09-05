#!/usr/bin/env bash
# Run on the CI runner. SSH configuration is supplied by the workflow.
set -Eeuo pipefail
mode=${1:?deployment mode required}
release_id=${2:?release ID required}
[[ "$mode" =~ ^(full|frontend)$ ]]
[[ "$release_id" =~ ^[0-9]+-[0-9]+-[a-f0-9]{40}$ ]]
: "${CONTROL_HOST:?}" "${PUBLIC_URL:?}"
[[ "$CONTROL_HOST" =~ ^[a-zA-Z0-9.-]+$ ]]
[[ "$PUBLIC_URL" =~ ^https://[a-zA-Z0-9.-]+(:[0-9]+)?$ ]]
if [[ "$mode" == full ]]; then
  : "${RELAY_HOST:?}"
  [[ "$RELAY_HOST" =~ ^[a-zA-Z0-9.-]+$ ]]
fi
ssh_args=(-o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=4)
ssh_command=(ssh)
scp_command=(scp)
if [[ ! -s "$HOME/.ssh/id_ed25519" && -n "${SSHPASS:-}" ]]; then
  ssh_args[1]=BatchMode=no
  ssh_args+=(-o PreferredAuthentications=password,keyboard-interactive -o PubkeyAuthentication=no)
  ssh_command=(sshpass -e ssh)
  scp_command=(sshpass -e scp)
fi
remote="/opt/new-api/releases/$release_id"
# Upload all required files before restarting any service.
mkdir -p artifacts/control artifacts/relay
cp artifacts/frontend.tar.gz artifacts/control/
cp backend/deploy/ci/activate.sh artifacts/control/
if [[ "$mode" == full ]]; then
  cp artifacts/new-api-control artifacts/control/
  cp artifacts/new-api-relay backend/deploy/ci/activate.sh artifacts/relay/
fi
(cd artifacts/control && sha256sum frontend.tar.gz > SHA256SUMS)
if [[ "$mode" == full ]]; then
  (cd artifacts/control && sha256sum new-api-control >> SHA256SUMS)
  (cd artifacts/relay && sha256sum new-api-relay > SHA256SUMS)
fi
"${ssh_command[@]}" "${ssh_args[@]}" "root@$CONTROL_HOST" "mkdir -p '$remote'"
"${scp_command[@]}" "${ssh_args[@]}" artifacts/control/* "root@$CONTROL_HOST:$remote/"
if [[ "$mode" == full ]]; then
  "${ssh_command[@]}" "${ssh_args[@]}" "root@$RELAY_HOST" "mkdir -p '$remote'"
  "${scp_command[@]}" "${ssh_args[@]}" artifacts/relay/* "root@$RELAY_HOST:$remote/"
  "${ssh_command[@]}" "${ssh_args[@]}" "root@$CONTROL_HOST" "bash '$remote/activate.sh' control '$release_id'"
  "${ssh_command[@]}" "${ssh_args[@]}" "root@$RELAY_HOST" "bash '$remote/activate.sh' relay '$release_id'"
fi
"${ssh_command[@]}" "${ssh_args[@]}" "root@$CONTROL_HOST" "PUBLIC_URL='$PUBLIC_URL' bash '$remote/activate.sh' frontend '$release_id'"
curl -fLsS --max-time 20 "$PUBLIC_URL/healthz"
curl -fLsS --max-time 20 "$PUBLIC_URL/api/status" -o artifacts/live-status.json
printf 'Release %s deployed (%s)\n' "$release_id" "$mode" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
