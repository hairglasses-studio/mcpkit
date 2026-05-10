package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrSwarmPeerNotFound is returned when a unicast target is not registered.
	ErrSwarmPeerNotFound = errors.New("orchestrator: swarm peer not found")
	// ErrSwarmDuplicatePeer is returned when two peers share an ID.
	ErrSwarmDuplicatePeer = errors.New("orchestrator: duplicate swarm peer")
	// ErrSwarmInvalidPeer is returned when a peer has no ID or handler.
	ErrSwarmInvalidPeer = errors.New("orchestrator: invalid swarm peer")
)

// SwarmHandler handles one routed message.
type SwarmHandler func(ctx context.Context, message SwarmMessage) (SwarmMessage, error)

// SwarmPeer is one addressable peer in a swarm mesh.
type SwarmPeer struct {
	ID      string
	Handler SwarmHandler
}

// SwarmMessage is routed between swarm peers.
type SwarmMessage struct {
	From     string            `json:"from,omitempty"`
	To       string            `json:"to,omitempty"`
	Topic    string            `json:"topic,omitempty"`
	Data     map[string]any    `json:"data,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SwarmConfig configures broadcast and unicast execution.
type SwarmConfig struct {
	MaxConcurrency int
	Timeout        time.Duration
	FailFast       bool
}

// SwarmResult captures routed message responses.
type SwarmResult struct {
	Pattern    string            `json:"pattern"`
	Responses  []SwarmMessage    `json:"responses"`
	Errors     map[string]string `json:"errors,omitempty"`
	Duration   time.Duration     `json:"duration_ns"`
	PeerCount  int               `json:"peer_count"`
	TargetPeer string            `json:"target_peer,omitempty"`
}

// Swarm is an addressable peer-to-peer mesh.
type Swarm struct {
	peers []SwarmPeer
	index map[string]SwarmPeer
}

// NewSwarm creates a swarm mesh with unique peer IDs.
func NewSwarm(peers ...SwarmPeer) (*Swarm, error) {
	s := &Swarm{
		peers: make([]SwarmPeer, 0, len(peers)),
		index: make(map[string]SwarmPeer, len(peers)),
	}
	for _, peer := range peers {
		if peer.ID == "" || peer.Handler == nil {
			return nil, ErrSwarmInvalidPeer
		}
		if _, exists := s.index[peer.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrSwarmDuplicatePeer, peer.ID)
		}
		s.peers = append(s.peers, peer)
		s.index[peer.ID] = peer
	}
	return s, nil
}

// Broadcast routes message to every peer. Each peer receives a copy with To set
// to that peer ID.
func (s *Swarm) Broadcast(ctx context.Context, message SwarmMessage, config ...SwarmConfig) (*SwarmResult, error) {
	cfg := swarmConfig(config...)
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	start := time.Now()
	result := &SwarmResult{
		Pattern:   "swarm-broadcast",
		Responses: make([]SwarmMessage, 0, len(s.peers)),
		Errors:    make(map[string]string),
		PeerCount: len(s.peers),
	}
	if len(s.peers) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	responses := make([]SwarmMessage, len(s.peers))
	var mu sync.Mutex
	var firstErr error
	sem := make(chan struct{}, maxConcurrency(cfg.MaxConcurrency, len(s.peers)))
	var wg sync.WaitGroup

	for i, peer := range s.peers {
		wg.Add(1)
		go func(idx int, peer SwarmPeer) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-runCtx.Done():
				mu.Lock()
				result.Errors[peer.ID] = runCtx.Err().Error()
				mu.Unlock()
				return
			}

			routed := copySwarmMessage(message)
			routed.To = peer.ID
			response, err := peer.Handler(runCtx, routed)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Errors[peer.ID] = err.Error()
				if cfg.FailFast && firstErr == nil {
					firstErr = err
					cancel()
				}
				return
			}
			if response.From == "" {
				response.From = peer.ID
			}
			responses[idx] = response
		}(i, peer)
	}
	wg.Wait()

	for _, response := range responses {
		if response.From != "" || response.To != "" || response.Topic != "" || len(response.Data) > 0 || len(response.Metadata) > 0 {
			result.Responses = append(result.Responses, response)
		}
	}
	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	result.Duration = time.Since(start)
	return result, firstErr
}

// Unicast routes message to one target peer.
func (s *Swarm) Unicast(ctx context.Context, target string, message SwarmMessage, config ...SwarmConfig) (*SwarmResult, error) {
	peer, ok := s.index[target]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSwarmPeerNotFound, target)
	}
	cfg := swarmConfig(config...)
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	start := time.Now()
	routed := copySwarmMessage(message)
	routed.To = target
	response, err := peer.Handler(ctx, routed)
	result := &SwarmResult{
		Pattern:    "swarm-unicast",
		PeerCount:  1,
		TargetPeer: target,
		Duration:   time.Since(start),
	}
	if err != nil {
		result.Errors = map[string]string{target: err.Error()}
		return result, err
	}
	if response.From == "" {
		response.From = target
	}
	result.Responses = []SwarmMessage{response}
	return result, nil
}

func swarmConfig(config ...SwarmConfig) SwarmConfig {
	if len(config) == 0 {
		return SwarmConfig{}
	}
	return config[0]
}

func copySwarmMessage(message SwarmMessage) SwarmMessage {
	out := message
	if message.Data != nil {
		out.Data = make(map[string]any, len(message.Data))
		for key, value := range message.Data {
			out.Data[key] = value
		}
	}
	if message.Metadata != nil {
		out.Metadata = make(map[string]string, len(message.Metadata))
		for key, value := range message.Metadata {
			out.Metadata[key] = value
		}
	}
	return out
}
