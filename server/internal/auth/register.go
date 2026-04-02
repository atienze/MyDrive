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

// GenerateUUID produces a random UUID v4 string using crypto/rand.
// The format is lowercase hyphenated: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
// where 4 is the version nibble and y is the variant bits.
func GenerateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	// Set UUID v4 version bits.
	b[6] = (b[6] & 0x0f) | 0x40
	// Set UUID variant bits (RFC 4122).
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	), nil
}
