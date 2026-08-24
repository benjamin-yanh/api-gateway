# Agent development and deployment handoff

Last verified: 2026-08-23 (Asia/Shanghai)

This document is the operational handoff for the customized `api-gateway`
installation. Read it together with `AGENTS.md` before modifying or deploying the
project. It intentionally contains no passwords or complete database credentials.

## 1. Current production topology

| Component | Host | Service or path |
| --- | --- | --- |
| Nginx and static frontend | `101.132.177.78` | `nginx`, `/opt/new-api/web` |
| Control plane | `101.132.177.78` | `new-api-control`, `/opt/new-api/bin/new-api-control` |
| Data plane | `106.54.222.228` | `new-api-relay`, `/opt/new-api/bin/new-api-relay` |
| Shared database | External MySQL | Configuration is loaded from local environment variables |

The control plane owns database migrations, authentication, dashboard APIs, and
scheduled jobs. The data plane owns model discovery and relay traffic. Both planes
must use the same database, Redis, session secret, and crypto secret.

The control-plane systemd unit binds to `127.0.0.1:3001`. Deployment readiness
checks on the control host must poll `http://127.0.0.1:3001/api/status`; polling
port `3000` incorrectly times out and triggers rollback even when the new binary
started successfully.

Production is reachable through both HTTP and HTTPS on `101.132.177.78`. HTTP
redirects to HTTPS, so content validation must follow redirects. HTTPS validation
by IP may require `curl -k` because certificate hostname validation and IP
certificates are separate concerns.

### Local environment variables

Before reading deployment configuration, run:

```bash
source ~/.zshrc
```

The following variables are expected:

- `SSH_PASSWORD`: SSH password for the `root` account on both hosts.
- `SQL_PUB_JDBC_CONNECTION`: external MySQL connection address.
- `SQL_PUB_JDBC_USERNAME`: external MySQL username.
- `SQL_PUB_JDBC_DATABASE`: external MySQL database name.
- `SQL_PUB_JDBC_PASSWORD`: external MySQL password.

Never print these values, place them in source files, commit them, or include them
in an Agent response. The workstation has `/usr/bin/expect`; `sshpass` is not
installed.

## 2. Customized product behavior

### Brand and public UI

- The default site name is **G同学**.
- The backend default is in `backend/common/constants.go`.
- The frontend fallback is in `frontend/src/lib/constants.ts`.
- The static page title and title metadata are in `frontend/index.html`.
- `backend/model/frontend_option_migration.go` migrates the former built-in names
  `New API` and `纪同学` to `G同学`, while preserving administrator-defined custom
  names.
- The browser and Apple touch icon use `frontend/public/app-icon.png`.
- The home-page hero has exactly two primary actions: obtain an API key and open
  `/docs#desktop-clients`. Individual macOS downloads remain in the documentation
  section and are configured in `frontend/src/features/home/constants.ts`.
- The home page does not show provider/model/interface/governance count statistics.
- Every route displays a bottom-right contact card with customer-service QQ and
  the `https://t.me/gtongxue` Telegram group.
- The home-page hero does not show the `Enterprise AI Solutions` badge, and the
  default footer does not show the `Powerful API Management Platform` tagline.
- Home-page and documentation content has been reduced to the product's supported
  models, protocols, routing, and client usage. Do not restore upstream promotional
  or GitHub links without checking the customization requirements.

### Currency and pricing

- User-facing quota and pricing are configured for RMB/CNY rather than USD.
- The public `/pricing` route is an API quotation page with model-category
  filters and input/cache/output price columns. The former model-square card,
  filter-sidebar, drawer, and model-detail UI are intentionally hidden.
- The rankings navigation entry and `/rankings` public page are intentionally
  hidden; direct visits redirect to the home page.
- `backend/setting/operation_setting/general_setting.go` uses the CNY quota display
  type by default.
- Pricing editors must not expose floating-point artifacts such as
  `￥69.999999999997`; preserve the existing decimal formatting behavior when
  changing pricing code.

### Recharge center and redemption cards

- The authenticated user sidebar exposes `/wallet` as the recharge center and
  links card purchases to `https://www.kufaka.com/shop/GBCRMEYE`.
- Redemption cards are stored in `redemption_cards`. Supported groups are
  `3_RMB_CARD`, `10_RMB_CARD`, `50_RMB_CARD`, `100_RMB_CARD`, and
  `200_RMB_CARD`.
- A card redemption locks the card row and performs a compare-and-swap status
  update before crediting quota in the same database transaction. A card must
  never credit an account more than once, including concurrent retries.
- `GET /api/user/redemption-card/history` returns only the signed-in user's ten
  most recent successful card redemptions and never returns the card secret.
- The frontend language selector and bundled translations support only English
  and Simplified Chinese.

### Login, setup, and local application authorization

- New frontend password login uses `email` instead of `username`.
- The initial root account is created with its email as both `username` and
  `email`.
- Email/username storage supports up to 128 characters and validation errors must
  be friendly messages, not raw validator messages such as `failed on the 'max'
  tag`.
- `username` remains a deprecated compatibility alias for older clients.
- Local/native application OAuth uses authorization code + PKCE and returns login
  session credentials to the local application after browser authorization.
- Anthropic/Claude-compatible control-plane endpoints include:

  | Method | Endpoint | Purpose |
  | --- | --- | --- |
  | `POST` | `/auth/login` | Compatibility password login |
  | `POST` | `/anthropic/auth/login` | Same compatibility password login |
  | `GET` | `/anthropic/oauth/authorize` | Browser authorization entry |
  | `GET` | `/anthropic/oauth/code/callback` | Authorization callback |
  | `POST` | `/anthropic/v1/oauth/token` | Exchange or refresh OAuth credentials |
  | `POST` | `/anthropic/api/oauth/claude_cli/create_api_key` | Create a model API key for the signed-in user |
  | `GET` | `/anthropic/api/oauth/claude_cli/roles` | Return compatible roles |
  | `GET` | `/anthropic/api/oauth/profile` | Return current profile |

- Password login returns the current user's `access_token` and API keys, but only
  keys whose status is enabled. Disabled keys must never be returned.
- Password login also returns a `models` array filtered for the detected client:
  `/anthropic/*`, Claude/Anthropic client identifiers return Claude-family
  models; Codex, ChatGPT, and OpenAI identifiers return OpenAI-family models;
  unidentified clients receive all enabled models. Detection accepts the JSON
  `client` field, `Originator`, `X-Client-Name`, `X-Client`, `X-App`, and
  `User-Agent`; it is a display filter, not an authorization boundary.
- Accounts protected by 2FA must use the normal login flow; compatibility password
  login must not bypass the second factor.
- Password-login endpoints use a dedicated shared rate-limit bucket. A 429 response
  is JSON with `success`, `code`, and `message`, plus `Retry-After` when available.

### Data-plane model APIs

- `GET /v1/models` remains public when `Authorization` is absent. When an
  `Authorization` API key is supplied, it is validated and the response is
  restricted by that token's group and model limits. Invalid supplied keys
  receive the normal authentication error.
- `GET /v1/models` filters its response with the same client detection used by
  password login: Claude/Anthropic clients receive Claude-family models; Codex,
  ChatGPT, and OpenAI clients receive OpenAI-family models; unidentified clients
  receive all models available to their group and token. `client` may be declared
  as a query parameter in addition to the supported client headers. Anthropic
  protocol requests and `/anthropic/v1/models` always receive Claude-family models.
- Anthropic-prefixed data-plane endpoints are:

  | Method | Endpoint | Authentication |
  | --- | --- | --- |
  | `GET` | `/anthropic/v1/models` | Public |
  | `GET` | `/anthropic/v1/models/:model` | Public |
  | `POST` | `/anthropic/v1/messages` | Model API key required |

- The `/anthropic` prefix is removed internally after Gin route matching so the
  existing Anthropic validation, distribution, billing, and relay implementation is
  reused.
- `grok-imagine-image` is an image model and is supported only by
  `/v1/images/generations` and `/v1/images/edits`, not text-generation endpoints.

### Internal documentation page

- The default docs link is `/docs`.
- The frontend documentation implementation is under
  `frontend/src/features/docs/` and `frontend/src/routes/docs/`.
- It shows the public model catalogue from `/v1/models`, protocol endpoints, and
  current routing behavior.
- Footer and top navigation links point to the internal documentation sections.

### Data-plane access logs

- Access logging is installed only when the executable has a data-plane role, and
  only requests tagged `relay` are persisted. Control-plane, authentication,
  dashboard, and static traffic must not be written to this table.
- Each log can contain URL, request headers, JSON request body, status, latency,
  response size, IP, node name, and supported response content.
- Request bodies are stored only for JSON media types and are capped at 256 KiB.
  Non-JSON request bodies are ignored.
- JSON, `+json`, NDJSON, and SSE response content is captured up to 1 MiB.
- A streaming response is collected into one access-log record without delaying
  or disabling response flushing.
- Stored headers, sensitive URL query parameters, and common credential fields
  in JSON/NDJSON/SSE bodies are recursively redacted before persistence. Access
  logs can still contain prompts and model output, so treat administrator access
  to them as highly sensitive.
- The administrator UI route is `/access-logs`, located below task logs in the
  sidebar. Its detail dialog shows request headers, request body, and response body
  with JSON syntax highlighting.
- Backend list and detail APIs are `GET /api/access-log/` and
  `GET /api/access-log/:id`; they belong to the control plane and require an admin.

Relevant implementation files:

- `backend/middleware/access_log.go`
- `backend/model/access_log.go`
- `backend/controller/access_log.go`
- `frontend/src/features/access-logs/`

## 3. Working tree safety

This checkout may contain many accumulated, uncommitted user changes. Before doing
anything, inspect:

```bash
cd /Users/benjamin/Documents/github/api-gateway
git status --short
git diff --stat
git diff
```

Do not use `git reset --hard`, `git checkout --`, or overwrite unrelated changes.
Do not manually edit generated files in `frontend/dist` or
`backend/frontend/dist`; rebuild the frontend instead. Only commit when the user
asks for a commit.

## 4. Local verification

### Backend

Format every modified Go file, then run focused tests:

```bash
cd /Users/benjamin/Documents/github/api-gateway/backend
gofmt -w path/to/changed.go path/to/changed_test.go
go test ./model ./common
```

Select additional packages matching the change, for example:

```bash
go test ./controller ./middleware ./router
```

Run the full backend suite when practical:

```bash
go test ./...
```

If `backend/relaykit/` or its public API changes, its independent build is
mandatory:

```bash
cd /Users/benjamin/Documents/github/api-gateway/backend/relaykit
GOWORK=off go build ./...
```

### Frontend

Use Bun when it is available. In the current desktop shell, the already-installed
Rsbuild binary can be invoked directly if `bun` is not on `PATH`:

```bash
cd /Users/benjamin/Documents/github/api-gateway/frontend
./node_modules/.bin/rsbuild build
```

Run focused frontend tests relevant to the modified feature as well. Any new or
changed user-visible text must be synchronized across both locale files under
`frontend/src/i18n/locales/` according to the i18n project instructions.

## 5. Production build

Create deployment artifacts outside the repository:

```bash
mkdir -p /tmp/api-gateway-deploy

cd /Users/benjamin/Documents/github/api-gateway/backend

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -tags split \
  -ldflags '-s -w -X main.DefaultServerRole=control -X main.LockedServerRole=control' \
  -o /tmp/api-gateway-deploy/new-api-control .

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -tags split \
  -ldflags '-s -w -X main.DefaultServerRole=relay -X main.LockedServerRole=relay' \
  -o /tmp/api-gateway-deploy/new-api-relay .

COPYFILE_DISABLE=1 tar --no-xattrs \
  -C /Users/benjamin/Documents/github/api-gateway/frontend/dist \
  -czf /tmp/api-gateway-deploy/frontend.tar.gz .

ls -lh /tmp/api-gateway-deploy
```

The locked role is a security boundary. Do not ship an unlocked all-in-one binary
to either split production host.

## 6. Upload artifacts

Load the SSH password without displaying it:

```bash
source ~/.zshrc
export DEPLOY_PASSWORD="$SSH_PASSWORD"
```

Use `/usr/bin/expect` to supply it to `scp`. Example for one artifact:

```bash
/usr/bin/expect -c '
  set timeout 180
  set password $env(DEPLOY_PASSWORD)
  spawn scp -o StrictHostKeyChecking=no \
    /tmp/api-gateway-deploy/new-api-control \
    root@101.132.177.78:/tmp/new-api-control.new
  expect "*assword:*"
  send "$password\r"
  expect eof
  catch wait result
  exit [lindex $result 3]
'
```

Upload these exact mappings:

| Local artifact | Remote destination |
| --- | --- |
| `/tmp/api-gateway-deploy/new-api-control` | `root@101.132.177.78:/tmp/new-api-control.new` |
| `/tmp/api-gateway-deploy/frontend.tar.gz` | `root@101.132.177.78:/tmp/frontend.tar.gz.new` |
| `/tmp/api-gateway-deploy/new-api-relay` | `root@106.54.222.228:/tmp/new-api-relay.new` |

Run uploads separately so a failed transfer is obvious. Check each `expect` exit
code; seeing only a password prompt does not prove the transfer completed.

## 7. Activate with rollback

Deploy the control plane first so its startup migrations finish before a new
data-plane binary writes newly added columns. Keep the old data plane serving
traffic while the control-plane migration runs, then activate the data plane.

### Control plane and frontend (`101.132.177.78`)

Over SSH, execute the equivalent of:

```bash
set -e
stamp=$(date +%Y%m%d%H%M%S)
binary_backup=/opt/new-api/bin/new-api-control.backup.$stamp
web_backup=/opt/new-api/web.backup.$stamp
web_new=$(mktemp -d /opt/new-api/web.new.XXXXXX)

tar -xzf /tmp/frontend.tar.gz.new -C "$web_new"
chmod 755 "$web_new"
find "$web_new" -type d -exec chmod 755 {} +

cp /opt/new-api/bin/new-api-control "$binary_backup"
install -m 0755 /tmp/new-api-control.new /opt/new-api/bin/new-api-control
mv /opt/new-api/web "$web_backup"
mv "$web_new" /opt/new-api/web

if ! systemctl restart new-api-control || \
   ! systemctl is-active --quiet new-api-control; then
  mv /opt/new-api/web "/opt/new-api/web.failed.$stamp"
  mv "$web_backup" /opt/new-api/web
  install -m 0755 "$binary_backup" /opt/new-api/bin/new-api-control
  systemctl restart new-api-control
  exit 1
fi

nginx -t
systemctl reload nginx
```

Wait until `/api/status` is ready before activating the data plane.

### Data plane (`106.54.222.228`)

Over SSH, execute the equivalent of:

```bash
set -e
stamp=$(date +%Y%m%d%H%M%S)
backup=/opt/new-api/bin/new-api-relay.backup.$stamp

cp /opt/new-api/bin/new-api-relay "$backup"
install -m 0755 /tmp/new-api-relay.new /opt/new-api/bin/new-api-relay

if ! systemctl restart new-api-relay || \
   ! systemctl is-active --quiet new-api-relay; then
  install -m 0755 "$backup" /opt/new-api/bin/new-api-relay
  systemctl restart new-api-relay
  exit 1
fi
```

`mktemp -d` creates a directory with mode `0700`. The explicit `chmod 755` is
mandatory; otherwise Nginx returns HTTP 403 for the home page even while backend
APIs continue to work.

The control plane can take approximately 10–20 seconds to become ready while it
runs MySQL migrations. `systemctl is-active` alone is not sufficient readiness
validation; wait for the ready log or poll `/api/status`.

Useful diagnostics:

```bash
systemctl status new-api-control --no-pager -l
journalctl -u new-api-control -n 100 --no-pager
systemctl status new-api-relay --no-pager -l
journalctl -u new-api-relay -n 100 --no-pager
ss -lntp
```

## 8. Post-deployment verification

Verify both schemes:

```bash
curl -kLfsS http://101.132.177.78/healthz
curl -kLfsS http://101.132.177.78/api/status
curl -kLfsS http://101.132.177.78/v1/models

curl -kLfsS https://101.132.177.78/healthz
curl -kLfsS https://101.132.177.78/api/status
curl -kLfsS https://101.132.177.78/v1/models
```

Required results:

- `/healthz` succeeds.
- `/api/status` reports `system_name` as `G同学`.
- Direct HTTP requests redirect to HTTPS, and the final HTML `<title>` is `G同学`.
- `/v1/models` returns HTTP 200 without `x-api-key`.
- `new-api-control`, `new-api-relay`, and `nginx` are active.
- A data-plane model request creates one access-log record; control-plane requests
  do not.

Example compact validation:

```bash
for base_url in http://101.132.177.78 https://101.132.177.78; do
  system_name=$(curl -kLfsS "$base_url/api/status" |
    python3 -c 'import json,sys; print(json.load(sys.stdin).get("data", {}).get("system_name"))')
  page_title=$(curl -kLfsS "$base_url/" |
    sed -n 's:.*<title>\([^<]*\)</title>.*:\1:p' | head -1)
  models_code=$(curl -kLso /dev/null -w '%{http_code}' "$base_url/v1/models")
  printf '%s | system_name=%s | title=%s | /v1/models=%s\n' \
    "$base_url" "$system_name" "$page_title" "$models_code"
done
```

Last observed production result on 2026-08-23 (after following redirects):

```text
http://101.132.177.78  | system_name=G同学 | title=G同学 | /v1/models=200
https://101.132.177.78 | system_name=G同学 | title=G同学 | /v1/models=200
```

## 9. Update this handoff

When topology, credentials variable names, service names, paths, route ownership,
or deployment steps change, update this file in the same change. Do not place live
secrets in the document.
