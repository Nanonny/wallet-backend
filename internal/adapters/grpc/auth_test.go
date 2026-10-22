package grpcadapter

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type fakeValidator struct {
	token   string
	subject string
	err     error
}

func (f *fakeValidator) Validate(token string) (string, error) {
	if token != f.token {
		return "", f.err
	}
	return f.subject, nil
}

func interceptorCall(t *testing.T, validator TokenValidator, ctx context.Context) (bool, error) {
	t.Helper()
	interceptor := AuthUnaryServerInterceptor(validator)
	handled := false
	_, interceptorErr := interceptor(ctx, struct{}{}, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
		handled = true
		return "ok", nil
	})
	return handled, interceptorErr
}

func TestGRPCAuthRejectsMissingToken(t *testing.T) {
	_, err := interceptorCall(t, &fakeValidator{token: "tok-123", subject: "demo", err: errors.New("bad")}, context.Background())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("err = %v, want codes.Unauthenticated", err)
	}
}

func TestGRPCAuthRejectsInvalidToken(t *testing.T) {
	md := metadata.Pairs("authorization", "Bearer wrong-token")
	_, err := interceptorCall(t, &fakeValidator{token: "tok-123", subject: "demo", err: errors.New("bad")},
		metadata.NewIncomingContext(context.Background(), md))
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("err = %v, want codes.Unauthenticated", err)
	}
}

func TestGRPCAuthAcceptsValidBearerToken(t *testing.T) {
	validator := &fakeValidator{token: "tok-123", subject: "demo"}
	md := metadata.Pairs("authorization", "Bearer tok-123")
	handled, err := interceptorCall(t, validator, metadata.NewIncomingContext(context.Background(), md))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("handler did not run for a valid token")
	}
}

func TestGRPCAuthAcceptsRawTokenMetadata(t *testing.T) {
	validator := &fakeValidator{token: "tok-123", subject: "demo"}
	md := metadata.Pairs("grpc-token", "tok-123")
	_, err := interceptorCall(t, validator, metadata.NewIncomingContext(context.Background(), md))
	if err != nil {
		t.Fatalf("raw token via grpc-token metadata should validate: %v", err)
	}
}
