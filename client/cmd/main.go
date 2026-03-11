package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	stdsync "sync"
	"syscall"
	"time"

	"github.com/atienze/HomelabSecureSync/client/internal/config"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	"github.com/atienze/HomelabSecureSync/client/internal/status"
	bisync "github.com/atienze/HomelabSecureSync/client/internal/sync"
	"github.com/atienze/HomelabSecureSync/client/internal/ui"
	"github.com/atienze/HomelabSecureSync/common/protocol"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: vault-sync <sync|daemon>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "sync":
		runSync()
	case "daemon":
		runDaemon()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: vault-sync <sync|daemon>\n", os.Args[1])
		os.Exit(1)
	}
}

// runSync performs a single full bidirectional sync cycle and exits.
func runSync() {
	fmt.Println("--- VaultSync: One-Shot Sync ---")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	uploaded, downloaded, deleted, err := runSyncCycle(cfg)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}

	fmt.Println("\n--- Sync Complete ---")
	fmt.Printf("Uploaded:   %d\n", uploaded)
	fmt.Printf("Downloaded: %d\n", downloaded)
	fmt.Printf("Deleted:    %d\n", deleted)
}

// runDaemon starts the web UI, performs an initial sync, then waits for
// "Sync Now" triggers from the dashboard or a shutdown signal.
func runDaemon() {
	fmt.Println("--- VaultSync: Daemon Mode ---")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Shared state between the sync loop and the web UI.
	appStatus := status.New()
	forceSyncCh := make(chan struct{}, 1)
	var syncMu stdsync.Mutex

	// Load local state so the UI server can update it on per-file operations.
	statePath, err := config.StatePath()
	if err != nil {
		log.Fatalf("Resolve state path: %v", err)
	}
	st, err := state.Load(statePath)
	if err != nil {
		log.Fatalf("Load state: %v", err)
	}

	// Start the web UI server.
	uiServer := ui.NewUIServer(appStatus, forceSyncCh, cfg, st, statePath, &syncMu)
	go func() {
		if err := uiServer.Start("127.0.0.1:9876"); err != nil {
			log.Fatalf("UI server failed: %v", err)
		}
	}()
	fmt.Println("Dashboard: http://localhost:9876")

	// Open browser (best-effort, non-fatal on error).
	exec.Command("open", "http://localhost:9876").Start()

	// Initial sync — non-fatal on error so the daemon stays alive.
	appStatus.AddActivity("Daemon started, running initial sync...")
	syncMu.Lock()
	doSyncCycle(cfg, appStatus)
	syncMu.Unlock()

	// Wait for force-sync triggers or shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Daemon running. Press Ctrl+C to stop.")
	for {
		select {
		case <-forceSyncCh:
			syncMu.Lock()
			doSyncCycle(cfg, appStatus)
			syncMu.Unlock()
		case <-sigCh:
			fmt.Println("\nShutting down.")
			return
		}
	}
}

// doSyncCycle runs one sync cycle and updates the shared status.
// Errors are recorded in status for the UI — this function never exits the daemon.
func doSyncCycle(cfg *config.Config, appStatus *status.Status) {
	appStatus.SetSyncing(true)
	appStatus.AddActivity("Starting sync cycle...")

	uploaded, downloaded, deleted, err := runSyncCycle(cfg)

	appStatus.SetSyncing(false)
	appStatus.SetLastSync(uploaded, downloaded, deleted, err)

	if err != nil {
		appStatus.SetConnected(false)
		appStatus.AddActivity(fmt.Sprintf("Sync failed: %v", err))
		log.Printf("Sync failed: %v", err)
	} else {
		appStatus.SetConnected(true)
		appStatus.AddActivity(fmt.Sprintf(
			"Sync complete: %d uploaded, %d downloaded, %d deleted",
			uploaded, downloaded, deleted,
		))
		updateStorageStats(cfg, appStatus)
	}
}

// updateStorageStats reads state.json and computes file count + total size.
func updateStorageStats(cfg *config.Config, appStatus *status.Status) {
	statePath, err := config.StatePath()
	if err != nil {
		return
	}
	st, err := state.Load(statePath)
	if err != nil {
		return
	}

	totalFiles := len(st.Files)
	var totalSize int64
	for relPath := range st.Files {
		fullPath := filepath.Join(cfg.SyncDir, relPath)
		if info, err := os.Stat(fullPath); err == nil {
			totalSize += info.Size()
		}
	}
	appStatus.SetStorageStats(totalFiles, totalSize)
}

// runSyncCycle opens a connection, performs a full bidirectional sync, and closes.
// Each cycle gets a fresh TCP connection to avoid stale connection issues.
func runSyncCycle(cfg *config.Config) (uploaded, downloaded, deleted int, err error) {
	fmt.Printf("Sync dir: %s\n", cfg.SyncDir)
	fmt.Printf("Server:   %s\n", cfg.ServerAddr)

	// Load local state (tracks what we synced last time).
	statePath, err := config.StatePath()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("resolve state path: %w", err)
	}
	st, err := state.Load(statePath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("load state: %w", err)
	}

	// Connect to the server.
	conn, err := net.Dial("tcp", cfg.ServerAddr)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("connect to server: %w", err)
	}
	defer conn.Close()

	encoder := gob.NewEncoder(conn)
	decoder := protocol.NewDecoder(conn)

	// Handshake.
	shake := protocol.Handshake{
		MagicNumber: protocol.MagicNumber,
		Version:     protocol.Version,
		Token:       cfg.Token,
	}
	if err := encoder.Encode(shake); err != nil {
		return 0, 0, 0, fmt.Errorf("handshake: %w", err)
	}

	// Run the full bidirectional sync.
	start := time.Now()
	syncer := bisync.NewSyncer(encoder, decoder, cfg.SyncDir, statePath, st)
	uploaded, downloaded, deleted, err = syncer.RunFullSync()
	if err != nil {
		return uploaded, downloaded, deleted, err
	}

	fmt.Printf("Sync took %s\n", time.Since(start))
	return uploaded, downloaded, deleted, nil
}
