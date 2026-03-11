# Testing Patterns

**Analysis Date:** 2025-03-11

## Test Framework

**Runner:**
- Go's built-in `testing` package (standard library)
- No external test framework (no Jest, Vitest, etc.)
- Tests run via `go test ./...` from workspace root

**Assertion Library:**
- None; manual assertion via `if got != want { t.Errorf(...) }`
- No assertion library like testify

**Run Commands:**
```bash
# Run all tests in all modules
go test ./...

# Run specific package tests
go test ./server/internal/store/...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run coverage with detailed report
go test -coverprofile=coverage.out ./...
```

## Test File Organization

**Location:**
- Co-located with source: tests in same package as code, same directory
- Pattern: `{module}_test.go` in same directory as `{module}.go`
- Example: `server/internal/store/store.go` and `server/internal/store/store_test.go`

**Naming:**
- Test functions: `Test{FunctionName}(t *testing.T)` — PascalCase with Test prefix
- Examples from actual tests:
  - `TestObjectPath(t *testing.T)`
  - `TestWriteAndHasObject(t *testing.T)`
  - `TestWriteObjectDedup(t *testing.T)`
  - `TestDeleteObjectZeroRefCount(t *testing.T)`

**Structure:**
```
server/internal/
├── store/
│   ├── store.go           # Implementation
│   └── store_test.go      # Tests (same package)
├── db/
│   └── db.go              # No test file (untested)
├── receiver/
│   └── handler.go         # No test file (untested)
└── auth/
    └── register.go        # No test file (untested)
```

## Test Structure

**Suite Organization:**
Go's `testing` package uses individual test functions, not suites. Each `Test*` function is independent.

```go
// From server/internal/store/store_test.go
func TestWriteAndHasObject(t *testing.T) {
	s := setupStore(t)           // Setup helper
	data := []byte("hello world")
	hash := testHash(data)

	if s.HasObject(hash) {
		t.Fatal("HasObject should be false before write")
	}

	if err := s.WriteObject(hash, data); err != nil {
		t.Fatalf("WriteObject failed: %v", err)
	}

	if !s.HasObject(hash) {
		t.Fatal("HasObject should be true after write")
	}
}
```

**Patterns:**
- Setup helper: `func setupStore(t *testing.T) *ObjectStore` — marked with `t.Helper()` to exclude from stack traces
- Arrange-Act-Assert structure implicit (setup, call function, check result)
- Inline test data: `data := []byte("test content")`
- Fatal vs. Error: `t.Fatalf` for critical failures (stops test), `t.Errorf` for assertion failures (continues test)

## Mocking

**Framework:**
- No mocking framework used; tests use real file system and real objects
- Setup/teardown via helpers: `setupStore(t)` creates `testing.TempDir()` for isolated tests

**Patterns:**
From `server/internal/store/store_test.go`:

```go
// Helper creates a real ObjectStore in a temporary directory
func setupStore(t *testing.T) *ObjectStore {
	t.Helper()
	dir := t.TempDir()  // Go's built-in temporary directory, auto-cleaned
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New(%q) failed: %v", dir, err)
	}
	return s
}

// Tests use real WriteObject, ReadObject, etc.
if err := s.WriteObject(hash, data); err != nil {
	t.Fatalf("WriteObject failed: %v", err)
}
```

**What to Mock:**
- Nothing in current tests — all use real implementations
- If mocking were needed, interface-based design would be required (not currently used)

**What NOT to Mock:**
- File I/O: tests use real `os.Create`, `os.Rename` on temp directories
- Hash computation: tests use real SHA-256
- Object storage: full `ObjectStore` is tested, not mocked

## Fixtures and Factories

**Test Data:**
Test helpers generate data inline rather than using fixtures:

```go
// From store_test.go
func testHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Usage in tests
data := []byte("hello world")
hash := testHash(data)
```

**Location:**
- In same `store_test.go` file, near top
- No separate fixtures directory or factory pattern
- Constants for test data: `data := []byte("test string")`

**Test Data Patterns:**

| Scenario | Pattern | Example |
|----------|---------|---------|
| Valid data | Simple byte string | `[]byte("hello world")` |
| Empty file | Zero bytes | `[]byte("")` |
| Large file | Synthesized in test | `make([]byte, 12*1024*1024)` for 12MB test |
| Temp files | `t.TempDir()` auto-cleanup | `dir := t.TempDir()` |

## Coverage

**Requirements:** No coverage enforcement in codebase (no `.coveragerc`, no CI check)

**View Coverage:**
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -html=coverage.out

# Text summary
go test -cover ./...
```

## Test Types

**Unit Tests:**
- Scope: Single package/function (e.g., `ObjectStore` methods)
- Approach: Direct function calls, real temp files, verify behavior
- Example: `TestObjectPath` verifies hash → file path mapping
- Example: `TestWriteAndHasObject` verifies write + existence check

**Integration Tests:**
- Not present in current codebase
- Would involve multiple modules communicating (e.g., client + server handshake)
- Not implemented: no end-to-end test suite

**E2E Tests:**
- Not automated
- Manual testing documented in CLAUDE.md: 15 test scenarios (file types, dedup, deletion, conflict resolution, etc.)
- Reference: HomelabSecureSync project memory lists all 15 passing tests performed manually

## Common Patterns

**Async Testing:**
Not applicable; Go's `testing` package runs tests sequentially by default. Tests that spawn goroutines use standard patterns:

```go
// If testing concurrency (not currently done):
done := make(chan error, 1)
go func() {
	// async work
	done <- err
}()
err := <-done
```

**Error Testing:**
Tests verify both success and error cases:

```go
// From store_test.go - success path
if err := s.WriteObject(hash, data); err != nil {
	t.Fatalf("WriteObject failed: %v", err)
}

// Implicit error handling: if function fails, test reports it
// No explicit error injection framework used
```

**Path Validation Testing:**
```go
// From store_test.go
func TestValidateRelPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"Documents/resume.pdf", true},
		{"file.txt", true},
		{"a/b/c/d.txt", true},
		{"", false},
		{"/absolute/path", false},
		{"../escape", false},
		{"foo/../../etc/passwd", false},
	}
	for _, tt := range tests {
		if got := ValidateRelPath(tt.path); got != tt.want {
			t.Errorf("ValidateRelPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
```

This pattern (table-driven tests) is standard in Go for testing multiple input cases.

## Test Coverage Summary

| Module | Tested | Tests | Notes |
|--------|--------|-------|-------|
| `server/internal/store` | ✓ | 11 tests | Full coverage: path logic, read/write, dedup, deletion, temp cleanup |
| `server/internal/db` | ✗ | 0 | No automated tests; SQL queried manually during development |
| `server/internal/receiver` | ✗ | 0 | Protocol handler; tested manually via TCP integration |
| `server/internal/auth` | ✗ | 0 | Token generation; tested manually in integration |
| `common/protocol` | ✗ | 0 | Packet encoding; tested indirectly via handler tests |
| `common/crypto` | ✗ | 0 | Hash computation; tested indirectly via store tests |
| `client/internal/scanner` | ✗ | 0 | Directory walking; tested manually with real filesystems |
| `client/internal/sender` | ✗ | 0 | File sending; tested manually via TCP integration |
| `client/internal/state` | ✗ | 0 | JSON persistence; tested manually with config files |
| `client/internal/config` | ✗ | 0 | TOML parsing; tested via daemon startup |
| `client/internal/status` | ✗ | 0 | Status tracking; tested via dashboard updates |
| `client/internal/sync` | ✗ | 0 | Bidirectional sync; tested manually (15 scenarios) |
| `client/internal/ui` | ✗ | 0 | HTTP server; tested manually via browser |

**Untested Modules Rationale:**
- Server-side: DB logic verified by manual SQL + integration testing; protocol handler verified by manual TCP testing
- Client-side: File I/O, sync logic, UI verified by manual testing with real TestVault directories
- No CI/CD automation currently (as of 2025-03-11)

## Test Execution

**Run All Tests:**
```bash
cd /Users/elijahatienza/Desktop/IndependentProjects/HomelabSecureSync
go test ./...
```

**Run Single Package:**
```bash
go test ./server/internal/store/...
```

**Run Specific Test:**
```bash
go test -run TestWriteObjectDedup ./server/internal/store
```

**Expected Output (sample):**
```
ok  	github.com/atienze/HomelabSecureSync/server/internal/store	0.015s
ok  	github.com/atienze/HomelabSecureSync/common	0.001s
ok  	github.com/atienze/HomelabSecureSync/client	0.001s
```

## Testing Gaps

**Known Untested Areas:**
- Database queries: `db.go` module (11 functions exported, 0 automated tests)
- Protocol handler: `receiver/handler.go` (complex state machine, manual TCP testing only)
- Client sync orchestration: `sync/bidirectional.go` (tested manually with 15 scenarios)
- Configuration loading: `config/config.go` (tested via daemon startup, no unit tests)
- Concurrency in Status: `status.go` uses `sync.RWMutex`, no concurrent access tests

**Recommended Coverage Improvements:**
- Add database integration tests (SQLite in-memory test DB)
- Add packet encoding/decoding tests for protocol types
- Add concurrent status mutation tests
- Add configuration validation tests

---

*Testing analysis: 2025-03-11*
