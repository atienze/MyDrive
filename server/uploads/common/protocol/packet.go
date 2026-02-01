package protocol

import (
	"encoding/gob"
	"io"
)

// Commands
const (
	CmdPing     = 1
	CmdSendFile = 2
)

// Packet is the generic envelope
type Packet struct {
	Cmd     uint8
	Payload []byte // This will hold the Gob-encoded data
}

// FileTransfer is the specific data we put inside the Envelope
type FileTransfer struct {
	RelPath string // "client/main.go"
	Hash    string // "a1b2c3..."
	Content []byte // The actual file data
}

// --- Helper Tools for the Network ---

// Encoder wrapper
type Encoder struct {
	inner *gob.Encoder
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{inner: gob.NewEncoder(w)}
}

func (e *Encoder) Encode(p Packet) error {
	return e.inner.Encode(p)
}

// Decoder wrapper
type Decoder struct {
	inner *gob.Decoder
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{inner: gob.NewDecoder(r)}
}

func (d *Decoder) Decode(p *Packet) error {
	return d.inner.Decode(p)
}