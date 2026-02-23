package protocol

// Handshake is the first message sent by the client.
// Version 2: Token replaces ClientID for cryptographic device authentication.
type Handshake struct {
	MagicNumber uint32 // A safety check: "Is this actually Vault-Sync?"
	Version     uint8  // Protocol version — must match server expectation
	Token       string // 64-char hex auth token issued by vault-sync-server register
}

// CONSTANTS (The "Secret Handshake")
const (
	// We send this number first. If the server doesn't see this,
	// it hangs up immediately. It prevents random traffic (like HTTP)
	// from confusing our server.
	MagicNumber = 0xCAFEBABE // A classic hex number used in programming
	Version     = 2          // Version 2: token-based auth (was ClientID string in v1)
)