// Package protocol implements the ZenTalk messaging protocol.
//
// The protocol package defines the core message types, encoding/decoding,
// and cryptographic primitives used in the ZenTalk decentralized messaging system.
//
// # Protocol Overview
//
// ZenTalk uses a custom binary protocol with the following features:
//   - 32-byte message headers with magic number validation
//   - Multiple message types for different operations
//   - Binary encoding for efficiency
//   - Cryptographic signatures for message authentication
//   - Support for end-to-end encrypted messaging
//
// # Message Types
//
// The protocol supports several categories of messages:
//
// Connection Management (0x00xx):
//   - Handshake/HandshakeAck: Initial connection setup
//   - Ping/Pong: Keep-alive messages
//   - Disconnect: Clean connection termination
//
// Relay Operations (0x01xx):
//   - RelayForward: Forward messages through relay nodes
//   - RelayAck: Acknowledge relay delivery
//   - RelayError: Report relay errors
//
// User Messages (0x02xx):
//   - DirectMessage: 1-to-1 encrypted messages
//   - GroupMessage: Group chat messages
//   - Typing: Typing indicators
//   - ReadReceipt: Message read confirmations
//   - Presence: User online/offline status
//
// Profile & Groups (0x03xx):
//   - ProfileUpdate: User profile changes
//   - ProfileRequest: Request user profile
//   - GroupCreate/Join/Leave/Update: Group management
//
// Media (0x04xx):
//   - MediaUpload/MediaDownload: File transfer operations
//
// System (0x05xx):
//   - Ack/Nack: Message acknowledgments
//   - Error: Protocol errors
//
// # Header Format
//
// Every message starts with a 32-byte header:
//   - Magic (4 bytes): Protocol identifier (0x5A54414C = "ZTAL")
//   - Version (2 bytes): Protocol version (0x0100 = v1.0)
//   - Type (2 bytes): Message type
//   - Length (4 bytes): Payload length
//   - Flags (2 bytes): Feature flags (encrypted, compressed, etc.)
//   - MessageID (16 bytes): Unique message identifier
//   - Reserved (2 bytes): Reserved for future use
//
// # Message Encoding
//
// Messages use binary encoding with big-endian byte order:
//   - Fixed-size fields use direct binary encoding
//   - Variable-length fields are prefixed with their length (4 bytes)
//   - Arrays use fixed sizes defined in the protocol
//
// # Cryptographic Primitives
//
// The protocol uses:
//   - X3DH (Extended Triple Diffie-Hellman) for key agreement
//   - Double Ratchet for forward secrecy
//   - RSA-4096 for signatures and key encryption
//   - AES-256-GCM for message encryption
//   - BLAKE2b-256 for hashing
//
// # Usage Example
//
//	// Create a direct message
//	msg := &protocol.DirectMessage{
//	    From:      senderAddress,
//	    To:        recipientAddress,
//	    Timestamp: protocol.NowUnixMilli(),
//	    ContentType: protocol.ContentTypeText,
//	    Content:   []byte("Hello!"),
//	}
//
//	// Encode to bytes
//	encoded := msg.Encode()
//
//	// Create protocol message with header
//	protoMsg := protocol.NewMessage(protocol.MsgTypeDirectMessage, encoded)
//
//	// Send over network...
//	protocol.WriteHeader(conn, protoMsg.Header)
//	conn.Write(protoMsg.Payload)
//
// # Security Considerations
//
// The protocol is designed with security as a priority:
//   - All user messages must be end-to-end encrypted before transmission
//   - Message IDs use cryptographically secure random generation
//   - Signatures must be verified before processing
//   - Relay nodes cannot decrypt message contents
//   - Forward secrecy is maintained through the Double Ratchet algorithm
//
// # Compatibility
//
// The protocol version is currently 1.0 (0x0100). Future versions will
// maintain backward compatibility where possible. Clients should check
// the protocol version in headers and handle version mismatches gracefully.
package protocol
