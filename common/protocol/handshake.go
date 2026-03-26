package protocol

// Handshake is the first message sent by the client after opening a TCP connection.
// The server validates the magic number, version, and token before processing any commands.
type Handshake struct {
	MagicNumber uint32 // fixed sentinel value; must equal the MagicNumber constant
	Version     uint8  // protocol version; must match the server's Version constant
	Token       string // 64-char hex auth token issued by vault-sync-server register
}

// MagicNumber is the fixed sentinel sent at the start of every connection.
// The server rejects connections that do not open with this value, preventing
// non-VaultSync traffic (such as stray HTTP requests) from being misinterpreted.
const MagicNumber = 0xCAFEBABE

// Version is the current protocol version.
// The server rejects connections whose handshake version does not match.
const Version = 3
