package auth

import (
	"context"
	"testing"
)

func TestSubject_RoundTrip(t *testing.T) {
	ctx := WithSubject(context.Background(), "alice")
	got := Subject(ctx)
	if got != "alice" {
		t.Errorf("Subject() = %q, want %q", got, "alice")
	}
}

func TestWithSubject_Override(t *testing.T) {
	ctx := WithSubject(context.Background(), "alice")
	ctx = WithSubject(ctx, "bob")
	got := Subject(ctx)
	if got != "bob" {
		t.Errorf("Subject() after override = %q, want %q", got, "bob")
	}
}

func TestSubject_EmptyContext(t *testing.T) {
	got := Subject(context.Background())
	if got != "" {
		t.Errorf("Subject() on empty context = %q, want %q", got, "")
	}
}
