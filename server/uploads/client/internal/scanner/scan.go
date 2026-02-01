package scanner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/atienze/HomelabSecureSync/common/crypto"
)

// FileMeta represents one file we found
type FileMeta struct {
	Path string // "Folder/resume.pdf"
	Hash string // "a1b2c3d4..."
	Size int64  // Bytes
}

// ScanDirectory walks through a folder and fingerprints every file
func ScanDirectory(rootPath string) ([]FileMeta, error) {
	var files []FileMeta
	fmt.Printf("Scanning directory: %s\n", rootPath)

	// filepath.WalkDir is a standard Go tool that recursively visits every subfolder
	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 1. IGNORE DIRECTORIES
		if d.IsDir() {
			// Skip the ".git" folder (too much noise)
			// Skip the "server" folder (prevent infinite recursion loop)
			if d.Name() == ".git" || d.Name() == "server" {
				return filepath.SkipDir
			}
			return nil
		}

		// 2. IGNORE SPECIFIC FILES
		if d.Name() == ".DS_Store" {
			return nil // Just skip this single file
		}

		// 3. HASH LOGIC
		// Calculate the Hash (Using the tool we built in 'common')
		hash, err := crypto.CalculateFileHash(path)
		if err != nil {
			fmt.Printf("Failed to hash %s: %v\n", path, err)
			return nil // Skip this file, keep going
		}

		// Get file size
		info, _ := d.Info()

		// Add to our list
		// We want the path to be relative (e.g., "resume.pdf", not "/Users/elijah/...")
		relPath, _ := filepath.Rel(rootPath, path)

		files = append(files, FileMeta{
			Path: relPath,
			Hash: hash,
			Size: info.Size(),
		})

		fmt.Printf("Found: %s (%s)\n", relPath, hash[:8]) // Print first 8 chars of hash
		return nil
	})

	return files, err
}