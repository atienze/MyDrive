package protocol

import (
	"encoding/gob"
	"io"
)

// Commands defines the protocol command codes exchanged between client and server.
// Each command corresponds to a specific operation in the sync protocol.
const (
	CmdPing       = 1
	CmdSendFile   = 2 // Upload file header (metadata)
	CmdCheckFile  = 3 // Client to server: do you need this file?
	CmdFileStatus = 4 // Server to client: response to CmdCheckFile
	CmdFileChunk  = 5 // Upload file data chunk (4MB)

	CmdDeleteFile      = 6  // Client to server: soft-delete a file
	CmdListServerFiles = 7  // Client to server: request full file manifest
	CmdServerFileList  = 8  // Server to client: file manifest response
	CmdRequestFile     = 9  // Client to server: request a file download
	CmdFileDataHeader  = 10 // Server to client: download file metadata
	CmdFileDataChunk   = 11 // Server to client: download file data (4MB)
)

// StatusResponses defines the response codes returned by the server for CmdCheckFile.
const (
	StatusNeed = 1
	StatusSkip = 2
)

// Packet is the generic envelope wrapping all protocol messages.
// Cmd identifies the operation; Payload holds the gob-encoded message body.
type Packet struct {
	Cmd     uint8
	Payload []byte
}

// FileTransfer carries the metadata header for an incoming file upload.
// It is sent before any CmdFileChunk packets so the server can open a temp
// file and initialize streaming hash verification.
type FileTransfer struct {
	RelPath string
	Hash    string
	Size    int64 // total declared size in bytes; zero means no chunks follow
}

// CheckFileRequest asks the server whether it already has a file with the
// given path and hash for the authenticated device.
type CheckFileRequest struct {
	RelPath string
	Hash    string
}

// FileStatusResponse carries the server's answer to a CmdCheckFile query.
// Status is either StatusNeed (client should send the file) or StatusSkip
// (server already has it).
type FileStatusResponse struct {
	Status uint8
}

// DeleteFileRequest asks the server to soft-delete a file by its relative path.
type DeleteFileRequest struct {
	RelPath string
}

// DeleteFileResponse carries the server's result for a CmdDeleteFile request.
type DeleteFileResponse struct {
	Success bool
	Message string
}

// ListServerFilesRequest is an empty request body for CmdListServerFiles.
// The server responds with a ServerFileListResponse containing all non-deleted files.
type ListServerFilesRequest struct{}

// ServerFileEntry is one file record in the server's file manifest.
type ServerFileEntry struct {
	RelPath  string
	Hash     string
	Size     int64
	DeviceID string // identifies which device owns this file
}

// ServerFileListResponse is the response body for CmdListServerFiles.
// Files contains all non-deleted file entries across all registered devices.
type ServerFileListResponse struct {
	Files []ServerFileEntry
}

// RequestFileRequest asks the server to stream a file by its relative path.
// If Hash is empty the server resolves the current hash from its database.
type RequestFileRequest struct {
	RelPath string
	Hash    string
}

// FileDataHeader is the metadata header sent by the server before streaming
// file data chunks in response to CmdRequestFile. An empty Hash signals
// that the file was not found.
type FileDataHeader struct {
	RelPath string
	Hash    string
	Size    int64
}

// Encoder wraps a gob.Encoder so callers work with typed Packet values
// rather than raw interface{} arguments.
type Encoder struct {
	inner *gob.Encoder
}

// NewEncoder creates an Encoder that writes gob-encoded Packet values to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{inner: gob.NewEncoder(w)}
}

// Encode writes p to the underlying gob stream.
func (e *Encoder) Encode(p Packet) error {
	return e.inner.Encode(p)
}

// Decoder wraps a gob.Decoder so callers work with typed Packet values
// rather than raw interface{} arguments.
type Decoder struct {
	inner *gob.Decoder
}

// NewDecoder creates a Decoder that reads gob-encoded Packet values from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{inner: gob.NewDecoder(r)}
}

// Decode reads the next Packet from the underlying gob stream into p.
func (d *Decoder) Decode(p *Packet) error {
	return d.inner.Decode(p)
}
