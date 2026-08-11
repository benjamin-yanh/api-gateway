# Control-plane and data-plane deployment

The split deployment produces three images:

- `new-api-web`: static frontend and reverse proxy.
- `new-api-control`: management, authentication, billing, and dashboard APIs.
- `new-api-relay`: model relay and media proxy APIs.

The relay executable is built with `LockedServerRole=relay`. Setting
`SERVER_ROLE=control` at runtime is rejected, so a relay image cannot expose
control-plane routes through configuration.

## Route ownership

| Route family | Owner |
| --- | --- |
| `/api/*`, `/dashboard/*`, `/v1/dashboard/*` | Control plane |
| `/anthropic/oauth/*`, `/anthropic/api/*`, `/anthropic/v1/oauth/*` | Control plane |
| `/v1/*`, `/v1beta/*`, `/mj/*`, `/:mode/mj/*` | Data plane |
| `/anthropic/v1/models*`, `/anthropic/v1/messages` | Data plane |
| `/suno/*`, `/kling/*`, `/jimeng/*`, `/pg/*` | Data plane |
| `/` and static assets | Web |

Both backend services expose `/healthz`. The response contains the locked
server role.

## Build images

```bash
docker build -f Dockerfile.backend --build-arg SERVER_ROLE=control -t new-api-control:local .
docker build -f Dockerfile.backend --build-arg SERVER_ROLE=relay -t new-api-relay:local .
docker build -f Dockerfile.web -t new-api-web:local .
```

The backend Dockerfile uses the `split` build tag, so it does not build or
embed the frontend bundle.

## Start with Docker Compose

SQLite and Redis are the default. The control and relay containers mount the
same `app_data` volume so they see the same SQLite database.

```bash
cp backend/deploy/.env.split.example .env.split
# Set REDIS_PASSWORD, SESSION_SECRET, and CRYPTO_SECRET.
# Leave SQL_DSN empty for SQLite.
docker compose --env-file .env.split -f docker-compose.split.yml up -d --build
```

To use PostgreSQL, set `POSTGRES_PASSWORD` and the PostgreSQL `SQL_DSN` shown
in `backend/deploy/.env.split.example`, then enable its profile:

```bash
docker compose --env-file .env.split -f docker-compose.split.yml \
  --profile postgres up -d --build
```

To use MySQL, set `MYSQL_ROOT_PASSWORD`, `MYSQL_PASSWORD`, and the MySQL
`SQL_DSN` shown in `backend/deploy/.env.split.example`, then enable its profile:

```bash
docker compose --env-file .env.split -f docker-compose.split.yml \
  --profile mysql up -d --build
```

## Start with local Kubernetes

The default manifest uses SQLite on the shared `app-data` volume and Redis:

```bash
kubectl create namespace new-api-local --dry-run=client -o yaml | kubectl apply -f -
kubectl -n new-api-local create secret generic new-api-secrets \
  --from-literal=redis-password='replace-with-a-strong-password' \
  --from-literal=redis-connection-string='redis://:replace-with-a-strong-password@redis:6379' \
  --from-literal=session-secret='replace-with-a-long-random-secret' \
  --from-literal=crypto-secret='replace-with-a-long-random-secret' \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f backend/deploy/kubernetes/local.yaml
```

The application secret `new-api-secrets` must contain
`redis-password`, `redis-connection-string`, `session-secret`, and
`crypto-secret`. When `new-api-database` is absent, `SQL_DSN` is not set and
the application selects SQLite.

For PostgreSQL, create the optional database secret and apply the PostgreSQL
manifest before restarting the backends:

```bash
kubectl -n new-api-local create secret generic new-api-database \
  --from-literal=POSTGRES_PASSWORD='replace-with-a-strong-password' \
  --from-literal=SQL_DSN='postgresql://newapi:replace-with-a-strong-password@postgres:5432/new-api' \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f backend/deploy/kubernetes/postgres.yaml
kubectl -n new-api-local rollout status statefulset/postgres
kubectl -n new-api-local rollout restart deployment/admin deployment/relay
```

For MySQL:

```bash
kubectl -n new-api-local create secret generic new-api-database \
  --from-literal=MYSQL_ROOT_PASSWORD='replace-with-a-strong-root-password' \
  --from-literal=MYSQL_PASSWORD='replace-with-a-strong-password' \
  --from-literal=SQL_DSN='newapi:replace-with-a-strong-password@tcp(mysql:3306)/new-api?charset=utf8mb4&parseTime=true&loc=Local' \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f backend/deploy/kubernetes/mysql.yaml
kubectl -n new-api-local rollout status statefulset/mysql
kubectl -n new-api-local rollout restart deployment/admin deployment/relay
```

Start the control plane before adding relay replicas. The control plane owns
database migrations and scheduled maintenance. Relay nodes force
`NODE_TYPE=slave`, never run control-plane jobs, and can be scaled
horizontally after the schema is ready.

## Operational notes

- Redis must be shared by both backend roles.
- SQLite is the zero-configuration default for local or single-node use. Both
  backends must mount the same database volume.
- Use MySQL or PostgreSQL for production multi-node deployments. A SQLite file
  on network storage is not a safe distributed database.
- Keep `SESSION_SECRET` and `CRYPTO_SECRET` identical across all containers.
- The edge proxy disables response buffering for SSE and WebSocket routes.
- Do not enable automatic retry at the edge; relay billing and channel retry
  must remain coordinated inside the data plane.
- Only the web container publishes a host port in the example Compose file.
