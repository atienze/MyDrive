package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

// needsMigration reports whether the files table still has the old single-column
// UNIQUE(rel_path) constraint. Both v2 and v3 schemas produce an autoindex named
// "sqlite_autoindex_files_1", so we distinguish them by counting the columns in
// that index: 1 column = v2 (needs migration), 2 columns = v3 (already migrated).
func needsMigration(conn *sql.DB) (bool, error) {
	rows, err := conn.Query("PRAGMA index_info(sqlite_autoindex_files_1)")
	if err != nil {
		return false, fmt.Errorf("PRAGMA index_info: %w", err)
	}
	defer rows.Close()

	var colCount int
	for rows.Next() {
		colCount++
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterating index_info: %w", err)
	}

	// 0 columns means the index doesn't exist (no files table?) — no migration needed
	// 1 column = v2 single-column UNIQUE(rel_path) — needs migration
	// 2 columns = v3 composite UNIQUE(rel_path, device_id) — already migrated
	return colCount == 1, nil
}

// RunMigration transitions the files table from the v2 schema (global UNIQUE rel_path)
// to the v3 schema (composite UNIQUE(rel_path, device_id)).
//
// It is safe to run multiple times: if the table has already been migrated the
// function returns nil immediately.
func RunMigration(dbPath string) error {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer conn.Close()

	needs, err := needsMigration(conn)
	if err != nil {
		return fmt.Errorf("checking migration status: %w", err)
	}
	if !needs {
		// Already on v3 schema — nothing to do.
		return nil
	}

	// Safety check: look for rows that would violate the new composite constraint
	// (same rel_path AND same device_id). This should never happen if data was
	// inserted correctly, but we guard against it to prevent silent data loss.
	dupRows, err := conn.Query(`
		SELECT rel_path, device_id, COUNT(*) as cnt
		FROM files
		GROUP BY rel_path, device_id
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		return fmt.Errorf("duplicate check query: %w", err)
	}
	defer dupRows.Close()

	var duplicates []string
	for dupRows.Next() {
		var relPath, deviceID string
		var cnt int
		if err := dupRows.Scan(&relPath, &deviceID, &cnt); err != nil {
			return fmt.Errorf("scanning duplicate row: %w", err)
		}
		duplicates = append(duplicates, fmt.Sprintf("  %s (device_id=%s, count=%d)", relPath, deviceID, cnt))
	}
	if err := dupRows.Err(); err != nil {
		return fmt.Errorf("iterating duplicate rows: %w", err)
	}
	dupRows.Close()

	if len(duplicates) > 0 {
		return fmt.Errorf("cannot migrate: found rows that would violate the new composite UNIQUE(rel_path, device_id) constraint:\n%s\nResolve duplicates before migrating",
			joinLines(duplicates))
	}

	// Perform the table-rename migration inside a transaction.
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() // no-op if Commit succeeds

	steps := []string{
		`CREATE TABLE files_new (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			rel_path    TEXT NOT NULL,
			hash        TEXT NOT NULL,
			size        INTEGER NOT NULL,
			device_id   TEXT NOT NULL,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted     BOOLEAN DEFAULT FALSE,
			UNIQUE(rel_path, device_id)
		)`,
		`INSERT INTO files_new (id, rel_path, hash, size, device_id, uploaded_at, deleted)
			SELECT id, rel_path, hash, size, device_id, uploaded_at, deleted FROM files`,
		`DROP TABLE files`,
		`ALTER TABLE files_new RENAME TO files`,
	}

	for _, stmt := range steps {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migration step failed (%s...): %w", truncate(stmt, 40), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}

	return nil
}

func main() {
	dbPath := os.Getenv("MYDRIVE_DB_PATH")
	if dbPath == "" {
		dbPath = "./mydrive.db"
	}

	fmt.Printf("=== myDrive Schema Migration v3: composite UNIQUE constraint ===\n")
	fmt.Printf("Database: %s\n\n", dbPath)

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("[error] failed to open database: %v", err)
	}

	needs, err := needsMigration(conn)
	conn.Close()
	if err != nil {
		log.Fatalf("[error] failed to check migration status: %v", err)
	}

	if !needs {
		fmt.Println("[skip] Already migrated — UNIQUE(rel_path, device_id) constraint is in place.")
		fmt.Println("[done] Nothing to do.")
		return
	}

	fmt.Println("[migrate] Migrating files table to composite unique constraint...")
	if err := RunMigration(dbPath); err != nil {
		log.Fatalf("[error] Migration failed: %v", err)
	}

	fmt.Println("[migrated] Successfully migrated files table to v3 schema.")
	fmt.Println("[done] UNIQUE(rel_path) → UNIQUE(rel_path, device_id)")
}

// joinLines joins a slice of strings with newlines.
func joinLines(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "\n"
		}
		out += s
	}
	return out
}

// truncate returns the first n characters of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
