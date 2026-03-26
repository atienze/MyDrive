package ui

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	stdsync "sync"

	"github.com/atienze/HomelabSecureSync/client/internal/config"
	"github.com/atienze/HomelabSecureSync/client/internal/scanner"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	"github.com/atienze/HomelabSecureSync/client/internal/status"
	syncclient "github.com/atienze/HomelabSecureSync/client/internal/sync"
)

//go:embed templates/*
var templateFS embed.FS

// fileListResponse is the JSON envelope returned by the file-list endpoints.
type fileListResponse struct {
	Files []fileEntry `json:"files"`
}

// fileEntry is a single file record in a file-list response.
type fileEntry struct {
	RelPath   string `json:"rel_path"`
	Hash      string `json:"hash"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"size_human"`
	DeviceID  string `json:"device_id,omitempty"`
}

// opResponse is the JSON envelope returned by mutating endpoints.
type opResponse struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// writeJSON sets Content-Type, writes the given status code, and encodes v as JSON.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response using opResponse.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, opResponse{Ok: false, Message: msg})
}

// validateRelPath returns an error if relPath is empty, starts with "/", or
// after path.Clean would escape the root via "..".
func validateRelPath(relPath string) error {
	if relPath == "" {
		return errors.New("path is required")
	}
	if relPath[0] == '/' {
		return errors.New("path must not be absolute")
	}
	cleaned := path.Clean(relPath)
	if len(cleaned) >= 2 && cleaned[:2] == ".." {
		return errors.New("path traversal not allowed")
	}
	return nil
}

// httpStatusFromErr maps syncclient sentinel errors to appropriate HTTP status codes.
func httpStatusFromErr(err error) int {
	switch {
	case errors.Is(err, syncclient.ErrServerUnreachable):
		return http.StatusBadGateway // 502
	case errors.Is(err, syncclient.ErrTimeout):
		return http.StatusGatewayTimeout // 504
	case errors.Is(err, syncclient.ErrAuthFailed):
		return http.StatusUnauthorized // 401
	case errors.Is(err, syncclient.ErrHashMismatch):
		return http.StatusInternalServerError // 500
	default:
		return http.StatusInternalServerError // 500
	}
}

// UIServer serves the web dashboard and exposes the status/force-sync API.
type UIServer struct {
	status      *status.Status
	forceSyncCh chan<- struct{}
	cfg         *config.Config
	st          *state.LocalState
	statePath   string
	syncMu      *stdsync.Mutex
}

// NewUIServer creates a UIServer that reads from status and signals syncs on forceSyncCh.
func NewUIServer(
	s *status.Status,
	forceSyncCh chan<- struct{},
	cfg *config.Config,
	st *state.LocalState,
	statePath string,
	syncMu *stdsync.Mutex,
) *UIServer {
	return &UIServer{
		status:      s,
		forceSyncCh: forceSyncCh,
		cfg:         cfg,
		st:          st,
		statePath:   statePath,
		syncMu:      syncMu,
	}
}

// Start begins listening on the given address. Blocks until the server exits.
func (u *UIServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", u.handleDashboard)
	mux.HandleFunc("/api/status", u.handleStatus)
	mux.HandleFunc("/api/force-sync", u.handleForceSync)

	mux.HandleFunc("GET /api/files/client", u.handleClientFileList)
	mux.HandleFunc("GET /api/files/server", u.handleServerFileList)
	mux.HandleFunc("POST /api/files/upload", u.handleUpload)
	mux.HandleFunc("POST /api/files/import", u.handleImport)
	mux.HandleFunc("POST /api/files/download", u.handleDownload)
	mux.HandleFunc("POST /api/files/pull", u.handlePull)
	mux.HandleFunc("DELETE /api/files/server", u.handleDeleteServer)
	mux.HandleFunc("DELETE /api/files/client", u.handleDeleteClient)

	log.Printf("UI server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleDashboard serves the embedded dashboard HTML.
func (u *UIServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := templateFS.ReadFile("templates/dashboard.html")
	if err != nil {
		http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleStatus returns the current status as JSON.
func (u *UIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snap := u.status.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

// handleForceSync signals the daemon to run a sync cycle.
func (u *UIServer) handleForceSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Non-blocking send: if a sync is already queued, drop silently.
	select {
	case u.forceSyncCh <- struct{}{}:
	default:
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// handleClientFileList returns a JSON list of all files in the local sync directory.
// GET /api/files/client — no mutex acquisition (read-only).
func (u *UIServer) handleClientFileList(w http.ResponseWriter, r *http.Request) {
	files, err := scanner.ScanDirectoryQuiet(u.cfg.SyncDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("scan failed: %v", err))
		return
	}

	entries := make([]fileEntry, 0, len(files))
	for _, f := range files {
		entries = append(entries, fileEntry{
			RelPath:   f.Path,
			Hash:      f.Hash,
			Size:      f.Size,
			SizeHuman: status.FormatSize(f.Size),
		})
	}

	writeJSON(w, http.StatusOK, fileListResponse{Files: entries})
}

// handleServerFileList returns a JSON list of all files tracked on the server.
// GET /api/files/server — no mutex acquisition (read-only).
func (u *UIServer) handleServerFileList(w http.ResponseWriter, r *http.Request) {
	serverFiles, err := syncclient.FetchServerFileList(u.cfg)
	if err != nil {
		writeError(w, httpStatusFromErr(err), fmt.Sprintf("fetch server files failed: %v", err))
		return
	}

	entries := make([]fileEntry, 0, len(serverFiles))
	for _, e := range serverFiles {
		entries = append(entries, fileEntry{
			RelPath:   e.RelPath,
			Hash:      e.Hash,
			Size:      e.Size,
			SizeHuman: status.FormatSize(e.Size),
			DeviceID:  e.DeviceID,
		})
	}

	writeJSON(w, http.StatusOK, fileListResponse{Files: entries})
}

// handleUpload pushes one local file to the server.
// POST /api/files/upload?path=<relPath> — acquires syncMu.
func (u *UIServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if err := validateRelPath(relPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u.syncMu.Lock()
	err := syncclient.UploadSingleFile(u.cfg, u.st, u.statePath, relPath)
	u.syncMu.Unlock()

	if err != nil {
		u.status.AddActivity(fmt.Sprintf("Upload failed: %s: %v", relPath, err))
		writeError(w, httpStatusFromErr(err), fmt.Sprintf("upload failed: %v", err))
		return
	}

	u.status.AddActivity(fmt.Sprintf("Uploaded %s", relPath))
	writeJSON(w, http.StatusOK, opResponse{Ok: true, Message: "uploaded"})
}

// handleImport copies an uploaded file into sync_dir then pushes it to the server.
// POST /api/files/import?subdir=<optional rel subdir> — acquires syncMu.
func (u *UIServer) handleImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse form: %v", err))
		return
	}

	uploadedFile, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("get file: %v", err))
		return
	}
	defer uploadedFile.Close()

	subdir := r.URL.Query().Get("subdir")
	if subdir != "" {
		if err := validateRelPath(subdir); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid subdir: %v", err))
			return
		}
	}

	filename := filepath.Base(header.Filename)
	destPath := filepath.Join(u.cfg.SyncDir, subdir, filename)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create dirs: %v", err))
		return
	}

	out, err := os.Create(destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create file: %v", err))
		return
	}
	if _, err := io.Copy(out, uploadedFile); err != nil {
		out.Close()
		os.Remove(destPath)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("write file: %v", err))
		return
	}
	out.Close()

	relPath, err := filepath.Rel(u.cfg.SyncDir, destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compute rel path: %v", err))
		return
	}

	u.syncMu.Lock()
	uploadErr := syncclient.UploadSingleFile(u.cfg, u.st, u.statePath, relPath)
	u.syncMu.Unlock()

	if uploadErr != nil {
		u.status.AddActivity(fmt.Sprintf("Import upload failed: %s: %v", relPath, uploadErr))
		writeError(w, httpStatusFromErr(uploadErr), fmt.Sprintf("upload failed: %v", uploadErr))
		return
	}

	u.status.AddActivity(fmt.Sprintf("Imported %s", relPath))
	writeJSON(w, http.StatusOK, struct {
		Ok      bool   `json:"ok"`
		RelPath string `json:"rel_path"`
	}{Ok: true, RelPath: relPath})
}

// handlePull downloads a file from a specific device's namespace on the server.
// POST /api/files/pull?from=<deviceID>&path=<relPath> — acquires syncMu.
func (u *UIServer) handlePull(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if err := validateRelPath(relPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	fromDevice := r.URL.Query().Get("from")
	if fromDevice == "" {
		writeError(w, http.StatusBadRequest, "from parameter is required")
		return
	}

	u.syncMu.Lock()
	err := syncclient.PullFile(u.cfg, u.st, u.statePath, fromDevice, relPath)
	u.syncMu.Unlock()

	if err != nil {
		u.status.AddActivity(fmt.Sprintf("Pull failed: %s from %s: %v", relPath, fromDevice, err))
		writeError(w, httpStatusFromErr(err), fmt.Sprintf("pull failed: %v", err))
		return
	}

	u.status.AddActivity(fmt.Sprintf("Pulled %s from %s", relPath, fromDevice))
	writeJSON(w, http.StatusOK, opResponse{Ok: true, Message: "pulled"})
}

// handleDownload pulls one file from the server to the local sync directory.
// POST /api/files/download?path=<relPath> — acquires syncMu.
func (u *UIServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if err := validateRelPath(relPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u.syncMu.Lock()
	err := syncclient.DownloadSingleFile(u.cfg, u.st, u.statePath, relPath)
	u.syncMu.Unlock()

	if err != nil {
		u.status.AddActivity(fmt.Sprintf("Download failed: %s: %v", relPath, err))
		writeError(w, httpStatusFromErr(err), fmt.Sprintf("download failed: %v", err))
		return
	}

	u.status.AddActivity(fmt.Sprintf("Downloaded %s", relPath))
	writeJSON(w, http.StatusOK, opResponse{Ok: true, Message: "downloaded"})
}

// handleDeleteServer soft-deletes one file from the server.
// DELETE /api/files/server?path=<relPath> — acquires syncMu.
func (u *UIServer) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if err := validateRelPath(relPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u.syncMu.Lock()
	err := syncclient.DeleteServerFile(u.cfg, u.st, u.statePath, relPath)
	u.syncMu.Unlock()

	if err != nil {
		u.status.AddActivity(fmt.Sprintf("Server delete failed: %s: %v", relPath, err))
		writeError(w, httpStatusFromErr(err), fmt.Sprintf("delete from server failed: %v", err))
		return
	}

	u.status.AddActivity(fmt.Sprintf("Deleted from server: %s", relPath))
	writeJSON(w, http.StatusOK, opResponse{Ok: true, Message: "deleted from server"})
}

// handleDeleteClient removes one file from the local sync directory.
// DELETE /api/files/client?path=<relPath> — acquires syncMu.
func (u *UIServer) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if err := validateRelPath(relPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	fullPath := filepath.Join(u.cfg.SyncDir, relPath)

	u.syncMu.Lock()
	err := os.Remove(fullPath)
	if err == nil {
		u.st.RemoveFile(relPath)
		err = u.st.Save(u.statePath)
	}
	u.syncMu.Unlock()

	if err != nil {
		u.status.AddActivity(fmt.Sprintf("Delete local failed: %s: %v", relPath, err))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete local failed: %v", err))
		return
	}

	u.status.AddActivity(fmt.Sprintf("Deleted local: %s", relPath))
	writeJSON(w, http.StatusOK, opResponse{Ok: true, Message: "deleted"})
}
