# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN if [ -z "$TARGETARCH" ]; then \
      CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /wallet-api ./cmd/wallet-api; \
    else \
      CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /wallet-api ./cmd/wallet-api; \
    fi

FROM alpine:3.22

WORKDIR /app

RUN apk upgrade --no-cache \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

COPY --from=builder /wallet-api /app/wallet-api

USER 10001:10001

EXPOSE 8080 50051 9090

CMD ["/app/wallet-api"]
