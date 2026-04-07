package status

import (
	"fmt"
	"sync"
	"time"
)

const maxActivities = 50

// ActivityEntry is one line in the activity log.
type ActivityEntry struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

// StatusSnapshot is a read-only copy of Status for JSON marshaling.
type StatusSnapshot struct {
	Connected       bool            `json:"connected"`
	Syncing         bool            `json:"syncing"`
	LastSyncTime    time.Time       `json:"last_sync_time"`
	LastSyncError   string          `json:"last_sync_error"`
	LastSyncUp      int             `json:"last_sync_uploaded"`
	LastSyncDown    int             `json:"last_sync_downloaded"`
	LastSyncDeleted int             `json:"last_sync_deleted"`
	TotalFiles      int             `json:"total_files"`
	TotalSize       int64           `json:"total_size"`
	Activities      []ActivityEntry `json:"activities"`
	DeviceName      string          `json:"device_name,omitempty"`
}

// Status is the thread-safe shared state between the daemon loop and the UI.
type Status struct {
	mu              sync.RWMutex
	connected       bool
	syncing         bool
	lastSyncTime    time.Time
	lastSyncError   string
	lastSyncUp      int
	lastSyncDown    int
	lastSyncDeleted int
	totalFiles      int
	totalSize       int64
	activities      []ActivityEntry
	deviceName      string
}

// New creates a new Status instance.
func New() *Status {
	return &Status{}
}

// SetDeviceName stores the device name so it can be surfaced in status snapshots.
func (s *Status) SetDeviceName(name string) {
	s.mu.Lock()
	s.deviceName = name
	s.mu.Unlock()
}

// AddActivity prepends an entry to the activity log, capped at 50.
func (s *Status) AddActivity(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := ActivityEntry{
		Time:    time.Now(),
		Message: msg,
	}
	s.activities = append([]ActivityEntry{entry}, s.activities...)
	if len(s.activities) > maxActivities {
		s.activities = s.activities[:maxActivities]
	}
}

// SetSyncing sets whether a sync is currently in progress.
func (s *Status) SetSyncing(v bool) {
	s.mu.Lock()
	s.syncing = v
	s.mu.Unlock()
}

// SetConnected sets the server connection status.
func (s *Status) SetConnected(v bool) {
	s.mu.Lock()
	s.connected = v
	s.mu.Unlock()
}

// SetStorageStats updates the total file count and size.
func (s *Status) SetStorageStats(files int, size int64) {
	s.mu.Lock()
	s.totalFiles = files
	s.totalSize = size
	s.mu.Unlock()
}

// SetLastSync records the results of a completed sync cycle.
func (s *Status) SetLastSync(uploaded, downloaded int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastSyncTime = time.Now()
	s.lastSyncUp = uploaded
	s.lastSyncDown = downloaded
	s.lastSyncDeleted = 0

	if err != nil {
		s.lastSyncError = err.Error()
	} else {
		s.lastSyncError = ""
	}
}

// Snapshot returns a deep copy of the current status for safe reading.
func (s *Status) Snapshot() StatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Deep copy the activities slice.
	acts := make([]ActivityEntry, len(s.activities))
	copy(acts, s.activities)

	return StatusSnapshot{
		Connected:       s.connected,
		Syncing:         s.syncing,
		LastSyncTime:    s.lastSyncTime,
		LastSyncError:   s.lastSyncError,
		LastSyncUp:      s.lastSyncUp,
		LastSyncDown:    s.lastSyncDown,
		LastSyncDeleted: s.lastSyncDeleted,
		TotalFiles:      s.totalFiles,
		TotalSize:       s.totalSize,
		Activities:      acts,
		DeviceName:      s.deviceName,
	}
}

// FormatSize returns a human-readable file size string.
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
