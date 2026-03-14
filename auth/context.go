package auth

import "context"

type subjectKey struct{}

// WithSubject returns a context with the authenticated subject.
func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

// Subject returns the authenticated subject from the context.
func Subject(ctx context.Context) string {
	s, _ := ctx.Value(subjectKey{}).(string)
	return s
}
