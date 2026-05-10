package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSwarmBroadcast(t *testing.T) {
	t.Parallel()

	peer := func(id string) SwarmPeer {
		return SwarmPeer{
			ID: id,
			Handler: func(_ context.Context, msg SwarmMessage) (SwarmMessage, error) {
				if msg.To != id {
					t.Fatalf("message To = %q, want %q", msg.To, id)
				}
				return SwarmMessage{
					From:  id,
					Topic: msg.Topic,
					Data:  map[string]any{"seen": msg.Data["value"]},
				}, nil
			},
		}
	}
	swarm, err := NewSwarm(peer("a"), peer("b"))
	if err != nil {
		t.Fatalf("NewSwarm: %v", err)
	}

	result, err := swarm.Broadcast(context.Background(), SwarmMessage{
		From:  "manager",
		Topic: "score",
		Data:  map[string]any{"value": 10},
	})
	if err != nil {
		t.Fatalf("Broadcast: %v", err)
	}
	if result.Pattern != "swarm-broadcast" || result.PeerCount != 2 {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	if len(result.Responses) != 2 {
		t.Fatalf("responses len = %d, want 2", len(result.Responses))
	}
	if result.Responses[0].From != "a" || result.Responses[1].From != "b" {
		t.Fatalf("responses not in peer order: %+v", result.Responses)
	}
	if result.Errors != nil {
		t.Fatalf("unexpected errors: %+v", result.Errors)
	}
}

func TestSwarmUnicast(t *testing.T) {
	t.Parallel()

	swarm, err := NewSwarm(SwarmPeer{
		ID: "worker",
		Handler: func(_ context.Context, msg SwarmMessage) (SwarmMessage, error) {
			return SwarmMessage{Topic: msg.Topic, Data: map[string]any{"target": msg.To}}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSwarm: %v", err)
	}

	result, err := swarm.Unicast(context.Background(), "worker", SwarmMessage{Topic: "plan"})
	if err != nil {
		t.Fatalf("Unicast: %v", err)
	}
	if result.Pattern != "swarm-unicast" || result.TargetPeer != "worker" {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	if got := result.Responses[0].Data["target"]; got != "worker" {
		t.Fatalf("target data = %v", got)
	}
	if result.Responses[0].From != "worker" {
		t.Fatalf("response From = %q", result.Responses[0].From)
	}
}

func TestSwarmValidation(t *testing.T) {
	t.Parallel()

	if _, err := NewSwarm(SwarmPeer{}); !errors.Is(err, ErrSwarmInvalidPeer) {
		t.Fatalf("invalid peer error = %v", err)
	}
	handler := func(context.Context, SwarmMessage) (SwarmMessage, error) { return SwarmMessage{}, nil }
	if _, err := NewSwarm(
		SwarmPeer{ID: "dup", Handler: handler},
		SwarmPeer{ID: "dup", Handler: handler},
	); !errors.Is(err, ErrSwarmDuplicatePeer) {
		t.Fatalf("duplicate peer error = %v", err)
	}
}

func TestSwarmUnicastMissingPeer(t *testing.T) {
	t.Parallel()

	swarm, err := NewSwarm()
	if err != nil {
		t.Fatalf("NewSwarm: %v", err)
	}
	_, err = swarm.Unicast(context.Background(), "missing", SwarmMessage{})
	if !errors.Is(err, ErrSwarmPeerNotFound) {
		t.Fatalf("missing peer error = %v", err)
	}
}

func TestSwarmBroadcastCollectsErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("worker failed")
	swarm, err := NewSwarm(
		SwarmPeer{ID: "ok", Handler: func(context.Context, SwarmMessage) (SwarmMessage, error) {
			return SwarmMessage{Data: map[string]any{"ok": true}}, nil
		}},
		SwarmPeer{ID: "bad", Handler: func(context.Context, SwarmMessage) (SwarmMessage, error) {
			return SwarmMessage{}, wantErr
		}},
	)
	if err != nil {
		t.Fatalf("NewSwarm: %v", err)
	}

	result, err := swarm.Broadcast(context.Background(), SwarmMessage{})
	if err != nil {
		t.Fatalf("Broadcast without fail-fast should not return peer error: %v", err)
	}
	if result.Errors["bad"] != wantErr.Error() {
		t.Fatalf("errors = %+v", result.Errors)
	}
	if len(result.Responses) != 1 || result.Responses[0].From != "ok" {
		t.Fatalf("responses = %+v", result.Responses)
	}
}

func TestSwarmBroadcastFailFast(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("stop")
	swarm, err := NewSwarm(
		SwarmPeer{ID: "bad", Handler: func(context.Context, SwarmMessage) (SwarmMessage, error) {
			return SwarmMessage{}, wantErr
		}},
		SwarmPeer{ID: "slow", Handler: func(ctx context.Context, _ SwarmMessage) (SwarmMessage, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return SwarmMessage{From: "slow"}, nil
			case <-ctx.Done():
				return SwarmMessage{}, ctx.Err()
			}
		}},
	)
	if err != nil {
		t.Fatalf("NewSwarm: %v", err)
	}

	result, err := swarm.Broadcast(context.Background(), SwarmMessage{}, SwarmConfig{FailFast: true})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Broadcast error = %v, want %v", err, wantErr)
	}
	if result.Errors["bad"] != wantErr.Error() {
		t.Fatalf("errors = %+v", result.Errors)
	}
}

func TestCopySwarmMessageIsIndependent(t *testing.T) {
	t.Parallel()

	original := SwarmMessage{
		Data:     map[string]any{"k": "v"},
		Metadata: map[string]string{"m": "v"},
	}
	copied := copySwarmMessage(original)
	copied.Data["k"] = "changed"
	copied.Metadata["m"] = "changed"
	if original.Data["k"] != "v" || original.Metadata["m"] != "v" {
		t.Fatalf("copy mutated original: %+v", original)
	}
}
