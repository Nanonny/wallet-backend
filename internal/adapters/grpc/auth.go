package grpcadapter

import (
	"context"
	"strings"

	"github.com/anon/wallet-devops-lab/internal/application"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TokenValidator verifies a raw bearer token and returns the subject
// username. application.TokenService satisfies this interface.
type TokenValidator interface {
	Validate(token string) (string, error)
}

// AuthUnaryServerInterceptor enforces bearer-token authentication on every
// gRPC call and attaches the authenticated subject to the context so the
// application layer can enforce wallet ownership.
func AuthUnaryServerInterceptor(validator TokenValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		subject, ok := extractSubject(ctx, validator)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid bearer token")
		}
		ctx = application.WithSubject(ctx, subject)
		return handler(ctx, req)
	}
}

func extractSubject(ctx context.Context, validator TokenValidator) (string, bool) {
	md, hasMD := metadata.FromIncomingContext(ctx)
	if !hasMD {
		return "", false
	}
	for _, key := range []string{"authorization", "grpc-token", "x-wallet-token"} {
		for _, value := range md.Get(key) {
			token := strings.TrimSpace(value)
			const bearerPrefix = "Bearer "
			if len(token) > len(bearerPrefix) && strings.EqualFold(token[:len(bearerPrefix)], bearerPrefix) {
				token = strings.TrimSpace(token[len(bearerPrefix):])
			}
			if token == "" {
				continue
			}
			subject, err := validator.Validate(token)
			if err == nil {
				return subject, true
			}
		}
	}
	return "", false
}
