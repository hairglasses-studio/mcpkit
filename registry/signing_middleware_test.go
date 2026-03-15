//go:build !official_sdk

package registry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func TestSignatureVerificationMiddleware_ValidSignature(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	td := makeTD("my_tool", "desc")
	store := NewSignatureStore()
	store.AddPublicKey("signer", pub)
	store.AddSignature(SignTool(td, priv, "signer"))

	nextCalled := false
	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		nextCalled = true
		return MakeTextResult("ok"), nil
	}

	onFailure := func(name string, err error) error {
		return errors.New("should not be called")
	}

	mw := SignatureVerificationMiddleware(store, onFailure)
	handler := mw("my_tool", td, next)
	result, err := handler(context.Background(), CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextCalled {
		t.Error("next should have been called")
	}
	if IsResultError(result) {
		t.Error("result should not be an error")
	}
}

func TestSignatureVerificationMiddleware_InvalidBlocking(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	original := makeTD("my_tool", "original")
	tampered := makeTD("my_tool", "tampered")

	store := NewSignatureStore()
	store.AddPublicKey("signer", pub)
	store.AddSignature(SignTool(original, priv, "signer"))

	nextCalled := false
	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		nextCalled = true
		return MakeTextResult("should not reach"), nil
	}

	onFailure := func(name string, err error) error {
		return errors.New("blocked")
	}

	mw := SignatureVerificationMiddleware(store, onFailure)
	handler := mw("my_tool", tampered, next)
	result, err := handler(context.Background(), CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextCalled {
		t.Error("next should NOT have been called")
	}
	if !IsResultError(result) {
		t.Error("expected error result")
	}
}

func TestSignatureVerificationMiddleware_WarningOnly(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	original := makeTD("my_tool", "original")
	tampered := makeTD("my_tool", "tampered")

	store := NewSignatureStore()
	store.AddPublicKey("signer", pub)
	store.AddSignature(SignTool(original, priv, "signer"))

	failureSeen := false
	next := func(_ context.Context, _ CallToolRequest) (*CallToolResult, error) {
		return MakeTextResult("proceeded"), nil
	}

	onFailure := func(name string, err error) error {
		failureSeen = true
		return nil // warning only
	}

	mw := SignatureVerificationMiddleware(store, onFailure)
	handler := mw("my_tool", tampered, next)
	result, err := handler(context.Background(), CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !failureSeen {
		t.Error("onFailure should have been called")
	}
	if IsResultError(result) {
		t.Error("result should not be an error in warning-only mode")
	}
	if text, _ := ExtractTextContent(result.Content[0]); text != "proceeded" {
		t.Errorf("expected 'proceeded', got %q", text)
	}
}
