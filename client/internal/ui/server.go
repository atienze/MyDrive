package ui

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/atienze/HomelabSecureSync/client/internal/status"
)

//go:embed templates/*
var templateFS embed.FS

// UIServer serves the web dashboard and exposes the status/force-sync API.
type UIServer struct {
	status      *status.Status
	forceSyncCh chan<- struct{}
}

// NewUIServer creates a UIServer that reads from status and signals syncs on forceSyncCh.
func NewUIServer(s *status.Status, forceSyncCh chan<- struct{}) *UIServer {
	return &UIServer{
		status:      s,
		forceSyncCh: forceSyncCh,
	}
}

// Start begins listening on the given address. Blocks until the server exits.
func (u *UIServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", u.handleDashboard)
	mux.HandleFunc("/api/status", u.handleStatus)
	mux.HandleFunc("/api/force-sync", u.handleForceSync)

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
