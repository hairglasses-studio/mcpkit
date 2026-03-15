package gateway

import "errors"

var (
	// ErrUpstreamNotFound is returned when an upstream name is not registered.
	ErrUpstreamNotFound = errors.New("gateway: upstream not found")

	// ErrUpstreamUnhealthy is returned when a call targets an unhealthy upstream.
	ErrUpstreamUnhealthy = errors.New("gateway: upstream unhealthy")

	// ErrDuplicateUpstream is returned when adding an upstream with a name already in use.
	ErrDuplicateUpstream = errors.New("gateway: duplicate upstream name")

	// ErrGatewayClosed is returned when operations are attempted on a closed gateway.
	ErrGatewayClosed = errors.New("gateway: closed")
)
