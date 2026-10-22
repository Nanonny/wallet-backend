package application

import "context"

type subjectKey struct{}

// WithSubject attaches the authenticated username to the request context.
// Transport middleware (HTTP / gRPC) sets it after token validation; the
// application layer then enforces wallet ownership against it.
func WithSubject(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, subjectKey{}, username)
}

// WithoutSubject strips any subject from the context. Used by internal
// system operations (load simulation) that must not be constrained by a
// single account's ownership.
func WithoutSubject(ctx context.Context) context.Context {
	return context.WithValue(ctx, subjectKey{}, "")
}

// SubjectFromContext returns the authenticated username, or "" when the
// request did not carry an authenticated subject.
func SubjectFromContext(ctx context.Context) string {
	subject, _ := ctx.Value(subjectKey{}).(string)
	return subject
}
