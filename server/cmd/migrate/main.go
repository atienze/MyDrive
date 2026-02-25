package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/atienze/HomelabSecureSync/common/crypto"
	"github.com/atienze/HomelabSecureSync/server/internal/db"
	"github.com/atienze/HomelabSecureSync/server/internal/store"
)

const (
	OldUploadsDir = "./uploads"
	NewVaultDir   = "./VaultData"
	DatabasePath  = "./vaultsync.db"
)

func main() {
	fmt.Println("=== VaultSync Migration: path-based -> content-addressable storage ===")
	fmt.Printf("Source:      %s\n", OldUploadsDir)
	fmt.Printf("Destination: %s\n", NewVaultDir)
	fmt.Printf("Database:    %s\n", DatabasePath)
	fmt.Println()

	// Check if uploads directory exists
	if _, err := os.Stat(OldUploadsDir); os.IsNotExist(err) {
		fmt.Println("No uploads/ directory found. Nothing to migrate.")
		return
	}

	// Open database
	database, err := db.Open(DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create object store
	objectStore, err := store.New(NewVaultDir)
	if err != nil {
		log.Fatalf("Failed to initialize object store: %v", err)
	}

	// Phase 1: Migrate files that are tracked in the database
	fmt.Println("--- Phase 1: Migrating database-tracked files ---")
	dbFiles, err := database.GetAllFiles()
	if err != nil {
		log.Fatalf("Failed to get files from database: %v", err)
	}

	migratedCount := 0
	skippedCount := 0
	missingCount := 0

	for _, record := range dbFiles {
		oldPath := filepath.Join(OldUploadsDir, record.RelPath)

		// Check if already migrated (idempotent)
		if objectStore.HasObject(record.Hash) {
			// Object already in store — remove old file if it still exists
			if _, err := os.Stat(oldPath); err == nil {
				os.Remove(oldPath)
				fmt.Printf("  [cleanup] %s (already migrated)\n", record.RelPath)
			} else {
				fmt.Printf("  [skip]    %s (already migrated)\n", record.RelPath)
			}
			skippedCount++
			continue
		}

		// Check if old file exists
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			fmt.Printf("  [missing] %s (not on disk, not in object store)\n", record.RelPath)
			missingCount++
			continue
		}

		// Verify hash matches DB record
		computedHash, err := crypto.CalculateFileHash(oldPath)
		if err != nil {
			log.Printf("  [error]   %s: failed to hash: %v", record.RelPath, err)
			continue
		}

		if computedHash != record.Hash {
			log.Printf("  [warn]    %s: hash mismatch (db: %s, disk: %s) — using disk version",
				record.RelPath, record.Hash[:12], computedHash[:12])
			// Use the actual file hash; update DB record
			record.Hash = computedHash
			database.UpsertFile(record.RelPath, computedHash, record.DeviceID, record.Size)
		}

		// Read and store the object
		data, err := os.ReadFile(oldPath)
		if err != nil {
			log.Printf("  [error]   %s: failed to read: %v", record.RelPath, err)
			continue
		}

		if err := objectStore.WriteObject(record.Hash, data); err != nil {
			log.Printf("  [error]   %s: failed to write object: %v", record.RelPath, err)
			continue
		}

		// Remove the old file
		os.Remove(oldPath)
		fmt.Printf("  [migrated] %s -> %s\n", record.RelPath, record.Hash[:12])
		migratedCount++
	}

	// Phase 2: Find orphaned files (on disk but not in DB)
	fmt.Println("\n--- Phase 2: Checking for orphaned files ---")
	orphanCount := 0

	filepath.WalkDir(OldUploadsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(OldUploadsDir, path)
		if err != nil {
			return nil
		}

		// Compute hash
		hash, err := crypto.CalculateFileHash(path)
		if err != nil {
			log.Printf("  [error]   orphan %s: failed to hash: %v", relPath, err)
			return nil
		}

		// Store the object (idempotent)
		if !objectStore.HasObject(hash) {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("  [error]   orphan %s: failed to read: %v", relPath, err)
				return nil
			}
			if err := objectStore.WriteObject(hash, data); err != nil {
				log.Printf("  [error]   orphan %s: failed to write object: %v", relPath, err)
				return nil
			}
		}

		// Remove the old file
		os.Remove(path)
		fmt.Printf("  [orphan]  %s -> %s (stored but not tracked in DB)\n", relPath, hash[:12])
		orphanCount++
		return nil
	})

	// Phase 3: Clean up empty directories in uploads/
	fmt.Println("\n--- Phase 3: Cleaning up empty directories ---")
	cleanEmptyDirs(OldUploadsDir)

	// Summary
	fmt.Println("\n=== Migration Complete ===")
	fmt.Printf("  Migrated:  %d files\n", migratedCount)
	fmt.Printf("  Skipped:   %d files (already migrated)\n", skippedCount)
	fmt.Printf("  Missing:   %d files (in DB but not on disk)\n", missingCount)
	fmt.Printf("  Orphaned:  %d files (on disk but not in DB)\n", orphanCount)
}

// cleanEmptyDirs removes empty directories bottom-up inside root.
func cleanEmptyDirs(root string) {
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == root {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		if len(entries) == 0 {
			os.Remove(path)
			fmt.Printf("  [removed] empty dir: %s\n", path)
		}
		return nil
	})
}
