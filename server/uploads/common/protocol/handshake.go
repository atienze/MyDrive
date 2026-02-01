package protocol

// Handshake is the first message sent by the client.
// It verifies that both sides are speaking the same language.
type Handshake struct {
	MagicNumber uint32 // A safety check: "Is this actually Vault-Sync?"
	Version     uint8  // "I am using version 1.0"
	ClientID    string // "I am Elijah's Laptop"
}

// CONSTANTS (The "Secret Handshake")
const (
	// We send this number first. If the server doesn't see this,
	// it hangs up immediately. It prevents random traffic (like HTTP)
	// from confusing our server.
	MagicNumber = 0xCAFEBABE // A classic hex number used in programming
	Version     = 1          // Current protocol version
)