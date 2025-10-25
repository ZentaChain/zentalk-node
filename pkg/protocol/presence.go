package protocol

import (
	"encoding/binary"
	"fmt"
)

// ===== TYPING INDICATOR =====

// TypingIndicator represents typing status
type TypingIndicator struct {
	From      Address // Sender
	To        Address // Recipient (or group ID)
	Timestamp uint64  // Timestamp
	IsTyping  bool    // Currently typing
}

// Encode encodes typing indicator to bytes
func (t *TypingIndicator) Encode() []byte {
	buf := make([]byte, 1+20+20+8+1) // Added 1 byte for message type
	offset := 0

	// Message type identifier
	buf[offset] = 0x01 // Type: Typing Indicator
	offset++

	copy(buf[offset:], t.From[:])
	offset += 20

	copy(buf[offset:], t.To[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], t.Timestamp)
	offset += 8

	if t.IsTyping {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}

	return buf
}

// Decode decodes typing indicator from bytes
func (t *TypingIndicator) Decode(buf []byte) error {
	if len(buf) < 50 {
		return fmt.Errorf("buffer too short for typing indicator")
	}

	offset := 0

	// Check message type
	if buf[offset] != 0x01 {
		return fmt.Errorf("invalid message type for typing indicator")
	}
	offset++

	copy(t.From[:], buf[offset:offset+20])
	offset += 20

	copy(t.To[:], buf[offset:offset+20])
	offset += 20

	t.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	t.IsTyping = buf[offset] == 1

	return nil
}

// ===== PRESENCE =====

// PresenceUpdate represents online/offline status
type PresenceUpdate struct {
	Address   Address // User address
	Status    uint8   // 0=offline, 1=online, 2=away, 3=busy
	LastSeen  uint64  // Last seen timestamp
	Timestamp uint64  // Update timestamp
}

// TODO: Add Encode/Decode methods when implementing presence updates
