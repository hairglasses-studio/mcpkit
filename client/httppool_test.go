package client

import (
	"net/http"
	"testing"
	"time"
)

func TestPooledClients(t *testing.T) {
	tests := []struct {
		name    string
		getter  func() *http.Client
		timeout time.Duration
	}{
		{"Fast", Fast, 5 * time.Second},
		{"Standard", Standard, 30 * time.Second},
		{"Slow", Slow, 2 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.getter()
			if c == nil {
				t.Fatal("client is nil")
			}
			if c.Timeout != tt.timeout {
				t.Errorf("timeout = %v, want %v", c.Timeout, tt.timeout)
			}
			if c2 := tt.getter(); c2 != c {
				t.Error("second call returned different instance")
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	c := WithTimeout(10 * time.Minute)
	if c.Timeout != 10*time.Minute {
		t.Errorf("timeout = %v, want 10m", c.Timeout)
	}
	if c.Transport != sharedTransport {
		t.Error("should use shared transport")
	}
}
