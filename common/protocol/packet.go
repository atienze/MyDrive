package protocol

import (
	"encoding/gob"
	"io"
)

// Commands
const (
	CmdPing       = 1
	CmdSendFile   = 2 // Upload file header (metadata)
	CmdCheckFile  = 3 // Client → Server: do you need this file?
	CmdFileStatus = 4 // Server → Client: response to CmdCheckFile
	CmdFileChunk  = 5 // Upload file data chunk (4MB)

	// Phase 4: Bidirectional sync
	CmdDeleteFile      = 6  // Client → Server: soft-delete a file
	CmdListServerFiles = 7  // Client → Server: request full file manifest
	CmdServerFileList  = 8  // Server → Client: file manifest response
	CmdRequestFile     = 9  // Client → Server: request a file download
	CmdFileDataHeader  = 10 // Server → Client: download file metadata
	CmdFileDataChunk   = 11 // Server → Client: download file data (4MB)
)

// Status Responses
const (
	StatusNeed = 1
	StatusSkip = 2
)

// Packet is the generic envelope
type Packet struct {
	Cmd     uint8
	Payload []byte
}

// FileTransfer is now JUST the Header (No Content field!)
type FileTransfer struct {
	RelPath string
	Hash    string
	Size    int64 // We need to know when to stop reading
}

// CheckFileRequest
type CheckFileRequest struct {
	RelPath string
	Hash    string
}

// FileStatusResponse
type FileStatusResponse struct {
	Status uint8
}

// Phase 4: Delete
type DeleteFileRequest struct {
	RelPath string
}

type DeleteFileResponse struct {
	Success bool
	Message string
}

// Phase 4: Server file listing
type ListServerFilesRequest struct{}

type ServerFileEntry struct {
	RelPath  string
	Hash     string
	Size     int64
	DeviceID string // identifies which device owns this file (PROT-01)
}

type ServerFileListResponse struct {
	Files []ServerFileEntry
}

// Phase 4: File download
type RequestFileRequest struct {
	RelPath string
	Hash    string
}

type FileDataHeader struct {
	RelPath string
	Hash    string
	Size    int64
}

// --- Helpers (Same as before) ---
type Encoder struct {
	inner *gob.Encoder
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{inner: gob.NewEncoder(w)}
}

func (e *Encoder) Encode(p Packet) error {
	return e.inner.Encode(p)
}

type Decoder struct {
	inner *gob.Decoder
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{inner: gob.NewDecoder(r)}
}

func (d *Decoder) Decode(p *Packet) error {
	return d.inner.Decode(p)
}
