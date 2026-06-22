APP_NAME=wallet-api
IMAGE_NAME=wallet-api:dev
NAMESPACE=wallet-demo

# Docker Hub settings. Override these when running make, for example:
# make dockerhub-build DOCKERHUB_USERNAME=yourname IMAGE_TAG=v1.0.0
DOCKERHUB_USERNAME?=your-dockerhub-username
DOCKERHUB_IMAGE?=$(DOCKERHUB_USERNAME)/$(APP_NAME)
IMAGE_TAG?=latest
IMAGE_DIGEST?=

.PHONY: run run-memory grpc-client fmt-check vet test test-ci ci-local build docker-build dockerhub-build dockerhub-push vuln minikube-start minikube-image deploy deploy-dockerhub-image deploy-dockerhub-digest rollout-status delete port-forward-envoy port-forward-prometheus port-forward-grafana seed load-smoke load-normal load-high proto

run:
	STORAGE=redis REDIS_ADDR=localhost:6379 go run ./cmd/wallet-api

run-memory:
	STORAGE=memory go run ./cmd/wallet-api

grpc-client:
	go run ./cmd/grpc-client

fmt-check:
	test -z "$$(gofmt -l .)"

vet:
	go vet ./...

test:
	go test ./...

test-ci:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...

ci-local: fmt-check vet test-ci docker-build

build:
	go build -o bin/$(APP_NAME) ./cmd/wallet-api

docker-build:
	docker build -t $(IMAGE_NAME) .

dockerhub-build:
	docker build -t $(DOCKERHUB_IMAGE):$(IMAGE_TAG) .

dockerhub-push:
	docker push $(DOCKERHUB_IMAGE):$(IMAGE_TAG)

vuln:
	govulncheck ./...

minikube-start:
	minikube start --driver=docker --cpus=4 --memory=6144 --disk-size=30g

minikube-image:
	eval $$(minikube docker-env) && docker build -t $(IMAGE_NAME) .

deploy:
	kubectl apply -f deploy/k8s/

# Use this after GitHub Actions has pushed the image to Docker Hub.
# Example:
# make deploy-dockerhub-image DOCKERHUB_USERNAME=yourname IMAGE_TAG=latest
deploy-dockerhub-image:
	kubectl -n $(NAMESPACE) set image deployment/wallet-api wallet-api=$(DOCKERHUB_IMAGE):$(IMAGE_TAG)
	kubectl -n $(NAMESPACE) rollout status deployment/wallet-api

deploy-dockerhub-digest:
	test -n "$(IMAGE_DIGEST)"
	kubectl -n $(NAMESPACE) set image deployment/wallet-api wallet-api=$(DOCKERHUB_IMAGE)@$(IMAGE_DIGEST)
	kubectl -n $(NAMESPACE) rollout status deployment/wallet-api

rollout-status:
	kubectl -n $(NAMESPACE) rollout status deployment/wallet-api

delete:
	kubectl delete -f deploy/k8s/ --ignore-not-found=true

port-forward-envoy:
	kubectl -n $(NAMESPACE) port-forward svc/envoy 8080:8080 50051:50051

port-forward-prometheus:
	kubectl -n $(NAMESPACE) port-forward svc/prometheus 9090:9090

port-forward-grafana:
	kubectl -n $(NAMESPACE) port-forward svc/grafana 3000:3000

seed:
	curl -X POST http://localhost:8080/v1/simulate/seed \
	  -H "Content-Type: application/json" \
	  -d '{"users":1000,"initial_balance_cents":100000}'

load-smoke:
	hey -n 100 -c 5 -m POST -H "Content-Type: application/json" \
	  -d '{"users":100,"amount_cents":100}' \
	  http://localhost:8080/v1/simulate/transfer

load-normal:
	hey -z 30s -c 20 -q 2 -m POST -H "Content-Type: application/json" \
	  -d '{"users":500,"amount_cents":100}' \
	  http://localhost:8080/v1/simulate/transfer

load-high:
	hey -z 60s -c 50 -q 2 -m POST -H "Content-Type: application/json" \
	  -d '{"users":1000,"amount_cents":100}' \
	  http://localhost:8080/v1/simulate/transfer

# Production protobuf generation path.
# This scaffold runs immediately with a grpc-go JSON codec, but keep this target
# when you want generated protobuf stubs from api/proto/wallet.proto.
proto:
	protoc --go_out=. --go-grpc_out=. api/proto/wallet.proto
