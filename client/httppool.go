package client

import (
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	fast     *http.Client
	fastOnce sync.Once

	standard     *http.Client
	standardOnce sync.Once

	slow     *http.Client
	slowOnce sync.Once
)

// sharedTransport is used by all pooled clients for maximum connection reuse.
var sharedTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
}

// Fast returns a shared client with a 5-second timeout.
// Intended for LAN/local services.
func Fast() *http.Client {
	fastOnce.Do(func() {
		fast = &http.Client{
			Timeout:   5 * time.Second,
			Transport: sharedTransport,
		}
	})
	return fast
}

// Standard returns a shared client with a 30-second timeout.
// Intended for cloud APIs.
func Standard() *http.Client {
	standardOnce.Do(func() {
		standard = &http.Client{
			Timeout:   30 * time.Second,
			Transport: sharedTransport,
		}
	})
	return standard
}

// Slow returns a shared client with a 2-minute timeout.
// Intended for downloads, uploads, and long-running API calls.
func Slow() *http.Client {
	slowOnce.Do(func() {
		slow = &http.Client{
			Timeout:   2 * time.Minute,
			Transport: sharedTransport,
		}
	})
	return slow
}

// WithTimeout returns a new client using the shared transport with a custom timeout.
// The returned client shares the connection pool with all other pooled clients.
func WithTimeout(d time.Duration) *http.Client {
	return &http.Client{
		Timeout:   d,
		Transport: sharedTransport,
	}
}
