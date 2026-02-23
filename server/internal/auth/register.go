package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateToken produces a cryptographically random 64-character hex string.
// It reads 32 bytes from the OS CSPRNG and hex-encodes them.
// Each call is guaranteed to produce a statistically unique token — safe to
// call multiple times for the same device name.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
