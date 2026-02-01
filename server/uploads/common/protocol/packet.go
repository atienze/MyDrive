package protocol

import (
	"encoding/gob"
	"io"
)

// Commands
const (
	CmdPing       = 1
	CmdSendFile   = 2 // The Header (Metadata)
	CmdCheckFile  = 3
	CmdFileStatus = 4
	CmdFileChunk  = 5 // New: A piece of the file
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