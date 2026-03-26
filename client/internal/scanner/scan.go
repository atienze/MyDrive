package scanner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/atienze/HomelabSecureSync/common/crypto"
)

// FileMeta represents a single file discovered during a directory scan,
// including its relative path, SHA-256 hash, and size in bytes.
type FileMeta struct {
	Path string // relative path within the sync directory, e.g. "folder/resume.pdf"
	Hash string // SHA-256 hex digest of the file content
	Size int64  // file size in bytes
}

// ScanDirectory walks through a folder and fingerprints every file (verbose).
func ScanDirectory(rootPath string) ([]FileMeta, error) {
	return scanDirectory(rootPath, true)
}

// ScanDirectoryQuiet walks through a folder and fingerprints every file
// without printing to stdout. Used by HTTP API handlers to avoid flooding
// the daemon's terminal output on every poll.
func ScanDirectoryQuiet(rootPath string) ([]FileMeta, error) {
	return scanDirectory(rootPath, false)
}

func scanDirectory(rootPath string, verbose bool) ([]FileMeta, error) {
	var files []FileMeta
	if verbose {
		fmt.Printf("Scanning directory: %s\n", rootPath)
	}

	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "server" {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() == ".DS_Store" {
			return nil
		}

		hash, err := crypto.CalculateFileHash(path)
		if err != nil {
			if verbose {
				fmt.Printf("Failed to hash %s: %v\n", path, err)
			}
			return nil
		}

		info, _ := d.Info()
		relPath, _ := filepath.Rel(rootPath, path)

		files = append(files, FileMeta{
			Path: relPath,
			Hash: hash,
			Size: info.Size(),
		})

		if verbose {
			fmt.Printf("Found: %s (%s)\n", relPath, hash[:8])
		}
		return nil
	})

	return files, err
}
