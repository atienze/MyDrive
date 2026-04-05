package main

import (
	"encoding/gob"
	"flag"
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

	"github.com/atienze/myDrive/client/internal/config"
	"github.com/atienze/myDrive/client/internal/state"
	"github.com/atienze/myDrive/client/internal/status"
	bisync "github.com/atienze/myDrive/client/internal/sync"
	"github.com/atienze/myDrive/client/internal/ui"
	"github.com/atienze/myDrive/common/protocol"
)

// syncDialTimeout is the TCP connection timeout for sync cycle dials.
// Matches the timeout used by DialAndHandshake in the operations package.
const syncDialTimeout = 10 * time.Second

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: mydrive <sync|daemon|pull>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "sync":
		runSync()
	case "daemon":
		runDaemon()
	case "pull":
		runPull(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: mydrive <sync|daemon|pull>\n", os.Args[1])
		os.Exit(1)
	}
}

// runSync performs a single push-only sync cycle and exits.
func runSync() {
	fmt.Println("--- myDrive: Push Sync ---")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	uploaded, err := runSyncCycle(cfg)
	if err != nil {
		log.Fatalf("Sync failed: %v", err)
	}

	fmt.Println("\n--- Sync Complete ---")
	fmt.Printf("Uploaded: %d\n", uploaded)
}

// runPull downloads a single file from a named device.
func runPull(args []string) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	fromDevice := fs.String("from", "", "Source device name (required)")
	fs.Parse(args)

	if *fromDevice == "" || fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: mydrive pull --from <device> <path>")
		os.Exit(1)
	}
	relPath := fs.Arg(0)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	statePath, err := config.StatePath()
	if err != nil {
		log.Fatalf("Resolve state path: %v", err)
	}
	st, err := state.Load(statePath)
	if err != nil {
		log.Fatalf("Load state: %v", err)
	}

	if err := bisync.PullFile(cfg, st, statePath, *fromDevice, relPath); err != nil {
		log.Fatalf("Pull failed: %v", err)
	}
	fmt.Printf("Pulled: %s (from %s)\n", relPath, *fromDevice)
}

// runDaemon starts the web UI and waits for "Full Sync" triggers from the
// dashboard or a shutdown signal. No sync runs automatically on startup.
func runDaemon() {
	fmt.Println("--- myDrive: Daemon Mode ---")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Shared state between the sync loop and the web UI.
	appStatus := status.New()
	appStatus.SetDeviceName(cfg.DeviceName)
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

	appStatus.AddActivity("Daemon started. Use 'Full Sync' to sync.")

	// Wait for force-sync triggers or shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Daemon running. Press Ctrl+C to stop.")
	for {
		select {
		case <-forceSyncCh:
			syncMu.Lock()
			doSyncCycle(cfg, appStatus, st, statePath)
			syncMu.Unlock()
		case <-sigCh:
			fmt.Println("\nShutting down.")
			return
		}
	}
}

// doSyncCycle runs one sync cycle using the shared state and updates the shared status.
// Errors are recorded in status for the UI — this function never exits the daemon.
func doSyncCycle(cfg *config.Config, appStatus *status.Status, st *state.LocalState, statePath string) {
	appStatus.SetSyncing(true)
	appStatus.AddActivity("Starting sync cycle...")

	uploaded, downloaded, err := runSyncCycleWithState(cfg, st, statePath)

	appStatus.SetSyncing(false)
	appStatus.SetLastSync(uploaded, err)

	if err != nil {
		appStatus.SetConnected(false)
		appStatus.AddActivity(fmt.Sprintf("Sync failed: %v", err))
		log.Printf("Sync failed: %v", err)
	} else {
		appStatus.SetConnected(true)
		appStatus.AddActivity(fmt.Sprintf("Sync complete: %d uploaded, %d downloaded", uploaded, downloaded))
		updateStorageStats(cfg, appStatus, st)
	}
}

// updateStorageStats computes file count + total size from the shared state.
// Uses the passed *LocalState directly — no disk reload.
func updateStorageStats(cfg *config.Config, appStatus *status.Status, st *state.LocalState) {
	keys := st.Keys()
	totalFiles := len(keys)
	var totalSize int64
	for _, relPath := range keys {
		fullPath := filepath.Join(cfg.SyncDir, relPath)
		if info, err := os.Stat(fullPath); err == nil {
			totalSize += info.Size()
		}
	}
	appStatus.SetStorageStats(totalFiles, totalSize)
}

// runSyncCycleWithState opens a connection, performs a bidirectional sync
// using the provided shared state, and closes. Used by the daemon to avoid
// per-cycle state.Load() calls and prevent races with UI state mutations.
func runSyncCycleWithState(cfg *config.Config, st *state.LocalState, statePath string) (int, int, error) {
	fmt.Printf("Sync dir: %s\n", cfg.SyncDir)
	fmt.Printf("Server:   %s\n", cfg.ServerAddr)

	// Connect to the server.
	conn, err := net.DialTimeout("tcp", cfg.ServerAddr, syncDialTimeout)
	if err != nil {
		return 0, 0, fmt.Errorf("connect to server: %w", err)
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
		return 0, 0, fmt.Errorf("handshake: %w", err)
	}

	// Run the bidirectional sync using the shared state.
	start := time.Now()
	syncer := bisync.NewSyncer(encoder, decoder, cfg.SyncDir, statePath, st, cfg)
	uploaded, downloaded, err := syncer.RunSync()
	if err != nil {
		return uploaded, downloaded, err
	}

	fmt.Printf("Sync took %s — uploaded: %d, downloaded: %d\n", time.Since(start), uploaded, downloaded)
	return uploaded, downloaded, nil
}

// runSyncCycle opens a connection, performs a push-only sync, and closes.
// Each cycle gets a fresh TCP connection to avoid stale connection issues.
func runSyncCycle(cfg *config.Config) (int, error) {
	fmt.Printf("Sync dir: %s\n", cfg.SyncDir)
	fmt.Printf("Server:   %s\n", cfg.ServerAddr)

	// Load local state (tracks what we synced last time).
	statePath, err := config.StatePath()
	if err != nil {
		return 0, fmt.Errorf("resolve state path: %w", err)
	}
	st, err := state.Load(statePath)
	if err != nil {
		return 0, fmt.Errorf("load state: %w", err)
	}

	// Connect to the server.
	conn, err := net.DialTimeout("tcp", cfg.ServerAddr, syncDialTimeout)
	if err != nil {
		return 0, fmt.Errorf("connect to server: %w", err)
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
		return 0, fmt.Errorf("handshake: %w", err)
	}

	// Run the bidirectional sync.
	start := time.Now()
	syncer := bisync.NewSyncer(encoder, decoder, cfg.SyncDir, statePath, st, cfg)
	uploaded, downloaded, err := syncer.RunSync()
	if err != nil {
		return uploaded, err
	}

	fmt.Printf("Sync took %s — uploaded: %d, downloaded: %d\n", time.Since(start), uploaded, downloaded)
	return uploaded, nil
}
