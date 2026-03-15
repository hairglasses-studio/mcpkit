//go:build !official_sdk

package registry

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func generateTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func TestSignTool_RoundTrip(t *testing.T) {
	t.Parallel()
	pub, priv := generateTestKey(t)

	td := makeTD("my_tool", "does stuff")
	sig := SignTool(td, priv, "deployer")

	if err := VerifyToolSignature(td, sig, pub); err != nil {
		t.Fatalf("round-trip verification failed: %v", err)
	}
	if sig.ToolName != "my_tool" {
		t.Errorf("ToolName = %q, want my_tool", sig.ToolName)
	}
	if sig.SignerID != "deployer" {
		t.Errorf("SignerID = %q, want deployer", sig.SignerID)
	}
	if sig.SignedAt.IsZero() {
		t.Error("SignedAt should not be zero")
	}
}

func TestVerifyToolSignature_TamperedDescription(t *testing.T) {
	t.Parallel()
	pub, priv := generateTestKey(t)

	original := makeTD("my_tool", "original")
	sig := SignTool(original, priv, "signer")

	tampered := makeTD("my_tool", "tampered description")
	err := VerifyToolSignature(tampered, sig, pub)
	if err == nil {
		t.Fatal("expected error for tampered description")
	}
}

func TestVerifyToolSignature_WrongKey(t *testing.T) {
	t.Parallel()
	_, priv := generateTestKey(t)
	otherPub, _ := generateTestKey(t)

	td := makeTD("my_tool", "desc")
	sig := SignTool(td, priv, "signer")

	err := VerifyToolSignature(td, sig, otherPub)
	if err == nil {
		t.Fatal("expected error for wrong public key")
	}
}

func TestSignatureStore_UnsignedTool(t *testing.T) {
	t.Parallel()
	store := NewSignatureStore()
	td := makeTD("unknown_tool", "desc")

	if err := store.Verify(td); err != nil {
		t.Fatalf("unsigned tool should return nil, got: %v", err)
	}
}

func TestSignatureStore_MissingPublicKey(t *testing.T) {
	t.Parallel()
	_, priv := generateTestKey(t)

	store := NewSignatureStore()
	td := makeTD("my_tool", "desc")
	sig := SignTool(td, priv, "unknown-signer")
	store.AddSignature(sig)
	// Note: NOT adding the public key

	err := store.Verify(td)
	if err == nil {
		t.Fatal("expected error when public key is missing")
	}
}

func TestSignatureStore_SignAll(t *testing.T) {
	t.Parallel()
	_, priv := generateTestKey(t)

	reg := NewToolRegistry()
	reg.RegisterModule(&testSigningModule{
		tools: []ToolDefinition{
			makeTD("tool_a", "first"),
			makeTD("tool_b", "second"),
			makeTD("tool_c", "third"),
		},
	})

	store := NewSignatureStore()
	store.SignAll(reg, priv, "batch-signer")

	for _, name := range []string{"tool_a", "tool_b", "tool_c"} {
		td, _ := reg.GetTool(name)
		if err := store.Verify(td); err != nil {
			t.Errorf("Verify(%q) failed: %v", name, err)
		}
	}
}

// testSigningModule implements ToolModule for signing tests.
type testSigningModule struct {
	tools []ToolDefinition
}

func (m *testSigningModule) Name() string            { return "signing-test" }
func (m *testSigningModule) Description() string     { return "test module" }
func (m *testSigningModule) Tools() []ToolDefinition { return m.tools }
