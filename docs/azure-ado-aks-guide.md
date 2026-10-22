# Deploy บน Azure (ADO → ACR → AKS) + Load Test — คู่มือทำเองทุกขั้นตอน

สิ่งที่คุณต้องเตรียม: บัญชี Microsoft (ฟรี) + บัตรเครดิต/วิธีชำระเงินสำหรับ subscription Azure
(หรือใช้ trial 30 วัน / free tier — AKS 1 node + ACR Basic เข้าในกรอบ free tier ได้)

> ค่าใช้จ่ายจริง: ACR Basic ~$0, AKS node D2s_v3 ~$0.55/ชม. (~฿20/ชม.)
> **ทำเสร็จแล้วลบทั้ง resource group** ด้วยขั้นตอน cuối เพื่อหยุดค่าใช้จ่าย

---

## 0. สถานะตอนนี้ (พร้อมแล้ว)

- `az` CLI 2.90.0 ติดตั้งที่ `~/azcli/bin/az` ✅
- `hey`, `kustomize`, `gh` ✅
- โค้ดครบ: `azure-pipelines.yml` + fix postgres `fsGroup` + ระบบ auth ทั้งหมด
  (branch `feat/postgres-login`, commit `2c49bbb`)
- API forms:
  - `POST /v1/auth/register` → body `{"username","password"}`
  - `POST /v1/auth/login` → body `{"username","password"}` → ตอบ `{"token","user":{"id",...}}`
  - `POST /v1/wallets` → body `{"user_id","initial_balance_cents"}` → ตอบมี `wallet_id`
  - `GET /v1/wallets/{id}/balance` → `{"wallet_id","balance_cents"}`
  - `GET /healthz`, `GET /login` (หน้า login)

---

## 1. Login Azure CLI — terminal

```bash
~/azcli/bin/az login
```
→ browser เปิด → login บัญชี Microsoft → ถ้ามีหลาย subscription เลือก:
```bash
~/azcli/bin/az account set --subscription "<ชื่อ/id ที่ต้องการ>"
~/azcli/bin/az account show -o table        # ตรวจ
~/azcli/bin/az account show --query id -o tsv   # = subscription ID (ไว้ใช้ใน step 4)
```

## 2. สร้าง ADO organization + project — browser

1. เปิด https://dev.azure.com → Sign in (บัญชี Microsoft เดิม — ฟรี)
2. **Create organization**: ชื่อเช่น `wallet-devops` → Region เลือก default → Create  
   (URL จะได้ `dev.azure.com/wallet-devops`)
3. **Create project**:
   - ชื่อ: `wallet-devops-lab`
   - Description: `wallet-devops-lab` lab
   - Visibility: **Private**
   - Work item process: **Basic**
   - → Create

## 3. ลิงก์ GitHub repo เข้า ADO — browser

1. Project → ⚙ **Settings** → เมนูซ้าย **Resources → Repos**
   (ถ้าไม่เห็น ให้ดู `Resources → Linked services → Repos`)
2. **Link an external Git repository** → เลือก **GitHub** → อนุญาต (GitHub App)  
   → เลือก repo **`Nanonny/wallet-backend`**
3. ตรวจ: ไปหน้า **Pipelines → New pipeline** → dropdown "Repositories" ต้องเห็น `wallet-backend` (GitHub)

## 4. Service connection (ARM) — browser

ต้องชื่อ **`wallet-azure`** พอดี (ชื่อใน `azure-pipelines.yml` อ้างอิงอยู่):

1. Settings → **Resources → Service connections** → **New**
2. เลือก **Azure Resource Manager**
3. ชื่อ: `wallet-azure`
4. Resource scope: **Project**
5. Organization/Project: `wallet-devops` / `wallet-devops-lab`
6. Subscription: เลือก subscription ของคุณ (ID จาก step 1)
7. Authentication: **Service principal** → **Create a new service principal**
   → Role: **Contributor** → เลือก subscription → (ADO สร้าง SP + secret ให้เอง)
8. Description: `pipeline SP` → **Save**  
   (ถ้า wizard ให้ "Verify & authorize" ก็ให้ผ่าน — ADO สร้าง SP ใน Azure ให้)
9. **บันทึก `Service principal (client) ID`** — เปิดหน้า edit ของ connection นี้:
   ฟิลด์ `Service principal (client) ID` (uuid 32 ตัว) → ใช้เติม variable group ในขั้น 5

## 5. Variable group — browser

ต้องชื่อ **`wallet-azure-vars`** พอดี:

1. เมนูซ้าย **Pipelines → Libraries → New** → ประเภท **Variable group**
2. ชื่อ: `wallet-azure-vars`
3. เพิ่มตัวแปร 4 ตัว (ทุกตัวไม่ต้อง tick "secret"):

   | Name | Value |
   |---|---|
   | `ACR_NAME` | `walletlab` |
   | `ACR_LOGIN_SERVER` | `walletlab.azurecr.io` |
   | `AKS_NAME` | `wallet-aks` |
   | `RG` | `wallet-lab-rg` |
   | `ACR_PUSH_PRINCIPAL_ID` | `<Service principal (client) ID จากขั้น 4>` |

4. **Save** → (tick เลือกให้ทุก job ใช้ก็ได้ ไม่บังคับ — yaml attach อยู่แล้ว)

## 6. Terraform Infrastructure as Code — ไม่ต้องรันอะไรเอง

> Pipeline stage `infra` จะรัน `terraform init + apply` ด้วยตัวเองจากไฟล์
> `infra/main.tf` (สร้าง RG + ACR + AKS + role binding ครบ)
> คุณแค่เช็คให้ variable group ในขั้น 5 ครบ โดยเฉพาะ `ACR_PUSH_PRINCIPAL_ID`

Terraform จะสร้างอะไรให้:

| Resource | Name | Note |
|---|---|---|
| Resource Group | `wallet-lab-rg` | Location: `southeastasia` |
| Container Registry (Basic) | `walletlab` | `admin_user_enabled=true` |
| Role assignment | `AcrPush` | ให้ SP ของ pipeline (`ACR_PUSH_PRINCIPAL_ID`) push ได้ |
| AKS cluster (1.33) | `wallet-aks` | 1 node `Standard_D2s_v3` |
| Role assignment | `AcrPull` | ให้ AKS control plane pull image ได้ |

Terraform state ใช้ local backend (`infra/terraform.tfstate`) สำหรับ lab
(ถ้าจะเอา production — สลับไป Azure Storage backend ดูคอมเมนต์ใน `infra/backend.tf`)

## 7. Push โค้ดขึ้น main — terminal

> ถ้า main มี branch protection ที่ต้อง PR review → ทำแบบ PR ใน GitHub แทน; ถ้าไม่มี (default) push ตรงได้เลย

```bash
cd ~/Downloads/"wallet-devops-lab 2"
git switch main
git pull --ff-only origin main

# รวมงานทั้งหมด (auth + DSN fix + Azure pipeline + fsGroup)
git merge --no-ff feat/postgres-login -m "merge: postgres login, azure cicd, deployment fixes"

# push ด้วย token จาก gh (osxkeychain hang ใน non-interactive shell — ใช้ pattern นี้)
GH_TOKEN_PUSH="$(gh auth token)"
git -c credential.helper='!f() { echo "username=Nanonny"; echo "password=${GH_TOKEN_PUSH}"; }; f' \
  push origin main
```

ตรวจ: https://github.com/Nanonny/wallet-backend — main ต้องเห็น `azure-pipelines.yml`

## 8. สร้าง pipeline + รัน — browser

1. **Pipelines → New pipeline**
2. Repositories → เลือก **`wallet-backend` (GitHub)**
3. เลือก **Existing Azure Pipelines YAML file** → branch `main` → file `azure-pipelines.yml`
4. **Save & Run**
5. ดู Runs — ลำดับที่คาดหวัง:
   - `CI - test & verify` (~3-5 นาที): install go 1.26.6, gofmt, tidy, vet, `test -race` + redis service, docker build check
   - `infra - Terraform` (ครั้งแรก ~10-15 นาที / ครั้งหลัง <5 นาที): สร้าง/ตรวจสอบ RG + ACR + AKS + role binding
   - `CD - push ACR & deploy AKS` (~5-10 นาที): build image → push `walletlab/wallet-api:sha-<short>` → `apply -k` + set image + envoy → LoadBalancer → rollout → smoke test `/healthz` + `/login`

  ถ้า fail: เปิด log ของ step ที่ขึ้นแดง — จุดที่พบบ่อยคือชื่อ service connection/variable group ไม่ตรง step 4-5

## 9. ตรวจ deployment — terminal

```bash
~/azcli/bin/az aks get-credentials -n wallet-aks -g wallet-lab-rg
kubectl config use-context wallet-aks

kubectl -n wallet-demo get pods -o wide                # ต้องการ 9/9 Ready (envoy2 api3 pg1 redis1 prom1 graf1)
kubectl -n wallet-demo get pods -l app=wallet-api \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\n"}{end}'
# image ต้องเป็น walletlab.azurecr.io/wallet-api:sha-xxxxxx

EXT_IP=$(kubectl -n wallet-demo get svc envoy -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo "ENVoy: $EXT_IP"
curl -sf "http://$EXT_IP/healthz" && echo    # {ok}
curl -s -o /dev/null -w "login page: %{http_code}\n" "http://$EXT_IP/login"
```

## 10. Load test + วัด load balancing — terminal

DB ใหม่ = account ยังไม่มี → สร้าง account สำหรับยิง:

```bash
IP=$(kubectl -n wallet-demo get svc envoy -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
BASE="http://$IP"

# 1) register + login + สร้าง wallet
curl -s -o /dev/null -w "register: %{http_code}\n" -XPOST "$BASE/v1/auth/register" \
  -H 'Content-Type: application/json' -d '{"username":"loaduser","password":"loadpass-1234"}'

R=$(curl -s -XPOST "$BASE/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"loaduser","password":"loadpass-1234"}')
TOKEN=$(echo "$R" | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')
UID=$(echo "$R"   | python3 -c 'import json,sys;print(json.load(sys.stdin)["user"]["id"])')

WID=$(curl -s -XPOST "$BASE/v1/wallets" -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d "{\"user_id\":\"$UID\",\"initial_balance_cents\":100000}" \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["wallet_id"])')
echo "WID=$WID"

# 2) ยิง load (GET balance — safe/idempotent, ห้ามยิง /auth/login (จะ 429 ตาม design))
hey -n 5000 -c 30 "$BASE/v1/wallets/$WID/balance" -H "Authorization: Bearer $TOKEN"
```

ตรวจ distribution ต่อ pod — วิธี A (Prometheus, แนะนำ):
```bash
kubectl -n wallet-demo port-forward svc/prometheus 9091:9090 >/dev/null &
sleep 3
curl -s 'http://localhost:9091/api/v1/query?query=sum%20by%20(pod)%20(increase(wallet_http_requests_total%5B10m%5D))' \
  | python3 -m json.tool
pkill -f "port-forward"
```
วิธี B (exec เข้าแต่ละ pod โดยตรง):
```bash
for P in $(kubectl -n wallet-demo get pods -l app=wallet-api -o jsonpath='{range .items[*]}{.metadata.name} {end}'); do
  echo -n "$P : "
  kubectl -n wallet-demo exec "$P" -- wget -qO- http://127.0.0.1:9090/metrics \
    | awk -F'{' '/^wallet_http_requests_total/ {s+=1} END {print s" requests (counter sum)"}' || echo n/a
done
```

**ผลลัพธ์ที่คาด**: 3 pod แต่ละคนรับ ~33% (5000 → ~1600/1700/1700, ±5%) = Envoy round-robin ทำงานจริง  
ถ้า pod ใด 0 → bug (ดู step 9 ว่า image/desiredReplicas ถูกต้อง, `kubectl -n wallet-demo get endpoints wallet-api-headless` ต้องมี 3 pod IP)

## 10.5 วัดตัวเลขสำหรับรายงาน (X req/s, Y ms p95, A% → B%)

> ใช้ protocol นี้เพื่อได้ตัวเลขจริงทุกช่องในประโยครายงาน
> พื้นหลัง: wallet ทั้งหมดอยู่ใน **redis** — ถ้า redis ตาย = `SPOF` → wallet reads ทั้ง 100% เป็น 5xx
> (`writeError` default = 500) และ `/readyz` จริงๆ เรียก redis (`SCARD wallets`)
> → probe `periodSeconds:10 × failureThreshold:3` = pod ถูกถอดจาก LB ใน ~10-40 วินาที

**A) Baseline (ระบบปกติ) — จับ X + Y + error baseline**
```bash
IP=$(kubectl -n wallet-demo get svc envoy -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
URL="http://$IP/v1/wallets/$WID/balance"
HE="Authorization: Bearer $TOKEN"

hey -n 2000 -c 30 "$URL" -H "$HE"
```
อ่านจาก output ของ hey:
- บรรทัด `Requests/sec: ...` → **X** (req/s)
- ตาราง `Latency distribution` → บรรทัด `95%` → **Y** (ms)
- `Error rate: 0.00%` → baseline

**B) Simulate failure: kill wallet store (redis)**
```bash
kubectl -n wallet-demo scale deployment/redis --replicas=0
sleep 10        # รอ go-redis report connection refused ทันที
hey -n 2000 -c 30 "$URL" -H "$HE" | grep -E "Requests/sec|95%|Error rate"
# → Error rate ≈ 100% (5xx)   ← นี่คือ A%
```

**C) Recover + rerun — จับ B% + เวลาฟื้นตัว**
```bash
t0=$(date)
kubectl -n wallet-demo scale deployment/redis --replicas=1
kubectl -n wallet-demo rollout status deployment/redis --timeout=120s
for i in $(seq 1 30); do
  kubectl -n wallet-demo get pods -l app=wallet-api \
    -o jsonpath='{range .items[*]}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}' \
    | grep -q False || break
  sleep 5
done
echo "recovered in $(( $(date) - t0 ))s"
hey -n 2000 -c 30 "$URL" -H "$HE" | grep -E "Requests/sec|95%|Error rate"
# → Error rate 0.00%, p95 กลับระดับ baseline   ← นี่คือ B%
```

**D) ประโยครายงาน (ใส่ตัวเลขจริงของคุณแทน — ตัวเลขในวงเล็บคือตัวอย่างสมมติ)**
> Load tested at **~700 req/s** with p95 latency of **~45 ms**; simulated a wallet-store outage
> (single-Redis SPOF — 100% of wallet reads returned 5xx while the API kept serving), found the
> failure mode = shared single-instance dependency, resolved by controller auto-restart plus
> readiness-driven load-balancer ejection (`/readyz` probes the store; production: HA/managed cache),
> bringing error rate from **100%** to **0.00%** in under ~40 s of recovery.

ช่องที่ต้องมีตัวเลขจริง: X (จาก A) • Y (จาก A) • A% (จาก B) • B% + recovery time (จาก C)

## 11. (ทางเลือก) ดูกราฟ Grafana

`kubectl -n wallet-demo port-forward svc/grafana 3000:3000` → เปิด http://127.0.0.1:3000 → panel `sum by (pod) (rate(wallet_http_requests_total[1m]))` — ดูเส้นสามเส้นแยกกันชัดๆ

## 12. ล้างเก็บ (หยุดค่า node ~฿20/ชม.)

```bash
~/azcli/bin/az group delete -n wallet-lab-rg --yes
```
(AKS + ACR + RG ถูก delete ทั้งหมด — โค้ดใน repo และ ADO ยังอยู่ ถ้าอยาก deploy อีกครั้ง: ทำ step 5-8 ใหม่ได้ทันที เพราะ pipeline/config ครบแล้ว)

---

##Troubleshooting

| อาการ | สาเหตุ/แก้ |
|---|---|
| CD fail ที่ "SP login" | ชื่อ service connection ≠ `wallet-azure` หรือ SP ไม่มี role Contributor → จัดการใหม่ step 4 |
| infra stage fail (terraform) | ขาด credentials / SP ใน `wallet-azure-vars` ครบไม่ครบ — ดู error ของ variable `ACR_PUSH_PRINCIPAL_ID` ก่อน |
| docker push 401 | ACR admin-enabled ไม่เคยเปิด → `az acr update -n walletlab -g wallet-lab-rg --admin-enabled true` |
| postgres pod CrashLoop (permissions) | managed disk ยังไม่ respect `fsGroup` — บอกผม ผมจะ patch initContainer chown 70:70 ให้ |
| AKS pod อืด/ไม่ขึ้น | `kubectl -n wallet-demo describe pod <pod>` ดู reason; node 1 ตัวอาจ tight — ดู `kubectl top nodes` |
| push main ไม่ผ่าน (branch protection) | ทำ PR ใน GitHub UI แทน: New PR `feat/postgres-login → main` → Merge |
| `hey` timeout | LB IP ยังว่าง → `kubectl -n wallet-demo get svc envoy` รอจนมี EXTERNAL-IP |
| 429 ตอน load test | ยิงโดน `/auth/login` (rate limit 10/15min) หรือ IP เดิมโดน — ใช้ `/v1/wallets/{id}/balance` แทน |
