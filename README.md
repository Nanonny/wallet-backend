# Cloud-Native Wallet Transaction Platform

DevOps / Platform Engineering portfolio project using:

- Minikube
- Kubernetes YAML
- Golang
- gRPC
- Hexagonal Architecture
- PostgreSQL (user accounts)
- Redis
- JWT authentication (bcrypt-hashed credentials, bearer tokens)
- Envoy Load Balancer
- Prometheus
- Grafana
- hey load testing

> Note: The service includes `api/proto/wallet.proto`. To make the scaffold runnable immediately without requiring generated protobuf files, the included gRPC adapter uses grpc-go with a JSON codec and mirrors the proto messages. For production, generate normal protobuf stubs with `make proto` and replace the adapter with generated `walletpb` handlers.

> Security: every `/v1/wallets/*` and `/v1/simulate/*` endpoint (HTTP **and** gRPC) now requires a bearer token obtained from `/v1/auth/login`. Credentials are stored as bcrypt hashes in PostgreSQL, sessions are HS256 JWTs, failed logins are rate-limited (10 per 15 min per IP+user), and wallets are owned by their account (an outsider's wallet returns 404, never 403/leaked data). Full report: [`docs/security-audit.md`](docs/security-audit.md).

---

## Architecture Diagram

```mermaid
flowchart TD
    B[Browser / hey load test] --> L[Login page :8080/login<br/>register + login]
    L --> JWT[/v1/auth/login<br/>bcrypt verify + issue JWT]
    B --> E[Envoy Proxy<br/>HTTP :8080<br/>gRPC :50051]
    E --> P1[wallet-api pod 1<br/>authn middleware<br/>+ ownership check]
    E --> P2[wallet-api pod 2<br/>Go HTTP + gRPC]
    E --> P3[wallet-api pod 3<br/>Go HTTP + gRPC]
    P1 --> R[(Redis<br/>wallet state<br/>transactions<br/>idempotency)]
    P2 --> R
    P3 --> R
    P1 --> PG[(PostgreSQL :5432<br/>users<br/>bcrypt password hashes)]
    P2 --> PG
    P3 --> PG
    P1 --> M[/metrics :9090]
    P2 --> M
    P3 --> M
    M --> PR[Prometheus]
    PR --> G[Grafana Dashboard]
```

---

## Project Structure

```text
wallet-devops-lab/
├── api/proto/wallet.proto
├── cmd/
│   ├── wallet-api/main.go
│   └── grpc-client/main.go
├── internal/
│   ├── domain/                  # Wallet, Transaction, User, errors
│   ├── application/             # services, ports, subject context, token service
│   ├── adapters/
│   │   ├── http/                # wallet API + /v1/auth/* + static login UI
│   │   ├── grpc/                # gRPC handlers + auth interceptor
│   │   ├── redis/               # wallet state
│   │   ├── memory/              # in-process wallet + user repositories
│   │   ├── postgres/            # user accounts (accounts live here)
│   │   ├── credential/          # bcrypt hashing + timing-equalized dummy
│   │   └── metrics/
│   └── config/
├── deploy/k8s/
│   ├── 00-namespace.yaml
│   ├── 00-secrets.yaml
│   ├── 01-redis.yaml
│   ├── 01b-postgres.yaml       # PostgreSQL Deployment + PVC + Service + PDB
│   ├── 02-wallet-api.yaml
│   ├── 03-envoy-config.yaml
│   ├── 04-envoy.yaml
│   ├── 05-prometheus.yaml
│   ├── 06-grafana.yaml
│   ├── 07-hpa.yaml
│   ├── 08-pdb.yaml
│   ├── 09-networkpolicy.yaml
│   └── kustomization.yaml
├── deploy/argocd/
│   └── apps/wallet-api.yaml
├── docs/
│   ├── security-audit.md       # full security review + hardening notes
│   ├── ci-cd-dockerhub.md
│   ├── self-hosted-runner.md
│   └── argocd-cd.md
├── loadtest/
├── docker-compose.yml          # local Redis + PostgreSQL
├── Dockerfile
├── Makefile
└── README.md
```

---

## Hexagonal Architecture

```text
HTTP Handler / gRPC Handler
        ↓
Application Service
        ↓
WalletRepository Port Interface
        ↓
Redis Adapter / Memory Adapter
        ↓
Redis or in-memory map
```

The domain and application layers do not depend on Kubernetes, Redis, HTTP, or gRPC.

---

## Local Run Without Redis

Use this first to verify the Go code quickly.

```bash
go mod tidy
make run-memory
```

Test HTTP API (all `/v1/*` calls need a bearer token now):

```bash
# 1) create an account
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"wallet-demo-1234"}'

# 2) log in and keep the token
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"wallet-demo-1234"}' | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')

# 3) use it on protected routes
curl -X POST http://localhost:8080/v1/simulate/seed \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"users":10,"initial_balance_cents":100000}'

curl -X POST http://localhost:8080/v1/simulate/transfer \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"users":10,"amount_cents":100}'

curl http://localhost:8080/readyz
curl http://localhost:9090/metrics
```

Open `http://localhost:8080` for the built-in Wallet Console. It now opens at
the login page (`/login`); after signing in it can create wallets, check
balances, deposit, withdraw, and transfer without a separate frontend
deployment.

Test gRPC local client:

```bash
make grpc-client
```

---

## Local Run With Redis + PostgreSQL

The full stack (Redis for wallet state, PostgreSQL for user accounts) runs
with one command:

```bash
make deps-up        # docker compose up -d  (redis:6379 + postgres:5432)
make run
```

Or manually:

```bash
docker compose up -d
go mod tidy
make run
```

---

## Authentication (PostgreSQL + JWT)

All wallet and simulation endpoints are protected. The flow:

```text
GET  /login                      → minimal login UI (register / sign in tabs)
POST /v1/auth/register           → {username, password}     → 201 user
POST /v1/auth/login              → {username, password}     → 200 {token, user}
POST /v1/auth/logout             → no-op (stateless JWTs)   → 204
GET  /v1/auth/me                 → Bearer token             → 200 {username}
```

- **Storage:** users live in PostgreSQL (`users` table), passwords as **bcrypt** (cost 12). Only the hash reaches the database — never plaintext.
- **Sessions:** HS256 JWT (`iss=wallet-api`, subject = username, 8h TTL), signed with `JWT_SECRET`. Send it as `Authorization: Bearer <token>`.
- **Password policy:** 8–128 characters, at least one letter and one number.
- **Usernames:** 3–32 characters, lowercase letters, numbers, or `_` (lookups are case-insensitive).
- **Brute-force protection:** failed logins are rate-limited to 10 per 15 minutes per IP+username (HTTP 429 + `Retry-After`); unknown usernames still burn a bcrypt comparison (timing-leak guard).
- **Ownership:** wallets belong to the account that created them. Another user's wallet returns `404` (no existence leak). Transfers **to** a foreign wallet are allowed (that is paying someone); transfers **from** one are not.
- **gRPC:** the same tokens are accepted via the `authorization` (or `grpc-token`) metadata; missing/invalid → `UNAUTHENTICATED`.
- **Metrics:** `wallet_auth_events_total{event="login|register|api_request",result=...}` is available in Prometheus/Grafana.

Local demo helper (registers/logs in a `demo` user and prints its token):

```bash
make login
```

Environment variables (`internal/config/config.go`):

| Variable     | Default    | Purpose                            |
| ------------ | ---------- | ---------------------------------- |
| `JWT_SECRET` | _(empty)*  | 16+ character token signing key    |
| `PGHOST`     | `localhost`| PostgreSQL host                    |
| `PGPORT`     | `5432`     | PostgreSQL port                    |
| `PGUSER`     | `wallet`   | PostgreSQL user                    |
| `PGPASSWORD` | `wallet`   | PostgreSQL password                |
| `PGDATABASE` | `wallet`   | PostgreSQL database                |
| `PGSSLMODE`  | `disable`  | TLS mode for the PG connection     |
| `PG_DSN`     | _(empty)_  | Full DSN override (wins over PG_*) |

\* With no `JWT_SECRET`, the server generates an ephemeral one and logs a big warning (sessions die with the process and multi-replica deployments disagree — always set it for real deployments; `deploy/k8s/00-secrets.yaml` shows the key).

Run the Postgres integration test against the local compose database:

```bash
make deps-up
make test-pg
```

---

## Minikube Setup for MacBook Air M4

For 8GB RAM:

```bash
minikube start --driver=docker --cpus=3 --memory=4096 --disk-size=25g
```

For 16GB RAM or more:

```bash
minikube start --driver=docker --cpus=4 --memory=6144 --disk-size=30g
```

Build the image inside Minikube Docker:

```bash
eval $(minikube docker-env)
docker build -t wallet-api:dev .
```

Deploy (this brings up Redis, **PostgreSQL**, wallet-api, Envoy, Prometheus,
Grafana, plus the NetworkPolicies and PDBs):

```bash
kubectl apply -k deploy/k8s/

kubectl get pods -n wallet-demo
kubectl get svc -n wallet-demo
```

`wallet-api` retries its Postgres connection for up to 30 seconds at startup,
so it is safe to roll the API before/while the database pod becomes ready.
The API's `/login` UI and `/v1/auth/*` endpoints work exactly as in local dev;
log in from the browser or via `curl` to get a bearer token.

For GitOps CD with Argo CD, see `docs/argocd-cd.md`.

---

## Port Forward

Envoy:

```bash
kubectl -n wallet-demo port-forward svc/envoy 8080:8080 50051:50051
```

Prometheus:

```bash
kubectl -n wallet-demo port-forward svc/prometheus 9090:9090
```

Grafana:

```bash
kubectl -n wallet-demo port-forward svc/grafana 3000:3000
```

Open:

```text
Envoy HTTP:   http://localhost:8080
Prometheus:   http://localhost:9090
Grafana:      http://localhost:3000
```

Grafana demo login:

```text
admin / wallet-demo-change-me
```

Change the demo Redis, Postgres, Grafana, and JWT values in
`deploy/k8s/00-secrets.yaml` before using a shared cluster. For production,
replace this plain Kubernetes Secret with External Secrets, Sealed Secrets, or
SOPS-encrypted configuration, and generate a strong JWT secret with
`openssl rand -hex 32`.

---

## Seed Wallets Before Load Testing

The make target handles authentication (it registers/logs in the `demo`
account and attaches the bearer token):

```bash
make seed
```

Manually, if you prefer raw curl:

```bash
TOKEN=$(make -s demo-token)

curl -X POST http://localhost:8080/v1/simulate/seed \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"users":1000,"initial_balance_cents":100000}'
```

---

## Load Testing with hey

Install:

```bash
brew install hey
```

The make targets obtain a bearer token first, then run hey:

```bash
make load-smoke     # 100 requests, 5 concurrent
make load-normal    # 30s, 20 concurrent (MacBook Air M4 friendly)
make load-high      # 60s, 50 concurrent
```

Or manually (with a token from `make login`):

```bash
hey -z 30s -c 20 -q 2 \
  -m POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"users":500,"amount_cents":100}' \
  http://localhost:8080/v1/simulate/transfer
```

---

## Prometheus Queries

RPS:

```promql
sum(rate(wallet_http_requests_total[1m]))
```

Transfer success/failure:

```promql
sum(rate(wallet_transfer_total[1m])) by (status)
```

P95 latency:

```promql
histogram_quantile(
  0.95,
  sum(rate(wallet_request_duration_seconds_bucket[5m])) by (le)
)
```

HTTP status codes:

```promql
sum(rate(wallet_http_requests_total[1m])) by (status)
```

Auth events (logins, rejections, unauthorized API calls):

```promql
sum(increase(wallet_auth_events_total[15m])) by (event, result)
```

Failed logins per minute (a brute-force canary):

```promql
sum(rate(wallet_auth_events_total{event="login",result="invalid_credentials"}[5m]))
```

---

## Grafana Panels to Create

- RPS
- HTTP status code rate
- Transfer success/failure rate
- P95 latency
- P99 latency
- Active wallets
- CPU usage per pod
- Memory usage per pod
- Auth events (logins, rejections, unauthorized API calls)
- Failed logins per minute

---

## HPA

Enable metrics server:

```bash
minikube addons enable metrics-server
```

Apply HPA:

```bash
kubectl apply -f deploy/k8s/07-hpa.yaml
kubectl get hpa -n wallet-demo
```

---

## Failure Simulation

Kill wallet-api pods:

```bash
kubectl delete pod -n wallet-demo -l app=wallet-api
kubectl get pods -n wallet-demo -w
```

Restart Redis:

```bash
kubectl rollout restart deployment redis -n wallet-demo
```

Watch logs:

```bash
kubectl logs -n wallet-demo -l app=wallet-api --tail=100 -f
```

---

## Interview Explanation

> I built a cloud-native wallet transaction system using Golang, gRPC, Redis, Envoy, Minikube, Prometheus, Grafana, and hey. The service follows hexagonal architecture, so business logic is separated from transport and infrastructure. Redis stores wallet balances, transactions, and idempotency keys. Envoy acts as a central HTTP/gRPC load balancer. Prometheus scrapes application metrics and Grafana visualizes RPS, latency, error rate, and transfer status. I used hey to simulate high user traffic safely on a MacBook Air M4.

---

## Recommended Git Tags

```bash
git tag v1-basic-wallet-api
git tag v2-hexagonal-architecture
git tag v3-grpc-api
git tag v4-redis-storage
git tag v5-dockerized
git tag v6-minikube-yaml
git tag v7-envoy-load-balancer
git tag v8-prometheus-metrics
git tag v9-prometheus-deployment
git tag v10-grafana-dashboard
git tag v11-hey-load-test
git tag v12-hpa-resource-limits
git tag v13-failure-simulation
git tag v14-final-documentation
```

---

## CI/CD with GitHub Actions, Docker Hub, and Kubernetes

This project includes a CI/CD pipeline in:

```text
.github/workflows/ci-cd.yml
```

The workflow runs on pull requests, pushes to `main`, manual runs, and Git tags like `v1.0.0`.

### Pipeline Stages

```text
CI:
- Set up Go
- Download dependencies
- Check gofmt
- Verify go.mod and go.sum are tidy
- Run go vet
- Run go test with race detector and coverage
- Run govulncheck
- Build Docker image locally
- Scan Docker image with Trivy

CD:
- Login to Docker Hub
- Build multi-architecture Docker image
- Push image to Docker Hub
- Publish SBOM and provenance attestations
- Optional manual deploy to Kubernetes by image digest on a self-hosted runner
```

### Required GitHub Secrets

Add these secrets in your GitHub repository:

```text
DOCKERHUB_USERNAME
DOCKERHUB_TOKEN
```

Use a Docker Hub personal access token for `DOCKERHUB_TOKEN`, not your Docker Hub password.

For GitHub Actions deployments to a cloud Kubernetes cluster, also add:

```text
KUBE_CONFIG_B64
```

Generate it from the kubeconfig for your target cluster:

```bash
kubectl config view --raw --minify | base64 | tr -d '\n'
```

For local Minikube on your Mac, use a self-hosted runner with the `wallet-deploy` label instead. In that mode the workflow uses the runner's existing `~/.kube/config`, so `KUBE_CONFIG_B64` is optional.

### Docker Hub Image Output

On push to `main`:

```text
DOCKERHUB_USERNAME/wallet-api:latest
DOCKERHUB_USERNAME/wallet-api:sha-<short-commit-sha>
```

On Git tag:

```text
DOCKERHUB_USERNAME/wallet-api:v1.0.0
```

### Deploy Docker Hub Image to Minikube

```bash
kubectl -n wallet-demo set image deployment/wallet-api \
  wallet-api=your-dockerhub-username/wallet-api:sha-<short-commit-sha>

kubectl -n wallet-demo rollout status deployment/wallet-api
```

Or use:

```bash
make deploy-dockerhub-image DOCKERHUB_USERNAME=your-dockerhub-username IMAGE_TAG=sha-<short-commit-sha>
```

For digest-based deploys:

```bash
make deploy-dockerhub-digest \
  DOCKERHUB_USERNAME=your-dockerhub-username \
  IMAGE_DIGEST=sha256:<image-digest>
```

Full guide: `docs/ci-cd-dockerhub.md`

Self-hosted Minikube runner guide: `docs/self-hosted-runner.md`
