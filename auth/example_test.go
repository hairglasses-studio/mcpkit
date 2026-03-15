//go:build !official_sdk

package auth_test

import (
	"fmt"
	"time"

	"github.com/hairglasses-studio/mcpkit/auth"
)

func ExampleNewDPoPValidator() {
	v := auth.NewDPoPValidator(auth.DPoPConfig{
		MaxClockSkew: 30 * time.Second,
		MaxProofAge:  60 * time.Second,
	})

	// ValidateProof returns an error for a malformed proof.
	err := v.ValidateProof("bad.proof.jwt", "", "GET", "https://api.example.com/tools")
	fmt.Println(err != nil)
	// Output:
	// true
}

func ExampleNewJWKSValidator() {
	v := auth.NewJWKSValidator(auth.JWKSConfig{
		JWKSURL:  "https://auth.example.com/.well-known/jwks.json",
		CacheTTL: 30 * time.Minute,
	})

	// TokenValidator returns a function for use with Middleware.
	validator := v.TokenValidator()
	_, err := validator("not-a-real-token")
	fmt.Println(err != nil)
	// Output:
	// true
}
