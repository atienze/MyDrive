package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver as a side effect
)

// DB holds the connection to the SQLite database and exposes all file and
// device operations used by the server.
type DB struct {
	conn *sql.DB
}

// FileRecord represents one row in the files table, including metadata about
// the synced file and which device owns it.
type FileRecord struct {
	ID         int64
	RelPath    string
	Hash       string
	Size       int64
	DeviceID   string
	UploadedAt time.Time
	Deleted    bool
}

// Open opens (or creates) the SQLite database at the given file path,
// applies any pending schema migrations, and returns a ready-to-use DB.
// Call this once at server startup.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// SQLite supports only one concurrent writer. SetMaxOpenConns(1) prevents
	// "database is locked" errors under concurrent HTTP handler goroutines.
	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn}

	// createTables uses CREATE TABLE IF NOT EXISTS so it is safe to call on every startup.
	if err := db.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	// migrateSchema upgrades pre-existing databases that may have been created with
	// an older schema (e.g. UNIQUE(rel_path) alone instead of UNIQUE(rel_path, device_id)).
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

// createTables creates the files and devices tables if they do not already exist.
// It is safe to call on every startup.
func (db *DB) createTables() error {
	schema := `
    CREATE TABLE IF NOT EXISTS files (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        rel_path    TEXT NOT NULL,
        hash        TEXT NOT NULL,
        size        INTEGER NOT NULL,
        device_id   TEXT NOT NULL,
        uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        deleted     BOOLEAN DEFAULT FALSE,
        UNIQUE(rel_path, device_id)
    );

    CREATE TABLE IF NOT EXISTS devices (
        id         TEXT PRIMARY KEY,
        name       TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `

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

// UpsertFile inserts a new file record for the given device, or updates the
// existing record if (rel_path, device_id) already exists. Re-syncing the same
// file updates the hash and size rather than returning an error.
func (db *DB) UpsertFile(relPath, hash, deviceID string, size int64) error {
	query := `
    INSERT INTO files (rel_path, hash, size, device_id, uploaded_at, deleted)
    VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, FALSE)
    ON CONFLICT(rel_path, device_id) DO UPDATE SET
        hash        = excluded.hash,
        size        = excluded.size,
        uploaded_at = CURRENT_TIMESTAMP,
        deleted     = FALSE
    `

	_, err := db.conn.Exec(query, relPath, hash, size, deviceID)
	if err != nil {
		return fmt.Errorf("upsert file %s: %w", relPath, err)
	}
	return nil
}

// FileExists reports whether the server already has a non-deleted record for
// the given (relPath, hash, deviceID) triplet. Returns true when the file is
// already stored and unchanged; the client can skip re-uploading it.
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

// GetAllFiles returns every non-deleted file in the database, ordered by relative path.
func (db *DB) GetAllFiles() ([]FileRecord, error) {
	query := `
    SELECT id, rel_path, hash, size, device_id, uploaded_at
    FROM files
    WHERE deleted = FALSE
    ORDER BY rel_path
    `

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get all files: %w", err)
	}
	defer rows.Close()

	var files []FileRecord
	for rows.Next() {
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

// RegisterDevice creates a new device record with the given token as its primary key.
// If a record with the same id already exists, the insert is silently ignored.
func (db *DB) RegisterDevice(id, name string) error {
	query := `
    INSERT INTO devices (id, name, created_at)
    VALUES (?, ?, CURRENT_TIMESTAMP)
    ON CONFLICT(id) DO NOTHING
    `
	_, err := db.conn.Exec(query, id, name)
	return err
}

// DeviceExists reports whether a device with the given ID is registered.
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
