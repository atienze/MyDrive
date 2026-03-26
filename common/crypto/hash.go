package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// CalculateFileHash reads the file at filePath and returns its SHA-256 hash
// as a lowercase hex string. The file is streamed in chunks so large files
// do not need to be loaded into memory all at once.
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}
