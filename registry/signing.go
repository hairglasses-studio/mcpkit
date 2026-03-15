//go:build !official_sdk

package registry

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ToolSignature records a cryptographic signature over a tool's fingerprint.
type ToolSignature struct {
	ToolName    string    `json:"tool_name"`
	Fingerprint string    `json:"fingerprint"` // hex-encoded SHA-256
	Signature   []byte    `json:"signature"`   // Ed25519 signature over fingerprint bytes
	SignedAt    time.Time `json:"signed_at"`
	SignerID    string    `json:"signer_id"`
}

// SignTool computes the fingerprint of td and signs it with the given Ed25519
// private key. The returned ToolSignature can be stored and later verified.
func SignTool(td ToolDefinition, privateKey ed25519.PrivateKey, signerID string) ToolSignature {
	fp := Fingerprint(td)
	fpHex := hex.EncodeToString(fp[:])

	sig := ed25519.Sign(privateKey, fp[:])

	return ToolSignature{
		ToolName:    td.Tool.Name,
		Fingerprint: fpHex,
		Signature:   sig,
		SignedAt:    time.Now().UTC(),
		SignerID:    signerID,
	}
}

// VerifyToolSignature checks that sig matches td's current fingerprint and
// that the Ed25519 signature is valid under publicKey.
// Returns nil on success, or an error describing the failure.
func VerifyToolSignature(td ToolDefinition, sig ToolSignature, publicKey ed25519.PublicKey) error {
	fp := Fingerprint(td)
	fpHex := hex.EncodeToString(fp[:])

	if fpHex != sig.Fingerprint {
		return fmt.Errorf("signing: fingerprint mismatch for tool %q (current %s, signed %s)",
			td.Tool.Name, fpHex, sig.Fingerprint)
	}

	if !ed25519.Verify(publicKey, fp[:], sig.Signature) {
		return fmt.Errorf("signing: invalid signature for tool %q", td.Tool.Name)
	}

	return nil
}

// SignatureStore is a thread-safe store of tool signatures and their
// associated public keys. It supports batch signing and per-tool verification.
type SignatureStore struct {
	mu         sync.RWMutex
	signatures map[string]ToolSignature    // tool name -> signature
	publicKeys map[string]ed25519.PublicKey // signer ID -> public key
}

// NewSignatureStore creates an empty SignatureStore.
func NewSignatureStore() *SignatureStore {
	return &SignatureStore{
		signatures: make(map[string]ToolSignature),
		publicKeys: make(map[string]ed25519.PublicKey),
	}
}

// AddPublicKey registers a public key for a signer ID.
func (s *SignatureStore) AddPublicKey(signerID string, key ed25519.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publicKeys[signerID] = key
}

// AddSignature stores a signature for a tool.
func (s *SignatureStore) AddSignature(sig ToolSignature) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signatures[sig.ToolName] = sig
}

// Verify checks a tool definition against its stored signature.
// Returns nil if the tool has no stored signature (permissive by default).
// Returns an error if the signature is invalid or the signer's public key
// is not registered.
func (s *SignatureStore) Verify(td ToolDefinition) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sig, hasSig := s.signatures[td.Tool.Name]
	if !hasSig {
		return nil // unsigned tool — no opinion
	}

	pubKey, hasKey := s.publicKeys[sig.SignerID]
	if !hasKey {
		return fmt.Errorf("signing: no public key registered for signer %q", sig.SignerID)
	}

	return VerifyToolSignature(td, sig, pubKey)
}

// SignAll batch-signs all tools in the given registry and stores the
// signatures. The signer's public key is automatically registered.
func (s *SignatureStore) SignAll(reg *ToolRegistry, privateKey ed25519.PrivateKey, signerID string) {
	publicKey := privateKey.Public().(ed25519.PublicKey)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.publicKeys[signerID] = publicKey

	for _, td := range reg.GetAllToolDefinitions() {
		fp := Fingerprint(td)
		fpHex := hex.EncodeToString(fp[:])
		sig := ed25519.Sign(privateKey, fp[:])

		s.signatures[td.Tool.Name] = ToolSignature{
			ToolName:    td.Tool.Name,
			Fingerprint: fpHex,
			Signature:   sig,
			SignedAt:    time.Now().UTC(),
			SignerID:    signerID,
		}
	}
}
