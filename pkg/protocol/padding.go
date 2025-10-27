package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
)

// Padding scheme types (moved here to avoid import cycle)
type PaddingScheme int

const (
	PaddingNone      PaddingScheme = 0 // No padding
	PaddingFixedSize PaddingScheme = 1 // Fixed cell size
	PaddingRandom    PaddingScheme = 2 // Random padding
)

// Cell sizes
const (
	CellSize512  = 512
	CellSize1024 = 1024
	CellSize4096 = 4096
	CellSize8192 = 8192
)

// PaddedMessage wraps a message with padding information
type PaddedMessage struct {
	OriginalLength uint32 // Length before padding
	Data           []byte // Padded data
}

// AddMessagePadding adds padding to a message payload
// Returns padded data and sets the FlagPadded flag if padding was added
func AddMessagePadding(msg *Message, scheme PaddingScheme) (*Message, error) {
	if scheme == PaddingNone {
		return msg, nil
	}

	// Add padding to payload
	paddedPayload, originalLen, err := addPadding(msg.Payload, scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to add padding: %w", err)
	}

	// Create padded message structure: [original_length(4 bytes)] + [padded_payload]
	paddedData := make([]byte, 4+len(paddedPayload))
	binary.BigEndian.PutUint32(paddedData[0:4], uint32(originalLen))
	copy(paddedData[4:], paddedPayload)

	// Update message
	paddedMsg := &Message{
		Header: &Header{
			Magic:     msg.Header.Magic,
			Version:   msg.Header.Version,
			Type:      msg.Header.Type,
			Length:    uint32(len(paddedData)),
			Flags:     msg.Header.Flags | FlagPadded, // Set padded flag
			MessageID: msg.Header.MessageID,
			Reserved:  msg.Header.Reserved,
		},
		Payload: paddedData,
	}

	return paddedMsg, nil
}

// RemoveMessagePadding removes padding from a message payload
func RemoveMessagePadding(msg *Message) (*Message, error) {
	// Check if message is padded
	if !msg.Header.HasFlag(FlagPadded) {
		// No padding, return as-is
		return msg, nil
	}

	if len(msg.Payload) < 4 {
		return nil, fmt.Errorf("padded message too short")
	}

	// Extract original length
	originalLen := binary.BigEndian.Uint32(msg.Payload[0:4])
	paddedPayload := msg.Payload[4:]

	// Remove padding
	originalPayload, err := removePadding(paddedPayload, int(originalLen))
	if err != nil {
		return nil, fmt.Errorf("failed to remove padding: %w", err)
	}

	// Return message with original payload
	unpaddedMsg := &Message{
		Header: &Header{
			Magic:     msg.Header.Magic,
			Version:   msg.Header.Version,
			Type:      msg.Header.Type,
			Length:    uint32(len(originalPayload)),
			Flags:     msg.Header.Flags &^ FlagPadded, // Clear padded flag
			MessageID: msg.Header.MessageID,
			Reserved:  msg.Header.Reserved,
		},
		Payload: originalPayload,
	}

	return unpaddedMsg, nil
}

// ShouldPadMessage determines if a message should be padded based on type
func ShouldPadMessage(msgType uint16) bool {
	switch msgType {
	case MsgTypeDirectMessage, MsgTypeGroupMessage:
		// Pad user messages (sensitive)
		return true

	case MsgTypeRelayForward:
		// Already padded at onion layer
		return false

	case MsgTypeHandshake, MsgTypeHandshakeAck:
		// Pad handshakes (hide client type)
		return true

	case MsgTypePing, MsgTypePong:
		// Don't pad keep-alive messages (performance)
		return false

	default:
		// Pad other message types by default
		return true
	}
}

// GetRecommendedPaddingScheme returns the recommended padding scheme for a message type
func GetRecommendedPaddingScheme(msgType uint16) PaddingScheme {
	switch msgType {
	case MsgTypeDirectMessage, MsgTypeGroupMessage:
		// Use fixed-size padding for user messages (most resistance to analysis)
		return PaddingFixedSize

	case MsgTypeHandshake, MsgTypeHandshakeAck:
		// Use fixed-size for handshakes
		return PaddingFixedSize

	case MsgTypeMediaUpload, MsgTypeMediaDownload:
		// Media already large, use random padding (less overhead)
		return PaddingRandom

	default:
		// Default to fixed-size
		return PaddingFixedSize
	}
}

// addPadding adds padding (internal implementation)
func addPadding(message []byte, scheme PaddingScheme) ([]byte, int, error) {
	originalLen := len(message)

	if scheme == PaddingNone {
		return message, originalLen, nil
	}

	var targetSize int

	if scheme == PaddingFixedSize {
		// Choose appropriate cell size
		switch {
		case originalLen <= CellSize512:
			targetSize = CellSize512
		case originalLen <= CellSize1024:
			targetSize = CellSize1024
		case originalLen <= CellSize4096:
			targetSize = CellSize4096
		case originalLen <= CellSize8192:
			targetSize = CellSize8192
		default:
			targetSize = ((originalLen + CellSize8192 - 1) / CellSize8192) * CellSize8192
		}
	} else {
		// Random padding: add 0-256 bytes
		var randomBytes [1]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return nil, 0, err
		}
		targetSize = originalLen + int(randomBytes[0])
	}

	paddingLen := targetSize - originalLen
	if paddingLen == 0 {
		return message, originalLen, nil
	}

	// Create padded message
	padded := make([]byte, targetSize)
	copy(padded, message)

	// Fill with random data
	if _, err := rand.Read(padded[originalLen:]); err != nil {
		return nil, 0, err
	}

	return padded, originalLen, nil
}

// removePadding removes padding (internal implementation)
func removePadding(padded []byte, originalLen int) ([]byte, error) {
	if originalLen > len(padded) || originalLen < 0 {
		return nil, errors.New("invalid padding length")
	}
	return padded[:originalLen], nil
}
