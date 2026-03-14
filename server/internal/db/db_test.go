package db

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestUniquePerDevice(t *testing.T) {
	d := openTestDB(t)

	if err := d.UpsertFile("docs/readme.md", "aaa", "device-A", 100); err != nil {
		t.Fatalf("insert device-A: %v", err)
	}
	if err := d.UpsertFile("docs/readme.md", "bbb", "device-B", 200); err != nil {
		t.Fatalf("insert device-B: %v", err)
	}

	files, err := d.GetAllFiles()
	if err != nil {
		t.Fatalf("GetAllFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 rows for same path on different devices, got %d", len(files))
	}
}

func TestUpsertFileCompositeKey(t *testing.T) {
	d := openTestDB(t)

	if err := d.UpsertFile("file.txt", "hash1", "device-A", 10); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := d.UpsertFile("file.txt", "hash2", "device-A", 20); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	files, err := d.GetAllFiles()
	if err != nil {
		t.Fatalf("GetAllFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 row after upsert, got %d", len(files))
	}
	if files[0].Hash != "hash2" {
		t.Errorf("expected hash2 after upsert, got %s", files[0].Hash)
	}
}

func TestFileExistsScoped(t *testing.T) {
	d := openTestDB(t)

	if err := d.UpsertFile("photo.jpg", "abc", "device-A", 500); err != nil {
		t.Fatalf("insert: %v", err)
	}

	exists, err := d.FileExists("photo.jpg", "abc", "device-A")
	if err != nil {
		t.Fatalf("FileExists device-A: %v", err)
	}
	if !exists {
		t.Error("expected FileExists=true for device-A")
	}

	exists, err = d.FileExists("photo.jpg", "abc", "device-B")
	if err != nil {
		t.Fatalf("FileExists device-B: %v", err)
	}
	if exists {
		t.Error("expected FileExists=false for device-B")
	}
}

func TestGetFileHashScoped(t *testing.T) {
	d := openTestDB(t)

	if err := d.UpsertFile("doc.txt", "xyz", "device-A", 30); err != nil {
		t.Fatalf("insert: %v", err)
	}

	hash, found, err := d.GetFileHash("doc.txt", "device-A")
	if err != nil {
		t.Fatalf("GetFileHash device-A: %v", err)
	}
	if !found || hash != "xyz" {
		t.Errorf("expected (xyz, true), got (%s, %v)", hash, found)
	}

	hash, found, err = d.GetFileHash("doc.txt", "device-B")
	if err != nil {
		t.Fatalf("GetFileHash device-B: %v", err)
	}
	if found {
		t.Errorf("expected not found for device-B, got hash=%s", hash)
	}
}

func TestMarkDeletedScoped(t *testing.T) {
	d := openTestDB(t)

	if err := d.UpsertFile("shared.txt", "hhh", "device-A", 10); err != nil {
		t.Fatalf("insert A: %v", err)
	}
	if err := d.UpsertFile("shared.txt", "iii", "device-B", 20); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	if err := d.MarkDeleted("shared.txt", "device-A"); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}

	// device-A's file should be gone
	_, found, _ := d.GetFileHash("shared.txt", "device-A")
	if found {
		t.Error("expected device-A file to be deleted")
	}

	// device-B's file should still exist
	hash, found, _ := d.GetFileHash("shared.txt", "device-B")
	if !found || hash != "iii" {
		t.Error("expected device-B file to still exist")
	}
}

func TestGetFilesForDevice(t *testing.T) {
	d := openTestDB(t)

	for _, f := range []struct{ path, hash, dev string }{
		{"a.txt", "h1", "device-A"},
		{"b.txt", "h2", "device-A"},
		{"c.txt", "h3", "device-A"},
		{"d.txt", "h4", "device-B"},
		{"e.txt", "h5", "device-B"},
	} {
		if err := d.UpsertFile(f.path, f.hash, f.dev, 10); err != nil {
			t.Fatalf("insert %s: %v", f.path, err)
		}
	}

	files, err := d.GetFilesForDevice("device-A")
	if err != nil {
		t.Fatalf("GetFilesForDevice: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 files for device-A, got %d", len(files))
	}
}

func TestGetSharedFiles(t *testing.T) {
	d := openTestDB(t)

	for _, f := range []struct{ path, hash, dev string }{
		{"a.txt", "h1", "device-A"},
		{"b.txt", "h2", "device-A"},
		{"c.txt", "h3", "device-A"},
		{"d.txt", "h4", "device-B"},
		{"e.txt", "h5", "device-B"},
	} {
		if err := d.UpsertFile(f.path, f.hash, f.dev, 10); err != nil {
			t.Fatalf("insert %s: %v", f.path, err)
		}
	}

	files, err := d.GetSharedFiles("device-A")
	if err != nil {
		t.Fatalf("GetSharedFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 shared files (device-B's), got %d", len(files))
	}
}

func TestHashRefCountMultiDevice(t *testing.T) {
	d := openTestDB(t)

	// Same hash on two devices
	if err := d.UpsertFile("file1.txt", "samehash", "device-A", 10); err != nil {
		t.Fatalf("insert A: %v", err)
	}
	if err := d.UpsertFile("file2.txt", "samehash", "device-B", 10); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	count, err := d.HashRefCount("samehash")
	if err != nil {
		t.Fatalf("HashRefCount: %v", err)
	}
	if count != 2 {
		t.Errorf("expected ref count 2, got %d", count)
	}

	// Delete one
	if err := d.MarkDeleted("file1.txt", "device-A"); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}

	count, err = d.HashRefCount("samehash")
	if err != nil {
		t.Fatalf("HashRefCount after delete: %v", err)
	}
	if count != 1 {
		t.Errorf("expected ref count 1 after delete, got %d", count)
	}
}

func TestBlobCleanupAfterAllDevicesDelete(t *testing.T) {
	d := openTestDB(t)

	if err := d.UpsertFile("f1.txt", "deadhash", "device-A", 10); err != nil {
		t.Fatalf("insert A: %v", err)
	}
	if err := d.UpsertFile("f2.txt", "deadhash", "device-B", 10); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	d.MarkDeleted("f1.txt", "device-A")
	d.MarkDeleted("f2.txt", "device-B")

	count, err := d.HashRefCount("deadhash")
	if err != nil {
		t.Fatalf("HashRefCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected ref count 0 after all deletes, got %d", count)
	}

	// Extended: after purging both rows, no rows remain for this hash at all.
	if err := d.PurgeDeletedRecord("f1.txt", "device-A"); err != nil {
		t.Fatalf("PurgeDeletedRecord device-A: %v", err)
	}
	if err := d.PurgeDeletedRecord("f2.txt", "device-B"); err != nil {
		t.Fatalf("PurgeDeletedRecord device-B: %v", err)
	}

	var rawCount int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM files WHERE hash = 'deadhash'`).Scan(&rawCount); err != nil {
		t.Fatalf("raw count query: %v", err)
	}
	if rawCount != 0 {
		t.Errorf("expected 0 rows after purge, got %d (soft-deleted rows still linger)", rawCount)
	}
}

func TestPurgeDeletedRecord(t *testing.T) {
	d := openTestDB(t)

	// Setup: insert and soft-delete a file for device-A.
	if err := d.UpsertFile("purge.txt", "purgeme", "device-A", 100); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := d.MarkDeleted("purge.txt", "device-A"); err != nil {
		t.Fatalf("MarkDeleted: %v", err)
	}

	// Purge the soft-deleted row.
	if err := d.PurgeDeletedRecord("purge.txt", "device-A"); err != nil {
		t.Fatalf("PurgeDeletedRecord: %v", err)
	}

	// Row must be physically gone — query without deleted filter.
	var count int
	if err := d.conn.QueryRow(
		`SELECT COUNT(*) FROM files WHERE rel_path = ? AND device_id = ?`,
		"purge.txt", "device-A",
	).Scan(&count); err != nil {
		t.Fatalf("raw count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected row to be physically removed, got count=%d", count)
	}

	// Idempotent: calling again on the now-absent row must not error.
	if err := d.PurgeDeletedRecord("purge.txt", "device-A"); err != nil {
		t.Errorf("PurgeDeletedRecord idempotent call failed: %v", err)
	}

	// Safety: PurgeDeletedRecord must NOT remove a non-deleted (live) row.
	if err := d.UpsertFile("live.txt", "livehash", "device-A", 50); err != nil {
		t.Fatalf("upsert live: %v", err)
	}
	if err := d.PurgeDeletedRecord("live.txt", "device-A"); err != nil {
		t.Fatalf("PurgeDeletedRecord on live row: %v", err)
	}
	var liveCount int
	if err := d.conn.QueryRow(
		`SELECT COUNT(*) FROM files WHERE rel_path = ? AND device_id = ?`,
		"live.txt", "device-A",
	).Scan(&liveCount); err != nil {
		t.Fatalf("live count: %v", err)
	}
	if liveCount != 1 {
		t.Errorf("expected live row to be untouched, got count=%d", liveCount)
	}
}
