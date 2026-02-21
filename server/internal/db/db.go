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
    -- Tracks every file the server has received
    CREATE TABLE IF NOT EXISTS files (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        rel_path    TEXT NOT NULL UNIQUE,   -- e.g. "Documents/resume.pdf"
        hash        TEXT NOT NULL,          -- SHA-256 of the file content
        size        INTEGER NOT NULL,       -- file size in bytes
        device_id   TEXT NOT NULL,          -- which device sent this
        uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        deleted     BOOLEAN DEFAULT FALSE   -- soft delete flag for future use
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
    ON CONFLICT(rel_path) DO UPDATE SET
        hash        = excluded.hash,
        size        = excluded.size,
        device_id   = excluded.device_id,
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
func (db *DB) FileExists(relPath, hash string) (bool, error) {
    query := `
    SELECT COUNT(*) FROM files
    WHERE rel_path = ? AND hash = ? AND deleted = FALSE
    `

    var count int
    // QueryRow is used for queries that return exactly one row.
    // Scan() reads the result into our variable (count).
    err := db.conn.QueryRow(query, relPath, hash).Scan(&count)
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

// MarkDeleted soft-deletes a file — we keep the record but flag it as gone.
// We'll use this in Phase 4 when the client tells us a file was deleted.
func (db *DB) MarkDeleted(relPath string) error {
    query := `UPDATE files SET deleted = TRUE WHERE rel_path = ?`
    _, err := db.conn.Exec(query, relPath)
    return err
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