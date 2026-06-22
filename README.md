# Cloud-Native Wallet Transaction Platform

DevOps / Platform Engineering portfolio project using:

- Minikube
- Kubernetes YAML
- Golang
- gRPC
- Hexagonal Architecture
- Redis
- Envoy Load Balancer
- Prometheus
- Grafana
- hey load testing

> Note: The service includes `api/proto/wallet.proto`. To make the scaffold runnable immediately without requiring generated protobuf files, the included gRPC adapter uses grpc-go with a JSON codec and mirrors the proto messages. For production, generate normal protobuf stubs with `make proto` and replace the adapter with generated `walletpb` handlers.

---

## Architecture Diagram

```mermaid
flowchart TD
    U[User / hey load test] --> E[Envoy Proxy<br/>HTTP :8080<br/>gRPC :50051]
    E --> P1[wallet-api pod 1<br/>Go HTTP + gRPC]
    E --> P2[wallet-api pod 2<br/>Go HTTP + gRPC]
    E --> P3[wallet-api pod 3<br/>Go HTTP + gRPC]
    P1 --> R[(Redis<br/>wallet state<br/>transactions<br/>idempotency)]
    P2 --> R
    P3 --> R
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
│   ├── domain/
│   ├── application/
│   ├── adapters/
│   │   ├── http/
│   │   ├── grpc/
│   │   ├── redis/
│   │   ├── memory/
│   │   └── metrics/
│   └── config/
├── deploy/k8s/
│   ├── 00-namespace.yaml
│   ├── 01-redis.yaml
│   ├── 02-wallet-api.yaml
│   ├── 03-envoy-config.yaml
│   ├── 04-envoy.yaml
│   ├── 05-prometheus.yaml
│   ├── 06-grafana.yaml
│   ├── 07-hpa.yaml
│   └── 08-pdb.yaml
├── loadtest/
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

Test HTTP API:

```bash
curl -X POST http://localhost:8080/v1/simulate/seed \
  -H "Content-Type: application/json" \
  -d '{"users":10,"initial_balance_cents":100000}'

curl -X POST http://localhost:8080/v1/simulate/transfer \
  -H "Content-Type: application/json" \
  -d '{"users":10,"amount_cents":100}'

curl http://localhost:8080/readyz
curl http://localhost:9090/metrics
```

Test gRPC local client:

```bash
make grpc-client
```

---

## Local Run With Redis

```bash
docker run --name wallet-redis -p 6379:6379 -d redis:7-alpine

go mod tidy
make run
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

Deploy:

```bash
kubectl apply -f deploy/k8s/

kubectl get pods -n wallet-demo
kubectl get svc -n wallet-demo
```

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

Grafana default login:

```text
admin / admin
```

---

## Seed Wallets Before Load Testing

```bash
curl -X POST http://localhost:8080/v1/simulate/seed \
  -H "Content-Type: application/json" \
  -d '{"users":1000,"initial_balance_cents":100000}'
```

---

## Load Testing with hey

Install:

```bash
brew install hey
```

Smoke test:

```bash
hey -n 100 -c 5 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"users":100,"amount_cents":100}' \
  http://localhost:8080/v1/simulate/transfer
```

Normal load for MacBook Air M4:

```bash
hey -z 30s -c 20 -q 2 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"users":500,"amount_cents":100}' \
  http://localhost:8080/v1/simulate/transfer
```

High but still local-safe:

```bash
hey -z 60s -c 50 -q 2 \
  -m POST \
  -H "Content-Type: application/json" \
  -d '{"users":1000,"amount_cents":100}' \
  http://localhost:8080/v1/simulate/transfer
```

Avoid starting with this on a MacBook Air:

```bash
hey -z 5m -c 500 http://localhost:8080/v1/simulate/transfer
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
