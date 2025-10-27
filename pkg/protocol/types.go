package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// Protocol constants
const (
	// Magic number for Zentalk protocol ('ZTAL')
	ProtocolMagic = 0x5A54414C

	// Protocol version
	ProtocolVersion = 0x0100 // v1.0

	// Header size
	HeaderSize = 32
)

// Message types
const (
	// Connection Management (0x00xx)
	MsgTypeHandshake    uint16 = 0x0001
	MsgTypeHandshakeAck uint16 = 0x0002
	MsgTypePing         uint16 = 0x0003
	MsgTypePong         uint16 = 0x0004
	MsgTypeDisconnect   uint16 = 0x0005

	// Relay Operations (0x01xx)
	MsgTypeRelayForward uint16 = 0x0100
	MsgTypeRelayAck     uint16 = 0x0101
	MsgTypeRelayError   uint16 = 0x0102

	// User Messages (0x02xx)
	MsgTypeDirectMessage uint16 = 0x0200
	MsgTypeGroupMessage  uint16 = 0x0201
	MsgTypeTyping        uint16 = 0x0202
	MsgTypeReadReceipt   uint16 = 0x0203
	MsgTypePresence      uint16 = 0x0204

	// Profile & Groups (0x03xx)
	MsgTypeProfileUpdate  uint16 = 0x0300
	MsgTypeProfileRequest uint16 = 0x0301
	MsgTypeGroupCreate    uint16 = 0x0302
	MsgTypeGroupJoin      uint16 = 0x0303
	MsgTypeGroupLeave     uint16 = 0x0304
	MsgTypeGroupUpdate    uint16 = 0x0305

	// Media (0x04xx)
	MsgTypeMediaUpload   uint16 = 0x0400
	MsgTypeMediaDownload uint16 = 0x0401

	// System (0x05xx)
	MsgTypeError uint16 = 0x0500
	MsgTypeAck   uint16 = 0x0501
	MsgTypeNack  uint16 = 0x0502 // Negative acknowledgment
)

// Flags
const (
	FlagEncrypted   uint16 = 0x0001 // Payload is encrypted
	FlagCompressed  uint16 = 0x0002 // Payload is compressed
	FlagFragmented  uint16 = 0x0004 // Message is fragmented
	FlagUrgent      uint16 = 0x0008 // High priority message
	FlagRequiresAck uint16 = 0x0010 // Requires acknowledgment
	FlagPadded      uint16 = 0x0020 // Message has padding (for traffic analysis resistance)
)

// Content types
const (
	ContentTypeText     uint8 = 0x00
	ContentTypeImage    uint8 = 0x01
	ContentTypeVideo    uint8 = 0x02
	ContentTypeAudio    uint8 = 0x03
	ContentTypeFile     uint8 = 0x04
	ContentTypeLocation uint8 = 0x05
	ContentTypeContact  uint8 = 0x06
	ContentTypeSticker  uint8 = 0x07
	ContentTypePoll     uint8 = 0x08
)

// Client types
const (
	ClientTypeUser  uint8 = 0x00
	ClientTypeRelay uint8 = 0x01
)

// Address represents an Ethereum address (20 bytes)
type Address [20]byte

// MessageID represents a unique message identifier (16 bytes)
type MessageID [16]byte

// GroupID represents a unique group identifier (32 bytes)
type GroupID [32]byte

// Hash represents a BLAKE2b hash (32 bytes)
type Hash [32]byte

// ===== HELPER FUNCTIONS =====

// GenerateMessageID generates a random message ID
func GenerateMessageID() MessageID {
	var id MessageID
	// Use timestamp for first 8 bytes (for uniqueness and ordering)
	timestamp := time.Now().UnixNano()
	binary.BigEndian.PutUint64(id[0:8], uint64(timestamp))

	// Use crypto/rand for secure random bytes in remaining 8 bytes
	if _, err := rand.Read(id[8:]); err != nil {
		// Fallback: use timestamp-based pseudo-random if crypto/rand fails
		// This is better than leaving zeros, though very unlikely to occur
		binary.BigEndian.PutUint64(id[8:], uint64(timestamp^0xDEADBEEF))
	}

	return id
}

// IsZeroAddress checks if address is zero
func IsZeroAddress(addr Address) bool {
	zero := Address{}
	return addr == zero
}

// NowUnixMilli returns current time in Unix milliseconds
func NowUnixMilli() int64 {
	return time.Now().UnixMilli()
}
