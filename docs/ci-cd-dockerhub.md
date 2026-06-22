# CI/CD with GitHub Actions, Docker Hub, and Kubernetes Deploys

The main delivery workflow lives at:

```text
.github/workflows/ci-cd.yml
```

## Pipeline Stages

```text
1. CI - Test, Scan, and Build
   - Checkout source code
   - Set up Go
   - Download dependencies
   - Check gofmt
   - Verify go.mod and go.sum are tidy
   - Run go vet
   - Run go test with race detector and coverage
   - Run govulncheck
   - Build Docker image locally
   - Scan the local image with Trivy for HIGH and CRITICAL vulnerabilities

2. CD - Push Docker Image to Docker Hub
   - Login to Docker Hub
   - Generate image tags and labels
   - Build linux/amd64 and linux/arm64 images
   - Push image tags to Docker Hub
   - Publish SBOM and provenance attestations from BuildKit

3. CD - Deploy to Kubernetes
   - Manual only through workflow_dispatch
   - Runs on a self-hosted runner labeled wallet-deploy
   - Apply Kubernetes manifests
   - Set wallet-api image by immutable image digest
   - Wait for Kubernetes rollout status
```

## Required GitHub Secrets

Create these repository secrets in GitHub:

```text
DOCKERHUB_USERNAME = your Docker Hub username
DOCKERHUB_TOKEN    = your Docker Hub personal access token
```

Do not use your Docker Hub account password. Use a Docker Hub personal access token.

For GitHub Actions deployments to a cloud cluster, also add:

```text
KUBE_CONFIG_B64 = base64 encoded kubeconfig for the target cluster
```

Create it from your current kubeconfig:

```bash
kubectl config view --raw --minify | base64 | tr -d '\n'
```

For local Minikube deploys, use a self-hosted GitHub Actions runner on your Mac. In that mode, `KUBE_CONFIG_B64` is optional because the deploy job can use the runner's existing `~/.kube/config`.

## Docker Hub Image Tags

When you push to `main`, GitHub Actions pushes:

```text
your-dockerhub-username/wallet-api:latest
your-dockerhub-username/wallet-api:sha-<short-commit-sha>
```

When you push a Git tag like `v1.0.0`, GitHub Actions also pushes:

```text
your-dockerhub-username/wallet-api:v1.0.0
```

Prefer `sha-<short-commit-sha>` or an image digest for deployments. Use `latest` only for local demos.

## Local Minikube Deploy

Build the image inside Minikube and apply the manifests:

```bash
make minikube-start
eval $(minikube docker-env)
make docker-build
make deploy
make rollout-status
```

Expose the app through Envoy:

```bash
make port-forward-envoy
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Deploy a Docker Hub Image from Your Workstation

Apply the Kubernetes resources, then update the deployment to a registry image:

```bash
make deploy
make deploy-dockerhub-image \
  DOCKERHUB_USERNAME=your-dockerhub-username \
  IMAGE_TAG=sha-<short-commit-sha>
```

For the strongest traceability, deploy by digest:

```bash
make deploy-dockerhub-digest \
  DOCKERHUB_USERNAME=your-dockerhub-username \
  IMAGE_DIGEST=sha256:<image-digest>
```

Check rollout:

```bash
kubectl -n wallet-demo rollout status deployment/wallet-api
kubectl -n wallet-demo get pods -l app=wallet-api
```

## Deploy from GitHub Actions

1. Add `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` as repository secrets.
2. Register a self-hosted runner with the `wallet-deploy` label.
3. Make sure the runner machine can run `kubectl get nodes`.
4. Open GitHub Actions.
5. Select `CI/CD - Build, Scan, Push, and Deploy`.
6. Click `Run workflow`.
7. Set `deploy_to_cluster` to `true`.
8. Choose `staging` or `production`.
9. Run the workflow.

Self-hosted runner setup guide: `docs/self-hosted-runner.md`

The deploy job applies `deploy/k8s/`, updates `deployment/wallet-api` to:

```text
DOCKERHUB_USERNAME/wallet-api@sha256:<digest>
```

Then it waits for:

```bash
kubectl -n wallet-demo rollout status deployment/wallet-api --timeout=180s
```

## Rollback

Rollback the previous ReplicaSet:

```bash
kubectl -n wallet-demo rollout undo deployment/wallet-api
kubectl -n wallet-demo rollout status deployment/wallet-api
```

View rollout history:

```bash
kubectl -n wallet-demo rollout history deployment/wallet-api
```
