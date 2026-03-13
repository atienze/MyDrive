package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

const v2Schema = `
CREATE TABLE IF NOT EXISTS files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    rel_path    TEXT NOT NULL UNIQUE,
    hash        TEXT NOT NULL,
    size        INTEGER NOT NULL,
    device_id   TEXT NOT NULL,
    uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted     BOOLEAN DEFAULT FALSE
);
`

func createV2DB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}

	if _, err := db.Exec(v2Schema); err != nil {
		db.Close()
		t.Fatalf("failed to create v2 schema: %v", err)
	}

	return dbPath, db
}

// autoIndexColumnCount returns the number of columns in sqlite_autoindex_files_1.
// v2 schema has 1 column (rel_path), v3 has 2 columns (rel_path, device_id).
func autoIndexColumnCount(db *sql.DB) (int, error) {
	rows, err := db.Query("PRAGMA index_info(sqlite_autoindex_files_1)")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

// TestMigration verifies the basic migration path:
// creates a v2 DB, inserts 2 rows for different devices, runs migration,
// confirms the old unique index is gone and both rows are preserved.
func TestMigration(t *testing.T) {
	dbPath, db := createV2DB(t)

	// Insert 2 rows with different rel_paths, different device_ids
	_, err := db.Exec(`INSERT INTO files (rel_path, hash, size, device_id) VALUES (?, ?, ?, ?)`,
		"docs/report.txt", "aaaa", 100, "device-A")
	if err != nil {
		t.Fatalf("insert row 1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO files (rel_path, hash, size, device_id) VALUES (?, ?, ?, ?)`,
		"photos/img.jpg", "bbbb", 200, "device-B")
	if err != nil {
		t.Fatalf("insert row 2: %v", err)
	}

	// Verify the autoindex has 1 column before migration (v2 schema)
	colCount, err := autoIndexColumnCount(db)
	if err != nil {
		t.Fatalf("checking pre-migration index: %v", err)
	}
	if colCount != 1 {
		t.Fatalf("expected 1-column autoindex in v2 schema, got %d", colCount)
	}
	db.Close()

	// Run migration
	if err := RunMigration(dbPath); err != nil {
		t.Fatalf("RunMigration failed: %v", err)
	}

	// Reopen and verify
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to reopen DB: %v", err)
	}
	defer db2.Close()

	// The autoindex should now have 2 columns (composite constraint)
	colCount, err = autoIndexColumnCount(db2)
	if err != nil {
		t.Fatalf("checking post-migration index: %v", err)
	}
	if colCount != 2 {
		t.Errorf("expected 2-column autoindex after migration, got %d", colCount)
	}

	// Both rows must still exist
	var count int
	if err := db2.QueryRow("SELECT COUNT(*) FROM files").Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows after migration, got %d", count)
	}
}

// TestMigrationPreservesDeviceID verifies that device_id values survive the migration intact.
func TestMigrationPreservesDeviceID(t *testing.T) {
	dbPath, db := createV2DB(t)

	rows := []struct {
		relPath  string
		hash     string
		size     int
		deviceID string
	}{
		{"file-a.txt", "hash1", 10, "device-Alpha"},
		{"file-b.txt", "hash2", 20, "device-Beta"},
	}

	for _, r := range rows {
		_, err := db.Exec(`INSERT INTO files (rel_path, hash, size, device_id) VALUES (?, ?, ?, ?)`,
			r.relPath, r.hash, r.size, r.deviceID)
		if err != nil {
			t.Fatalf("insert %s: %v", r.relPath, err)
		}
	}
	db.Close()

	if err := RunMigration(dbPath); err != nil {
		t.Fatalf("RunMigration failed: %v", err)
	}

	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen DB: %v", err)
	}
	defer db2.Close()

	// Verify each row's device_id is unchanged
	for _, want := range rows {
		var gotDeviceID string
		err := db2.QueryRow("SELECT device_id FROM files WHERE rel_path = ?", want.relPath).Scan(&gotDeviceID)
		if err != nil {
			t.Fatalf("query %s: %v", want.relPath, err)
		}
		if gotDeviceID != want.deviceID {
			t.Errorf("rel_path=%s: want device_id=%q, got %q", want.relPath, want.deviceID, gotDeviceID)
		}
	}
}

// TestMigrationIdempotent verifies that running the migration twice is safe:
// no error, no data loss.
func TestMigrationIdempotent(t *testing.T) {
	dbPath, db := createV2DB(t)

	_, err := db.Exec(`INSERT INTO files (rel_path, hash, size, device_id) VALUES (?, ?, ?, ?)`,
		"idempotent.txt", "cccc", 50, "device-X")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	// First run
	if err := RunMigration(dbPath); err != nil {
		t.Fatalf("first RunMigration failed: %v", err)
	}

	// Second run — must not error or corrupt data
	if err := RunMigration(dbPath); err != nil {
		t.Fatalf("second RunMigration failed (idempotency violation): %v", err)
	}

	// Row count must be exactly 1 (no duplication)
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen DB: %v", err)
	}
	defer db2.Close()

	var count int
	if err := db2.QueryRow("SELECT COUNT(*) FROM files").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after idempotent migration, got %d", count)
	}
}

// TestMigrationAllowsDuplicatePaths verifies the composite constraint is in effect:
// two rows with the same rel_path but different device_ids must be insertable post-migration.
func TestMigrationAllowsDuplicatePaths(t *testing.T) {
	dbPath, db := createV2DB(t)

	// Insert one row before migration
	_, err := db.Exec(`INSERT INTO files (rel_path, hash, size, device_id) VALUES (?, ?, ?, ?)`,
		"shared/readme.md", "dddd", 30, "device-1")
	if err != nil {
		t.Fatalf("pre-migration insert: %v", err)
	}
	db.Close()

	if err := RunMigration(dbPath); err != nil {
		t.Fatalf("RunMigration failed: %v", err)
	}

	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen DB: %v", err)
	}
	defer db2.Close()

	// Same rel_path, different device_id — must succeed with composite constraint
	_, err = db2.Exec(`INSERT INTO files (rel_path, hash, size, device_id) VALUES (?, ?, ?, ?)`,
		"shared/readme.md", "eeee", 30, "device-2")
	if err != nil {
		t.Errorf("expected insert with same rel_path different device_id to succeed, got: %v", err)
	}

	// Same rel_path, same device_id — must fail (composite UNIQUE violated)
	_, err = db2.Exec(`INSERT INTO files (rel_path, hash, size, device_id) VALUES (?, ?, ?, ?)`,
		"shared/readme.md", "ffff", 30, "device-1")
	if err == nil {
		t.Error("expected insert with same rel_path AND same device_id to fail (UNIQUE violation), but it succeeded")
	}

	// Diagnostic only (not a real assertion) — suppress unused import warning
	_ = fmt.Sprintf
	_ = os.DevNull
}
