package protocol

import (
	"encoding/gob"
	"io"
)

// CommandType tells the server what we want to do
type CommandType int

const (
	CmdPing      CommandType = iota // 0: Just checking connectivity
	CmdAuth                         // 1: Sending password/token
	CmdSendFile                     // 2: I am about to send a file
	CmdSyncBlock                    // 3: Here is a chunk of data
)

// Packet is the envelope we send over the network
type Packet struct {
	Cmd     CommandType
	Payload []byte // The actual data (e.g., filename, or file bytes)
}

// Encoder wraps the connection to send Packets easily
type Encoder struct {
	enc *gob.Encoder
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{enc: gob.NewEncoder(w)}
}

func (e *Encoder) Encode(p Packet) error {
	return e.enc.Encode(p)
}

// Decoder wraps the connection to read Packets easily
type Decoder struct {
	dec *gob.Decoder
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{dec: gob.NewDecoder(r)}
}

func (d *Decoder) Decode(p *Packet) error {
	return d.dec.Decode(p)
}