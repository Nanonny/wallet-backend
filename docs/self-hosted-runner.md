# Self-Hosted GitHub Actions Runner for Local Minikube Deploys

Use this when you want GitHub Actions to deploy into Minikube running on your Mac. GitHub-hosted runners cannot reach a Minikube cluster that exists only on your laptop, so the deploy job runs on a self-hosted runner with the label:

```text
wallet-deploy
```

The CI and Docker Hub push jobs still run on GitHub-hosted Ubuntu runners. Only the Kubernetes deploy job uses your self-hosted runner.

## 1. Prepare Minikube on Your Mac

Start Minikube and confirm `kubectl` works from the same macOS user that will run the GitHub runner:

```bash
minikube start --driver=docker --cpus=4 --memory=6144 --disk-size=30g
kubectl config current-context
kubectl get nodes
```

Keep Docker Desktop running if your Minikube profile uses the Docker driver.

## 2. Add the Runner in GitHub

In your repository:

```text
Settings > Actions > Runners > New self-hosted runner
```

Choose your OS and architecture, then follow the download and configure commands GitHub shows. For Apple Silicon, choose macOS ARM64.

When running `config.sh`, add this label:

```bash
./config.sh \
  --url https://github.com/<owner>/<repo> \
  --token <token-from-github> \
  --labels wallet-deploy
```

GitHub's official runner docs describe using labels in `runs-on`; this project routes deploy jobs with:

```yaml
runs-on: [self-hosted, wallet-deploy]
```

## 3. Start the Runner

For local Minikube, start the runner manually first:

```bash
./run.sh
```

This is the least surprising option because it uses your current user's `~/.kube/config` and Minikube profile. If you later install it as a service, make sure the service user can run:

```bash
kubectl config current-context
kubectl get nodes
```

## 4. Required GitHub Secrets

For build and push:

```text
DOCKERHUB_USERNAME
DOCKERHUB_TOKEN
```

`KUBE_CONFIG_B64` is optional for self-hosted Minikube deploys. If it is not set, the deploy job uses:

```text
~/.kube/config
```

from the self-hosted runner machine.

## 5. Run the Deploy

Open GitHub Actions and run:

```text
CI/CD - Build, Scan, Push, and Deploy
```

Use these inputs:

```text
deploy_to_cluster = true
environment       = staging
```

The deploy job will:

```text
1. Use the self-hosted runner with label wallet-deploy
2. Validate Kubernetes access
3. Apply deploy/k8s/
4. Set wallet-api to the Docker Hub image digest pushed by the same workflow
5. Wait for rollout status
```

## 6. Quick Troubleshooting

If the workflow stays queued, check that the runner is online and has the `wallet-deploy` label.

If deploy fails with kubeconfig errors, run this on the Mac in the same terminal/user as the runner:

```bash
kubectl config current-context
kubectl get nodes
```

If pods cannot pull the Docker Hub image, make the Docker Hub repository public or add an image pull secret to the `wallet-demo` namespace.
