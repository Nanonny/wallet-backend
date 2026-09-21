# CD with Argo CD

เอกสารนี้เพิ่ม CD แบบ GitOps ด้วย Argo CD ต่อจาก CI/CD เดิมของโปรเจกต์นี้

## ภาพรวม Flow

```text
Developer push
  -> GitHub Actions runs tests, scans, builds image
  -> GitHub Actions pushes Docker image to Docker Hub
  -> Git image manifest is updated in deploy/k8s/kustomization.yaml
  -> Argo CD detects the Git change
  -> Argo CD syncs Kubernetes manifests to the cluster
```

หลังเปิดใช้ Argo CD แล้ว ให้ถือว่า Git คือ source of truth ของ cluster ถ้าต้องเปลี่ยน image, replicas, env, config หรือ resource limits ให้เปลี่ยนใน Git แล้วปล่อยให้ Argo CD sync แทนการ `kubectl set image` จาก pipeline โดยตรง

## Prerequisites

สำหรับเครื่อง local:

```bash
brew install argocd kustomize
```

ต้องมี:

- Kubernetes cluster ที่ `kubectl` ใช้งานได้ เช่น Minikube
- Docker Hub image ของ `wallet-api` ที่ cluster pull ได้
- GitHub repository ที่ Argo CD อ่านได้ ถ้าเป็น private repository ให้เพิ่ม repo credential ใน Argo CD ก่อน

## Files Added for Argo CD

```text
deploy/k8s/kustomization.yaml
deploy/argocd/apps/wallet-api.yaml
```

`deploy/k8s/kustomization.yaml` รวม Kubernetes manifests ทั้งหมดและกำหนด image เริ่มต้นเป็น:

```yaml
images:
  - name: wallet-api
    newName: wallet-api
    newTag: dev
```

สำหรับ CD จริง ให้เปลี่ยน image นี้เป็น Docker Hub image เช่น:

```bash
cd deploy/k8s
kustomize edit set image wallet-api=your-dockerhub-username/wallet-api:sha-<short-commit-sha>
cd ../..
git add deploy/k8s/kustomization.yaml
git commit -m "chore(gitops): deploy wallet-api sha-<short-commit-sha>"
git push
```

## 1. Install Argo CD on Minikube

```bash
kubectl create namespace argocd
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl -n argocd rollout status deployment/argocd-server
```

เปิด Argo CD UI:

```bash
kubectl -n argocd port-forward svc/argocd-server 8081:443
```

เปิด:

```text
https://localhost:8081
```

ดู initial admin password:

```bash
argocd admin initial-password -n argocd
```

Login ด้วย:

```text
username: admin
password: value จากคำสั่งด้านบน
```

## 2. Configure the Argo CD Application

Application ปัจจุบันชี้ไปที่ `Nanonny/wallet-backend` แล้ว หาก fork repository
ให้แก้ `repoURL` ใน `deploy/argocd/apps/wallet-api.yaml` ให้เป็น repository ของคุณ:

```yaml
source:
  repoURL: https://github.com/<owner>/<repo>.git
  targetRevision: main
  path: deploy/k8s
```

จากนั้น apply Application:

```bash
kubectl apply -f deploy/argocd/apps/wallet-api.yaml
kubectl -n argocd get application wallet-api
```

ถ้า repository เป็น private ให้เพิ่ม credential ก่อน sync:

```bash
argocd repo add https://github.com/<owner>/<repo>.git \
  --username <github-username> \
  --password <github-token>
```

ถ้าใช้ Argo CD CLI:

```bash
argocd app get wallet-api
argocd app sync wallet-api
```

เมื่อ sync สำเร็จ ตรวจ workload:

```bash
kubectl -n wallet-demo get deploy,pod,svc,hpa
kubectl -n wallet-demo rollout status deployment/wallet-api
```

## 3. Connect GitHub Actions to GitOps

workflow ปัจจุบันมี job `gitops-update` ซึ่ง build/push image แล้ว commit ค่า image
แบบ immutable `sha-<short-commit-sha>` กลับเข้า Git หลัง push image สำเร็จ

ตัวอย่าง job ที่มีอยู่ใน `.github/workflows/ci-cd.yml`:

```yaml
  gitops-update:
    name: CD - Update GitOps image tag for Argo CD
    runs-on: ubuntu-latest
    needs: dockerhub
    if: github.event_name != 'pull_request' && github.ref == 'refs/heads/main'
    permissions:
      contents: write

    steps:
      - name: Checkout source code
        uses: actions/checkout@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Install Kustomize
        run: |
          curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash
          sudo mv kustomize /usr/local/bin/kustomize

      - name: Update wallet-api image in GitOps manifest
        env:
          DOCKERHUB_USERNAME: ${{ secrets.DOCKERHUB_USERNAME }}
        run: |
          IMAGE_TAG="sha-${GITHUB_SHA::7}"
          cd deploy/k8s
          kustomize edit set image wallet-api="${DOCKERHUB_USERNAME}/wallet-api:${IMAGE_TAG}"
          cd ../..

      - name: Commit GitOps image update
        run: |
          if git diff --quiet deploy/k8s/kustomization.yaml; then
            echo "GitOps manifest already points to this image."
            exit 0
          fi
          git config user.name "github-actions[bot]"
          git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
          git add deploy/k8s/kustomization.yaml
          git commit -m "chore(gitops): deploy wallet-api sha-${GITHUB_SHA::7} [skip ci]"
          git push
```

หมายเหตุ:

- ถ้า branch `main` มี protection rule ที่ไม่ให้ `GITHUB_TOKEN` push ตรง ให้เปลี่ยน job นี้เป็นเปิด Pull Request แทน หรือใช้ bot token ที่ได้รับอนุญาต
- เมื่อใช้ Argo CD แล้ว job `deploy` แบบ self-hosted runner ที่ `kubectl set image` ควรใช้เฉพาะกรณีทดลอง/manual เท่านั้น เพราะ Argo CD จะ sync กลับตาม Git

## 4. Verify Sync and Health

```bash
kubectl -n argocd get application wallet-api
kubectl -n wallet-demo get pods
kubectl -n wallet-demo describe deployment wallet-api
```

ถ้าต้องการเปิด service:

```bash
kubectl -n wallet-demo port-forward svc/envoy 8080:8080 50051:50051
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## 5. Rollback

ใน GitOps ให้ rollback ด้วย Git:

```bash
git revert <commit-that-updated-kustomization>
git push
```

Argo CD จะเห็น commit ใหม่และ sync กลับไปยัง image ก่อนหน้า

## 6. Useful Commands

```bash
argocd app list
argocd app get wallet-api
argocd app diff wallet-api
argocd app sync wallet-api
argocd app history wallet-api
```

## References

- Argo CD Getting Started: https://argo-cd.readthedocs.io/en/stable/getting_started/
- Argo CD Application Specification: https://argo-cd.readthedocs.io/en/stable/user-guide/application-specification/
- Argo CD Automated Sync: https://argo-cd.readthedocs.io/en/stable/user-guide/auto_sync/
- Argo CD CI Automation: https://argo-cd.readthedocs.io/en/stable/user-guide/ci_automation/
- Argo CD Kustomize guide: https://argo-cd.readthedocs.io/en/stable/user-guide/kustomize/
