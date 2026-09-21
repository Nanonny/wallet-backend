# Security Audit — wallet-devops-lab

**Date:** 2026-09-21
**Scope:** Go application (`cmd/`, `internal/`), Kubernetes manifests (`deploy/k8s/`), CI (`.github/workflows/ci-cd.yml`), Dockerfile, and the embedded web frontends.
**Method:** manual code review + static tooling (`go vet`, `gofmt`, `govulncheck@latest`) + live black-box smoke tests against a running instance.

---

## Executive summary

Before this round of work the service had **no authentication at all**: anyone who could reach `:8080` or `:50051` could create wallets, mint funds via deposit, withdraw, and transfer between any wallet ID. On top of that, the dependency set carried **13 govulncheck-confirmed vulnerabilities** (grpc v1.81.1 + the local Go 1.26.2 toolchain), several of which were remote DoS vectors on the unauthenticated gRPC port.

**Fixed in this round (implemented and tested):**

| # | Finding | Severity | Status |
|---|---------|----------|--------|
| 1 | No authentication on any fund-movement endpoint (HTTP **and** gRPC) | Critical | ✅ Auth wall on all `/v1/*` + gRPC interceptor |
| 2 | No ownership model — any wallet ID readable/writable (IDOR) | Critical | ✅ Ownership enforced; foreign wallets → `404` |
| 3 | 13 confirmed dependency vulnerabilities | High | ✅ 0 (`grpc v1.83.2`, Go `1.26.6`) |
| 4 | No brute-force / rate-limit protection on login | High | ✅ 10 logins / 15 min / IP+user, 429 + `Retry-After` |
| 5 | Plaintext password storage path (none existed; risk on adding auth) | High | ✅ bcrypt cost 12, dummy-hash timing guard, hash-only in DB |
| 6 | No CSP / security headers on the frontends | Medium | ✅ CSP, `nosniff`, `frame-ancestors`, referrer & permissions policy |
| 7 | JWT secret had no source (hardcoding risk) | Medium | ✅ `JWT_SECRET` env + Secret manifest + safe warn-on-fallback |
| 8 | Unexpected errors leaked to clients (driver internals) | Medium | ✅ masked to generic 500, full error server-side only |
| 9 | No NetworkPolicies — every pod reached Redis/PG/metrics | High | ✅ `09-networkpolicy.yaml` (default-deny + allow rules) |
| 10 | Redis/Grafana/Prometheus containers unhardened; no PDB for Redis | Medium | ✅ securityContext + PDBs (Redis, Postgres) |
| 11 | Frontend auto-created backend wallets from an anonymous tab | Low | ✅ now requires a session |
| 12 | Time-derived IDs (collision risk) for new user rows | Low | ✅ crypto/rand IDs for users |

**Still open — recommended follow-ups** (all Medium/Low; the big one is TLS):

| # | Finding | Severity | Suggestion |
|---|---------|----------|------------|
| A | **No TLS anywhere** — Envoy is a plaintext passthrough; gRPC, Redis, and Postgres traffic is unencrypted in-cluster | High | Terminate TLS at Envoy (`tls_certificate` + `sni_context`), TLS upstream sockets to wallet-api, `sslmode=require` for Postgres, Redis `--tls`. Minimum: keep these inside the default-deny namespace and never port-forward them to untrusted networks |
| B | Default, documented credentials for Redis/Grafana/Postgres/JWT in `00-secrets.yaml` | High | Rotate per cluster before sharing it; move to SOPS/Sealed Secrets/ESO; never commit real values |
| C | `/metrics`, Prometheus UI, and `/readyz` expose wallet counts to anything that can reach the pod | Medium | NetworkPolicy already scopes this to the namespace; additionally drop the metrics port from the `wallet-api` Services and remove the wallet count from `/readyz` before exposing externally |
| D | CI gaps: no SAST (gosec/semgrep), no secret scanning (gitleaks), no k8s manifest policy check (kubesec/conftest), Trivy only scans `os,library`, actions pinned by tag, GitOps commits a mutable tag | Medium | Add gitleaks + gosec jobs (small, CI already gates govulncheck/Trivy); commit image **digests** to the Argo App |
| E | Self-hosted runner decodes a base64 KUBE_CONFIG + falls back to `~/.kube/config` | Medium | Workload identity / short-lived tokens; scope the kubeconfig to the `wallet-demo` namespace |
| F | Envoy still runs as root; Redis has a single replica and no TLS | Low | Non-root Envoy sidecar image; Redis Sentinel or an operator if durability matters |
| G | Transaction IDs derive from `UnixNano()` (two ops in one nanosecond collide) | Low | Use the existing `crypto/rand` `newID()` for `txn_` IDs |
| H | Wallets have no per-transaction amount cap | Low | Add a sane ceiling (e.g. ≤ 10M USD) in the application layer |

---

## Details

### 1. Authentication & authorization (fixed)

**Before:** `internal/adapters/http/handler.go` registered `/v1/wallets`, `/v1/wallets/transfer`, `/v1/simulate/*` with no middleware; `wallet_grpc_handler.go` had no interceptors. `domain/errors.go` even defined `ErrUnauthorized` — planned but never wired.

**After:**

- `POST /v1/auth/register` — username policy (3–32, `[a-z0-9_]`), password policy (8–128, letter+digit), bcrypt(12) hash stored. Duplicate → 409, weak → 400.
- `POST /v1/auth/login` — constant-time bcrypt compare; unknown usernames still burn a compare against a real dummy digest (`internal/adapters/credential`) so response timing doesn't reveal which usernames exist. 10 attempts / 15 min / (IP, username), then `429` + `Retry-After`.
- Sessions are HS256 JWTs (`iss=wallet-api`, sub=username, 8h TTL). Validation enforces signature, issuer, allowed method (rejects `alg=none`), and expiry. Secret ≥ 16 chars, from `JWT_SECRET`; a missing secret generates an ephemeral one **with a loud startup warning** rather than hardcoding anything.
- All `/v1/wallets/*` and `/v1/simulate/*` routes (HTTP and gRPC) require `Authorization: Bearer <token>` (gRPC also accepts `grpc-token` metadata). Missing/invalid → `401` / `UNAUTHENTICATED` — **fail closed**.
- **Ownership:** the authenticated subject is attached to the request context (`internal/application/subject.go`) and the application service enforces wallet ownership: balance/deposit/withdraw on a foreign wallet → `404 wallet not found` (no existence leak), creating a wallet for a foreign owner → `403`, transferring **to** a foreign wallet allowed, transferring **from** one → `404`. Load-simulation endpoints are system operations and intentionally bypass ownership (`WithoutSubject`) while still requiring a valid session.
- Auth events are visible in Prometheus: `wallet_auth_events_total{event, result}` — a failed-login rate panel is a brute-force canary.

**Evidence:** `internal/adapters/http/handler_test.go` (register/login/me round-trip, 401s, 404 cross-user, 403 foreign-owner, rate-limit 429), `internal/application/wallet_service_test.go` (ownership matrix), `internal/adapters/grpc/auth_test.go` (interceptor accept/reject). All pass with `-race`.

### 2. Secrets & key management

- Passwords: bcrypt cost 12; only the digest reaches Postgres (`INSERT` parameterized, `pq` driver, no query building from input).
- JWT secret: `JWT_SECRET` env → `00-secrets.yaml` key `jwt-secret` in clusters; documented `openssl rand -hex 32` generation.
- DSN passwords are stripped of their `password=` segment in error text (`maskPassword` in `internal/adapters/postgres/postgres.go`).
- **Open (B):** the committed defaults in `00-secrets.yaml` are demo values — fine for a private minikube, unacceptable for a shared cluster.

### 3. Transport & network (mostly open — see A, B, C)

- No TLS at any hop today (Envoy has no `tls_certificate`, gRPC insecure creds, `PGSSLMODE=disable`). This is the single largest remaining gap and the main item for the next round.
- `09-networkpolicy.yaml`: default-deny ingress in `wallet-demo` + explicit allows (envoy→api:8080/50051, envoy/prometheus→api:9090, grafana→prometheus:9090). Enforced on CNI clusters (Calico/Cilium/KubeRouter); minikube does not enforce NPs, so the file also documents intent.
- All Services are `ClusterIP`; there is no Ingress/LoadBalancer/NodePort, so external reach is deliberately limited to `kubectl port-forward`.

### 4. Input validation & injection

- JSON decoding hardening kept: 1 MiB `MaxBytesReader`, `DisallowUnknownFields` (verified by test), trailing-JSON rejection.
- Usernames, wallet IDs, request IDs are opaque strings; Postgres queries are 100% parameterized — no SQL injection surface exists (there was no SQL before this round either; the new adapter is the only one).
- **Open (G):** `txn` IDs still use `UnixNano()` in the Redis/memory repositories — low impact, listed above.

### 5. Dependencies & supply chain (fixed)

```text
govulncheck before:  13 vulnerabilities from 1 module + Go stdlib
                      (grpc v1.81.1: GO-2026-6443 server panic, GO-2026-6348 H2 OOM, GO-2026-6061;
                       go1.26.2 stdlib: 10 findings incl. html/template XSS — fixed ≤1.26.6)
govulncheck after:   "Your code is affected by 0 vulnerabilities."
                      (grpc v1.83.2, go directive 1.26.6, toolchain auto-download verified)
```

- New dependencies are minimal and standard: `golang-jwt/jwt/v5`, `golang.org/x/crypto` (bcrypt), `lib/pq`.
- CI already gates `govulncheck`, `Trivy (CRITICAL,HIGH)`, SBOM + provenance, and runs tests with `-race`. **Open (D):** add gitleaks + gosec + a manifest policy check; commit image digests to the Argo App.

### 6. Kubernetes hardening

**Kept (already good):** wallet-api runs non-root (10001), drops ALL capabilities, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, `seccompProfile: RuntimeDefault`, `automountServiceAccountToken: false`, full resource requests/limits, HPA 3–6, PDB, `maxUnavailable: 0` rollouts, topology spread probes.

**Added:** securityContext (non-root users matching the official images: redis 999, grafana 472, prometheus 65534, postgres 70) + dropped capabilities for redis/grafana/prometheus/postgres; PDBs for redis and postgres; default-deny NetworkPolicy set.

**Open:** Envoy image still needs root (F).

### 7. Frontend (fixed, with one caveat)

- Login page: zero external scripts; strict CSP (`default-src 'none'; script-src 'self'`), JS served from an embedded same-origin asset.
- Wallet console: CSP allow-listed to the exact CDN origins it actually uses, `nosniff`, `frame-ancestors 'none'`, `no-referrer`, permissions policy.
- Session handling: token in `localStorage` + `Authorization` header (no cookies → no CSRF surface); 401 anywhere → clear session → `/login`.
- **Caveat:** the console still loads Tailwind/React/Babel from CDNs without SRI hashes. For a local lab this is acceptable; pin with `integrity=` or self-host before exposing it publicly.

---

## Verification evidence

```text
$ go vet ./...                      # clean
$ gofmt -l .                        # clean
$ go test -race -cover ./...        # 7 packages ok (application 78%, http 63.8%, memory 73%, credential 80%)
$ govulncheck ./...                 # 0 vulnerabilities (was 13)
```

Live black-box run (`STORAGE=memory`, real HTTP):

```text
unauthenticated POST /v1/wallets            → 401
register demo/wallet1234                    → 201
duplicate register (case-variant)           → 409
wrong password login                        → 401
login                                       → 200 + JWT
POST /v1/wallets (Bearer)                   → 201, owned by "demo"
GET balance (Bearer)                        → 200
GET same wallet as different user           → 404  (no existence leak)
GET /login                                  → 200, strict CSP, JS asset 200
gRPC client: HTTP register+login → metadata
        CreateWallet + GetBalance           → OK, UNAUTHENTICATED without token
wallet_auth_events_total{...}               # all outcomes counted in Prometheus
```

---

## Residual risk statement

As a **local portfolio/lab** the project is now in good shape: authenticated, bcrypt-hashed, rate-limited, ownership-scoped, default-deny networking in-cluster, zero known dependency vulnerabilities, and a green race-enabled test suite.

As a **production money-moving service** it is not ready, and the blockers would be, in order: **TLS end-to-end (A)** → **real per-cluster secrets (B)** → **restrict the monitoring surface (C)** → **CI supply-chain hardening (D)**. None of them require architectural change; all are configuration and CI work.
