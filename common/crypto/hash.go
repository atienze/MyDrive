package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// CalculateFileHash reads a file and returns its SHA-256 fingerprint
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Create a new SHA256 hasher
	hasher := sha256.New()

	// Copy the file content into the hasher efficiently
	// io.Copy streams it, so we don't load the whole file into RAM
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	// Finalize the hash and turn it into a string
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}