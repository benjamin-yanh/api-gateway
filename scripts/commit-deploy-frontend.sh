#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "Usage: $0 -m <commit-message> -- <frontend-path> [frontend-path ...]"
  echo
  echo "Commits only the listed paths, builds that commit in an isolated worktree,"
  echo "then deploys the frontend to the production control-plane host."
}

commit_message=''
while (($# > 0)); do
  case "$1" in
    -m|--message)
      if (($# < 2)); then
        usage >&2
        exit 2
      fi
      commit_message=$2
      shift 2
      ;;
    --)
      shift
      break
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$commit_message" || $# -eq 0 ]]; then
  usage >&2
  exit 2
fi

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

if [[ -z "${SSH_PASSWORD:-}" ]]; then
  echo "SSH_PASSWORD is not set. Load the production environment first." >&2
  exit 1
fi

for command in git node shasum tar curl python3 /usr/bin/expect; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Required command is unavailable: $command" >&2
    exit 1
  fi
done

if [[ ! -x "$repo_root/frontend/node_modules/.bin/rsbuild" ]]; then
  echo "Frontend dependencies are unavailable. Install them before deploying." >&2
  exit 1
fi

selected_paths=("$@")
for path in "${selected_paths[@]}"; do
  case "$path" in
    frontend/*) ;;
    *)
      echo "Refusing non-frontend path: $path" >&2
      exit 2
      ;;
  esac

  if [[ ! -e "$path" ]] && ! git ls-files --error-unmatch -- "$path" >/dev/null 2>&1; then
    echo "Path does not exist and is not tracked: $path" >&2
    exit 2
  fi

  if ! git ls-files --error-unmatch -- "$path" >/dev/null 2>&1; then
    git add --intent-to-add -- "$path"
  fi
done

git diff --check -- "${selected_paths[@]}"
git commit --only -m "$commit_message" -- "${selected_paths[@]}"

commit_sha=$(git rev-parse HEAD)
control_host=${CONTROL_HOST:-101.132.177.78}
deploy_root=$(mktemp -d /tmp/api-gateway-frontend-deploy.XXXXXX)
build_tree="$deploy_root/source"
artifact="$deploy_root/frontend.tar.gz"
activation_script="$deploy_root/activate-frontend.sh"
worktree_added=false

cleanup() {
  if [[ "$worktree_added" == true ]]; then
    git worktree remove --force "$build_tree" >/dev/null 2>&1 || true
  fi
  rm -rf "$deploy_root"
}
trap cleanup EXIT

git worktree add --detach "$build_tree" "$commit_sha" >/dev/null
worktree_added=true
ln -s "$repo_root/frontend/node_modules" "$build_tree/frontend/node_modules"

(
  cd "$build_tree/frontend"
  export PATH="$PWD/node_modules/.bin:$PATH"

  node scripts/sync-i18n.mjs
  if ! git diff --exit-code -- src/i18n/locales >/dev/null; then
    echo "i18n synchronization changed locale files; commit the synchronized files first." >&2
    exit 1
  fi

  tsgo -b

  changed_frontend=()
  changed_typescript=()
  while IFS= read -r file; do
    relative_file=${file#frontend/}
    if [[ -e "$relative_file" ]]; then
      changed_frontend+=("$relative_file")
    fi
    case "$relative_file" in
      *.ts|*.tsx)
        changed_typescript+=("$relative_file")
        ;;
    esac
  done < <(
    git diff-tree --no-commit-id --name-only -r "$commit_sha" |
      grep -E '^frontend/' || true
  )
  if ((${#changed_typescript[@]} > 0)); then
    oxlint -c .oxlintrc.json "${changed_typescript[@]}"
  fi

  if ((${#changed_frontend[@]} > 0)); then
    node scripts/format-with-protected-headers.mjs --check "${changed_frontend[@]}"
  fi
  rsbuild build
)

COPYFILE_DISABLE=1 tar --no-xattrs -C "$build_tree/frontend/dist" -czf "$artifact" .
artifact_sha=$(shasum -a 256 "$artifact" | awk '{print $1}')

cat >"$activation_script" <<'REMOTE_SCRIPT'
#!/usr/bin/env bash
set -euo pipefail

expected_sha=$1
echo "$expected_sha  /tmp/frontend.tar.gz.new" | sha256sum -c -
tar -tzf /tmp/frontend.tar.gz.new >/dev/null

stamp=$(date +%Y%m%d%H%M%S)
web_backup="/opt/new-api/web.backup.$stamp"
web_new=$(mktemp -d /opt/new-api/web.new.XXXXXX)

tar -xzf /tmp/frontend.tar.gz.new -C "$web_new"
chmod 755 "$web_new"
find "$web_new" -type d -exec chmod 755 {} +

mv /opt/new-api/web "$web_backup"
mv "$web_new" /opt/new-api/web

if ! nginx -t || ! systemctl reload nginx; then
  mv /opt/new-api/web "/opt/new-api/web.failed.$stamp"
  mv "$web_backup" /opt/new-api/web
  nginx -t
  systemctl reload nginx
  exit 1
fi

systemctl is-active --quiet new-api-control
systemctl is-active --quiet nginx
echo "deployed_backup=$web_backup"
REMOTE_SCRIPT
chmod 755 "$activation_script"

export DEPLOY_PASSWORD=$SSH_PASSWORD
export DEPLOY_CONTROL_HOST=$control_host
export DEPLOY_ARTIFACT=$artifact
export DEPLOY_ACTIVATION_SCRIPT=$activation_script
export DEPLOY_ARTIFACT_SHA=$artifact_sha

/usr/bin/expect <<'EXPECT_UPLOAD'
set timeout 180
set password $env(DEPLOY_PASSWORD)
set host $env(DEPLOY_CONTROL_HOST)
set artifact $env(DEPLOY_ARTIFACT)
set activation $env(DEPLOY_ACTIVATION_SCRIPT)
spawn scp -o StrictHostKeyChecking=no $artifact $activation root@$host:/tmp/
expect "*assword:*"
send "$password\r"
expect eof
catch wait result
exit [lindex $result 3]
EXPECT_UPLOAD

/usr/bin/expect <<'EXPECT_ACTIVATE'
set timeout 180
set password $env(DEPLOY_PASSWORD)
set host $env(DEPLOY_CONTROL_HOST)
set artifact_sha $env(DEPLOY_ARTIFACT_SHA)
spawn ssh -o StrictHostKeyChecking=no root@$host "mv /tmp/frontend.tar.gz /tmp/frontend.tar.gz.new && mv /tmp/activate-frontend.sh /tmp/activate-frontend.sh.new && bash /tmp/activate-frontend.sh.new $artifact_sha"
expect "*assword:*"
send "$password\r"
expect eof
catch wait result
exit [lindex $result 3]
EXPECT_ACTIVATE

for scheme in http https; do
  base_url="$scheme://$control_host"
  curl_options=(-LfsS)
  if [[ "$scheme" == https ]]; then
    curl_options=(-kLfsS)
  fi

  health_role=$(
    curl "${curl_options[@]}" "$base_url/healthz" |
      python3 -c 'import json,sys; print(json.load(sys.stdin).get("role"))'
  )
  system_name=$(
    curl "${curl_options[@]}" "$base_url/api/status" |
      python3 -c 'import json,sys; print(json.load(sys.stdin).get("data", {}).get("system_name"))'
  )
  home_code=$(curl "${curl_options[@]}" -o /dev/null -w '%{http_code}' "$base_url/")
  models_code=$(curl "${curl_options[@]}" -o /dev/null -w '%{http_code}' "$base_url/v1/models")

  if [[ "$health_role" != web || "$system_name" != 'G同学' || "$home_code" != 200 || "$models_code" != 200 ]]; then
    echo "Production verification failed for $base_url" >&2
    exit 1
  fi

  echo "$base_url | system_name=$system_name | home=$home_code | /v1/models=$models_code"
done

echo "Committed and deployed $commit_sha"
