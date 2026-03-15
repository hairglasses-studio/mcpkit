package secrets

import (
	"testing"
	"time"
)

func TestSecret_IsExpired(t *testing.T) {
	t.Run("zero ExpiresAt is not expired", func(t *testing.T) {
		s := &Secret{Key: "k", Value: "v"}
		if s.IsExpired() {
			t.Error("expected zero ExpiresAt to not be expired")
		}
	})

	t.Run("past time is expired", func(t *testing.T) {
		s := &Secret{Key: "k", Value: "v", ExpiresAt: time.Now().Add(-time.Second)}
		if !s.IsExpired() {
			t.Error("expected past ExpiresAt to be expired")
		}
	})

	t.Run("future time is not expired", func(t *testing.T) {
		s := &Secret{Key: "k", Value: "v", ExpiresAt: time.Now().Add(time.Hour)}
		if s.IsExpired() {
			t.Error("expected future ExpiresAt to not be expired")
		}
	})
}

func TestSecret_Masked(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"empty string", "", "****"},
		{"one char", "a", "****"},
		{"four chars", "abcd", "****"},
		{"five chars", "abcde", "ab****de"},
		{"long value", "mysecretpassword", "my****rd"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Secret{Key: "k", Value: tc.value}
			got := s.Masked()
			if got != tc.want {
				t.Errorf("Masked() = %q, want %q", got, tc.want)
			}
		})
	}
}
