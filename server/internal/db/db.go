package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite" // The underscore means: import this for its side effects
	// (it registers the "sqlite" driver) but don't use it directly
)

// DB is our database handle. Every other file in the server will use this.
// Think of it as the object that holds the connection to SQLite open.
type DB struct {
	conn *sql.DB // sql.DB is Go's standard database connection type
}

// FileRecord represents one row in our 'files' table.
// When we read a file out of the database, it comes back as one of these.
type FileRecord struct {
	ID         int64
	RelPath    string
	Hash       string
	Size       int64
	DeviceID   string
	UploadedAt time.Time
	Deleted    bool
}

// Open opens (or creates) the SQLite database at the given file path.
// Call this once when the server starts up.
func Open(path string) (*DB, error) {
	// sql.Open doesn't actually connect yet — it just sets up the config.
	// The "sqlite" string tells Go which database driver to use (the one we imported above).
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// sql.Ping() actually tries to connect and will fail fast if something is wrong.
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// SQLite has one important quirk: it only supports ONE writer at a time.
	// SetMaxOpenConns(1) tells Go's connection pool to never open more than
	// one connection at a time, which prevents "database is locked" errors.
	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn}

	// Run our table creation SQL. This is safe to call every startup —
	// "CREATE TABLE IF NOT EXISTS" means it only creates if it doesn't exist.
	if err := db.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	// Run schema migrations for pre-existing databases that may have been created
	// with an older schema (e.g. UNIQUE(rel_path) alone instead of UNIQUE(rel_path, device_id)).
	if err := db.migrateSchema(); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	log.Println("Database initialized successfully")
	return db, nil
}

// Close shuts down the database connection cleanly.
// Always defer this after calling Open().
func (db *DB) Close() error {
	return db.conn.Close()
}

// createTables runs the SQL that creates our schema.
// Using backticks (`) in Go lets us write multi-line strings — perfect for SQL.
func (db *DB) createTables() error {
	schema := `
    -- Tracks every file the server has received (v3: composite unique per device)
    CREATE TABLE IF NOT EXISTS files (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        rel_path    TEXT NOT NULL,          -- e.g. "Documents/resume.pdf"
        hash        TEXT NOT NULL,          -- SHA-256 of the file content
        size        INTEGER NOT NULL,       -- file size in bytes
        device_id   TEXT NOT NULL,          -- which device sent this
        uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        deleted     BOOLEAN DEFAULT FALSE,  -- soft delete flag
        UNIQUE(rel_path, device_id)
    );

    -- Tracks registered devices (we'll fill this more in Phase 2)
    CREATE TABLE IF NOT EXISTS devices (
        id         TEXT PRIMARY KEY,        -- will become the auth token in Phase 2
        name       TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `

	// Exec runs SQL that doesn't return rows (CREATE, INSERT, UPDATE, DELETE).
	// For queries that return rows, we'd use Query() or QueryRow() instead.
	_, err := db.conn.Exec(schema)
	return err
}

// migrateSchema checks for and applies schema changes needed for pre-existing databases.
// Fresh databases created by createTables() already have the correct schema (UNIQUE(rel_path,
// device_id) defined inline in CREATE TABLE), so this is a no-op for them. For older databases
// that only had UNIQUE(rel_path) alone, this creates the correct composite unique index so that
// UpsertFile's ON CONFLICT(rel_path, device_id) clause resolves without errors.
//
// Detection strategy: use pragma_index_info to check whether any unique index on the files
// table actually covers both rel_path and device_id columns. This correctly identifies both
// named user indexes AND system autoindexes generated from inline UNIQUE constraints.
func (db *DB) migrateSchema() error {
	// Find all unique indexes on the files table.
	rows, err := db.conn.Query(`
		SELECT il.name FROM pragma_index_list('files') il WHERE il."unique" = 1
	`)
	if err != nil {
		return fmt.Errorf("check schema migration (index list): %w", err)
	}
	defer rows.Close()

	var indexNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan index name: %w", err)
		}
		indexNames = append(indexNames, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate index list: %w", err)
	}
	rows.Close() // close before next query

	// For each unique index, check if it covers both rel_path and device_id.
	for _, idxName := range indexNames {
		colRows, err := db.conn.Query(`SELECT name FROM pragma_index_info(?)`, idxName)
		if err != nil {
			return fmt.Errorf("check index info for %s: %w", idxName, err)
		}

		cols := make(map[string]bool)
		for colRows.Next() {
			var col string
			if err := colRows.Scan(&col); err != nil {
				colRows.Close()
				return fmt.Errorf("scan index column: %w", err)
			}
			cols[col] = true
		}
		colRows.Close()

		if cols["rel_path"] && cols["device_id"] {
			// Composite unique index covering both columns already exists.
			log.Println("Database schema up to date")
			return nil
		}
	}

	// No composite unique index found — run migration.
	log.Println("Database migration: adding composite unique index on files(rel_path, device_id)")
	_, err = db.conn.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_files_path_device ON files(rel_path, device_id)`,
	)
	if err != nil {
		return fmt.Errorf("apply schema migration: %w", err)
	}
	log.Println("Database migration: added composite unique index on files(rel_path, device_id)")
	return nil
}

// ----------------------------------------
// FILE OPERATIONS
// ----------------------------------------

// UpsertFile inserts a new file record, or updates it if the path already exists.
// "Upsert" = "Update or Insert" — a common database pattern.
// We use this so re-syncing the same file updates the hash instead of erroring.
func (db *DB) UpsertFile(relPath, hash, deviceID string, size int64) error {
	// The ? marks are placeholders — Go fills them in with our values in order.
	// NEVER build SQL by string concatenation (e.g. "... WHERE path = " + path)
	// That would be vulnerable to SQL injection. Always use placeholders.
	query := `
    INSERT INTO files (rel_path, hash, size, device_id, uploaded_at, deleted)
    VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, FALSE)
    ON CONFLICT(rel_path, device_id) DO UPDATE SET
        hash        = excluded.hash,
        size        = excluded.size,
        uploaded_at = CURRENT_TIMESTAMP,
        deleted     = FALSE
    `
	// "excluded.hash" is SQLite syntax meaning "the hash value we tried to insert"
	// So ON CONFLICT means: if rel_path already exists, update the other fields.

	_, err := db.conn.Exec(query, relPath, hash, size, deviceID)
	if err != nil {
		return fmt.Errorf("upsert file %s: %w", relPath, err)
	}
	return nil
}

// FileExists checks if we already have a file with this exact path AND hash.
// Returns true if the file is already stored and unchanged — meaning skip it.
// Returns false if we need the file (new file, or file has changed).
func (db *DB) FileExists(relPath, hash, deviceID string) (bool, error) {
	query := `
    SELECT COUNT(*) FROM files
    WHERE rel_path = ? AND hash = ? AND device_id = ? AND deleted = FALSE
    `

	var count int
	err := db.conn.QueryRow(query, relPath, hash, deviceID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check file exists %s: %w", relPath, err)
	}

	return count > 0, nil
}

// GetAllFiles returns every non-deleted file in the database.
// We'll use this in Phase 5 for bidirectional sync.
func (db *DB) GetAllFiles() ([]FileRecord, error) {
	query := `
    SELECT id, rel_path, hash, size, device_id, uploaded_at
    FROM files
    WHERE deleted = FALSE
    ORDER BY rel_path
    `

	// Query returns multiple rows, unlike QueryRow which returns one.
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get all files: %w", err)
	}
	defer rows.Close() // Always close rows when done — this releases the connection

	var files []FileRecord
	for rows.Next() { // rows.Next() advances to the next row, returns false when done
		var f FileRecord
		err := rows.Scan(
			&f.ID,
			&f.RelPath,
			&f.Hash,
			&f.Size,
			&f.DeviceID,
			&f.UploadedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan file row: %w", err)
		}
		files = append(files, f)
	}

	// rows.Err() catches any error that happened DURING iteration
	// (rows.Next() swallows errors — you have to check here)
	return files, rows.Err()
}

// GetFileHash returns the hash for a non-deleted file at the given path.
// Returns ("", false, nil) if the file does not exist or is already deleted.
func (db *DB) GetFileHash(relPath, deviceID string) (string, bool, error) {
	var hash string
	err := db.conn.QueryRow(
		`SELECT hash FROM files WHERE rel_path = ? AND device_id = ? AND deleted = FALSE`, relPath, deviceID,
	).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get file hash for %s: %w", relPath, err)
	}
	return hash, true, nil
}

// GetFileHashAnyDevice returns the hash for a non-deleted file at relPath,
// regardless of which device owns it. Used by CmdRequestFile for cross-device pull.
// Returns ("", "", false, nil) if not found.
func (db *DB) GetFileHashAnyDevice(relPath string) (hash string, deviceID string, found bool, err error) {
	err = db.conn.QueryRow(
		`SELECT hash, device_id FROM files WHERE rel_path = ? AND deleted = FALSE LIMIT 1`,
		relPath,
	).Scan(&hash, &deviceID)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("get file hash any device for %s: %w", relPath, err)
	}
	return hash, deviceID, true, nil
}

// MarkDeleted soft-deletes a file — we keep the record but flag it as gone.
func (db *DB) MarkDeleted(relPath, deviceID string) error {
	query := `UPDATE files SET deleted = TRUE WHERE rel_path = ? AND device_id = ?`
	_, err := db.conn.Exec(query, relPath, deviceID)
	return err
}

// PurgeDeletedRecord hard-deletes a soft-deleted row from the files table.
// It only removes rows where deleted=TRUE, so it is safe to call even if the
// caller is unsure whether MarkDeleted was already called — live rows are never
// touched. Calling PurgeDeletedRecord on a non-existent row is a no-op.
func (db *DB) PurgeDeletedRecord(relPath, deviceID string) error {
	_, err := db.conn.Exec(
		`DELETE FROM files WHERE rel_path = ? AND device_id = ? AND deleted = TRUE`,
		relPath, deviceID,
	)
	if err != nil {
		return fmt.Errorf("purge deleted record %s (device %s): %w", relPath, deviceID, err)
	}
	return nil
}

// GetFilesForDevice returns all non-deleted files owned by the given device.
func (db *DB) GetFilesForDevice(deviceID string) ([]FileRecord, error) {
	query := `
    SELECT id, rel_path, hash, size, device_id, uploaded_at
    FROM files
    WHERE device_id = ? AND deleted = FALSE
    ORDER BY rel_path
    `
	rows, err := db.conn.Query(query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("get files for device %s: %w", deviceID, err)
	}
	defer rows.Close()

	var files []FileRecord
	for rows.Next() {
		var f FileRecord
		err := rows.Scan(&f.ID, &f.RelPath, &f.Hash, &f.Size, &f.DeviceID, &f.UploadedAt)
		if err != nil {
			return nil, fmt.Errorf("scan file row: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// GetSharedFiles returns all non-deleted files from devices other than the given one.
func (db *DB) GetSharedFiles(excludeDeviceID string) ([]FileRecord, error) {
	query := `
    SELECT id, rel_path, hash, size, device_id, uploaded_at
    FROM files
    WHERE device_id != ? AND deleted = FALSE
    ORDER BY rel_path
    `
	rows, err := db.conn.Query(query, excludeDeviceID)
	if err != nil {
		return nil, fmt.Errorf("get shared files excluding %s: %w", excludeDeviceID, err)
	}
	defer rows.Close()

	var files []FileRecord
	for rows.Next() {
		var f FileRecord
		err := rows.Scan(&f.ID, &f.RelPath, &f.Hash, &f.Size, &f.DeviceID, &f.UploadedAt)
		if err != nil {
			return nil, fmt.Errorf("scan file row: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ----------------------------------------
// DEVICE OPERATIONS (used more in Phase 2)
// ----------------------------------------

// RegisterDevice creates a new device record.
// The id will eventually be the auth token.
func (db *DB) RegisterDevice(id, name string) error {
	query := `
    INSERT INTO devices (id, name, created_at)
    VALUES (?, ?, CURRENT_TIMESTAMP)
    ON CONFLICT(id) DO NOTHING
    `
	_, err := db.conn.Exec(query, id, name)
	return err
}

// DeviceExists checks if a device ID is registered.
// We'll use this in Phase 2 for auth — if the token isn't registered, reject.
func (db *DB) DeviceExists(id string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM devices WHERE id = ?`, id,
	).Scan(&count)
	return count > 0, err
}

// HashRefCount returns the number of non-deleted file rows referencing this hash.
// Used to decide whether a blob can safely be removed from disk.
func (db *DB) HashRefCount(hash string) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM files WHERE hash = ? AND deleted = FALSE`, hash,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("hash ref count for %s: %w", hash, err)
	}
	return count, nil
}

// GetDeviceName looks up the human-readable name for a registered token.
// Returns ("", false, nil) if the token is not found (unregistered).
// Returns (name, true, nil) on success.
// Returns ("", false, err) on a database error.
// Use this for auth checks — it combines existence check and name lookup in one query.
func (db *DB) GetDeviceName(token string) (string, bool, error) {
	var name string
	err := db.conn.QueryRow(
		`SELECT name FROM devices WHERE id = ?`, token,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get device name for token: %w", err)
	}
	return name, true, nil
}
