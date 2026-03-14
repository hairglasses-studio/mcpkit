package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

type jwtPayload struct {
	Sub string `json:"sub"`
	Iss string `json:"iss"`
	Aud any    `json:"aud"` // string or []string
	Exp int64  `json:"exp"`
	Nbf int64  `json:"nbf"`
	Iat int64  `json:"iat"`
}

func parseJWT(tokenStr string) (jwtHeader, jwtPayload, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return jwtHeader{}, jwtPayload{}, fmt.Errorf("expected 3 parts, got %d", len(parts))
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, jwtPayload{}, fmt.Errorf("decode header: %w", err)
	}

	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return jwtHeader{}, jwtPayload{}, fmt.Errorf("parse header: %w", err)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, jwtPayload{}, fmt.Errorf("decode payload: %w", err)
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return jwtHeader{}, jwtPayload{}, fmt.Errorf("parse payload: %w", err)
	}

	return header, payload, nil
}

func verifyRS256(tokenStr string, key *rsa.PublicKey) error {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	hash := sha256.Sum256([]byte(signingInput))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signature)
}
