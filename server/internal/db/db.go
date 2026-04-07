package db

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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
	// an older schema.
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
// Fresh databases get the new schema: devices.id is a UUID, devices.token_hash stores
// the HMAC-SHA256 of the raw token, and files.device_id stores the UUID.
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
        token_hash TEXT NOT NULL UNIQUE,
        name       TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    `

	_, err := db.conn.Exec(schema)
	return err
}

// migrateSchema checks for and applies schema changes needed for pre-existing databases.
//
// Migration 1 (files composite index): For older databases that only had UNIQUE(rel_path)
// alone, this creates the correct composite unique index so that UpsertFile's
// ON CONFLICT(rel_path, device_id) clause resolves without errors.
//
// Migration 2 (devices token_hash + UUID PK): For databases where devices.id stored the
// raw token and token_hash did not exist, this:
//   - Adds the token_hash column
//   - For each existing device: generates a new random UUID (v4), computes
//     HMAC-SHA256(key="mydrive-v1", data=raw_token) as the token_hash, inserts the
//     new row, updates files.device_id from device_name to UUID, deletes the old row.
//   - Creates the UNIQUE index on token_hash
func (db *DB) migrateSchema() error {
	// --- Migration 1: composite unique index on files(rel_path, device_id) ---
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

	hasCompositeIndex := false
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
			hasCompositeIndex = true
			break
		}
	}

	if !hasCompositeIndex {
		log.Println("Database migration: adding composite unique index on files(rel_path, device_id)")
		_, err = db.conn.Exec(
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_files_path_device ON files(rel_path, device_id)`,
		)
		if err != nil {
			return fmt.Errorf("apply schema migration (composite index): %w", err)
		}
		log.Println("Database migration: added composite unique index on files(rel_path, device_id)")
	} else {
		log.Println("Database schema (files composite index) up to date")
	}

	// --- Migration 2: devices table — UUID PK + token_hash column ---
	// Detect whether token_hash column already exists.
	var tokenHashExists bool
	colRows, err := db.conn.Query(`SELECT name FROM pragma_table_info('devices')`)
	if err != nil {
		return fmt.Errorf("check devices table info: %w", err)
	}
	for colRows.Next() {
		var colName string
		if err := colRows.Scan(&colName); err != nil {
			colRows.Close()
			return fmt.Errorf("scan devices column name: %w", err)
		}
		if colName == "token_hash" {
			tokenHashExists = true
			break
		}
	}
	colRows.Close()

	if tokenHashExists {
		log.Println("Database schema (devices token_hash) up to date")
		return nil
	}

	// token_hash column does not exist — run the devices migration.
	log.Println("Database migration: upgrading devices table to UUID PK + token_hash")

	// Step 1: Add token_hash column (nullable initially).
	_, err = db.conn.Exec(`ALTER TABLE devices ADD COLUMN token_hash TEXT`)
	if err != nil {
		return fmt.Errorf("alter devices add token_hash: %w", err)
	}
	log.Println("Database migration: added token_hash column to devices")

	// Step 2: Load all existing device rows (old_token is PK = raw token).
	type oldDevice struct {
		OldToken  string
		Name      string
		CreatedAt string
	}
	devRows, err := db.conn.Query(`SELECT id, name, created_at FROM devices`)
	if err != nil {
		return fmt.Errorf("load existing devices for migration: %w", err)
	}
	var devices []oldDevice
	for devRows.Next() {
		var d oldDevice
		if err := devRows.Scan(&d.OldToken, &d.Name, &d.CreatedAt); err != nil {
			devRows.Close()
			return fmt.Errorf("scan device row: %w", err)
		}
		devices = append(devices, d)
	}
	devRows.Close()

	// Step 3: For each device: generate UUID, compute token_hash, migrate rows.
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin devices migration transaction: %w", err)
	}

	for _, d := range devices {
		newUUID, err := generateUUID()
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("generate UUID for device %q: %w", d.Name, err)
		}

		tokenHash := computeTokenHash(d.OldToken)

		// Insert the new row with UUID as PK and the computed token_hash.
		_, err = tx.Exec(
			`INSERT INTO devices (id, token_hash, name, created_at) VALUES (?, ?, ?, ?)`,
			newUUID, tokenHash, d.Name, d.CreatedAt,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("insert migrated device %q: %w", d.Name, err)
		}

		// Update files: files.device_id used to store the device *name* (not the token).
		// After migration it must store the UUID.
		res, err := tx.Exec(
			`UPDATE files SET device_id = ? WHERE device_id = ?`,
			newUUID, d.Name,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("update files.device_id for device %q: %w", d.Name, err)
		}
		affected, _ := res.RowsAffected()
		log.Printf("Database migration: device %q → UUID %s (%d file rows updated)", d.Name, newUUID, affected)

		// Delete the old row (PK was the raw token).
		_, err = tx.Exec(`DELETE FROM devices WHERE id = ?`, d.OldToken)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("delete old device row for %q: %w", d.Name, err)
		}
	}

	// Step 4: Add UNIQUE index on token_hash.
	_, err = tx.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_token_hash ON devices(token_hash)`,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("create unique index on devices(token_hash): %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit devices migration transaction: %w", err)
	}

	log.Println("Database migration: devices table upgraded to UUID PK + token_hash successfully")
	return nil
}

// generateUUID produces a random UUID v4 string in lowercase hyphenated form:
// "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
// Uses crypto/rand only — no external library.
func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate UUID random bytes: %w", err)
	}
	// Set version 4 bits.
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant bits (RFC 4122).
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
}

// computeTokenHash computes HMAC-SHA256(key="mydrive-v1", data=token) and returns
// the result as a lowercase hex string. This is the canonical token_hash for storage.
func computeTokenHash(token string) string {
	mac := hmac.New(sha256.New, []byte("mydrive-v1"))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

// UpsertFile inserts a new file record for the given device, or updates the
// existing record if (rel_path, device_id) already exists. Re-syncing the same
// file updates the hash and size rather than returning an error.
// deviceID must be a UUID (after Plan 02 migration).
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

// RegisterDevice creates a new device record with the given UUID as its primary key,
// token_hash as the HMAC-SHA256 of the raw token, and name as the human-readable label.
// The caller (server/cmd/main.go) is responsible for generating the UUID and computing
// the token_hash before calling this method.
// If a record with the same id or token_hash already exists, the insert is silently ignored.
func (db *DB) RegisterDevice(uuid, tokenHash, name string) error {
	query := `
    INSERT INTO devices (id, token_hash, name, created_at)
    VALUES (?, ?, ?, CURRENT_TIMESTAMP)
    ON CONFLICT(id) DO NOTHING
    `
	_, err := db.conn.Exec(query, uuid, tokenHash, name)
	return err
}

// GetDeviceByTokenHash looks up a device by the HMAC-SHA256 of its raw token.
// Returns (uuid, name, true, nil) on success.
// Returns ("", "", false, nil) if no device has this token_hash (unregistered/invalid token).
// Returns ("", "", false, err) on a database error.
// Use this for per-connection auth in the TCP handler.
func (db *DB) GetDeviceByTokenHash(tokenHash string) (uuid string, name string, found bool, err error) {
	err = db.conn.QueryRow(
		`SELECT id, name FROM devices WHERE token_hash = ?`, tokenHash,
	).Scan(&uuid, &name)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("get device by token hash: %w", err)
	}
	return uuid, name, true, nil
}

// GetDeviceName looks up the human-readable name for a registered device by its UUID.
// Returns ("", false, nil) if the UUID is not found (unregistered).
// Returns (name, true, nil) on success.
// Returns ("", false, err) on a database error.
func (db *DB) GetDeviceName(uuid string) (string, bool, error) {
	var name string
	err := db.conn.QueryRow(
		`SELECT name FROM devices WHERE id = ?`, uuid,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get device name for UUID: %w", err)
	}
	return name, true, nil
}

// DeviceExists reports whether a device with the given UUID is registered.
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

// ComputeTokenHash is the exported version of computeTokenHash, for use by callers
// (e.g. server/cmd/main.go and server/internal/receiver/handler.go) that need to
// hash a raw token before passing it to RegisterDevice or GetDeviceByTokenHash.
func ComputeTokenHash(token string) string {
	return computeTokenHash(token)
}
