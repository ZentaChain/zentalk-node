package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
)

var (
	ErrInvalidPadding = errors.New("invalid padding")
)

// Standard cell sizes (like Tor)
const (
	CellSize512  = 512  // Small messages (text)
	CellSize1024 = 1024 // Medium messages
	CellSize4096 = 4096 // Large messages (images)
	CellSize8192 = 8192 // Very large messages
)

// PaddingScheme represents different padding strategies
type PaddingScheme int

const (
	// PaddingNone - No padding (original message size)
	PaddingNone PaddingScheme = iota

	// PaddingFixedSize - Pad to nearest fixed cell size
	PaddingFixedSize

	// PaddingRandom - Add random padding (less predictable)
	PaddingRandom
)

// AddPadding adds padding to a message based on the scheme
// Returns padded message and original length
func AddPadding(message []byte, scheme PaddingScheme) ([]byte, int, error) {
	originalLen := len(message)

	switch scheme {
	case PaddingNone:
		return message, originalLen, nil

	case PaddingFixedSize:
		return addFixedSizePadding(message, originalLen)

	case PaddingRandom:
		return addRandomPadding(message, originalLen)

	default:
		return nil, 0, fmt.Errorf("unknown padding scheme: %d", scheme)
	}
}

// addFixedSizePadding pads message to nearest cell size (512, 1024, 4096, 8192)
func addFixedSizePadding(message []byte, originalLen int) ([]byte, int, error) {
	var targetSize int

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
		// For very large messages, round up to nearest 8KB
		targetSize = ((originalLen + CellSize8192 - 1) / CellSize8192) * CellSize8192
	}

	paddingLen := targetSize - originalLen
	if paddingLen == 0 {
		return message, originalLen, nil
	}

	// Create padded message: [original data] + [random padding]
	padded := make([]byte, targetSize)
	copy(padded, message)

	// Fill padding with random data (makes it indistinguishable from encrypted data)
	if _, err := rand.Read(padded[originalLen:]); err != nil {
		return nil, 0, fmt.Errorf("failed to generate padding: %w", err)
	}

	return padded, originalLen, nil
}

// addRandomPadding adds 0-256 bytes of random padding
// More unpredictable but less bandwidth efficient
func addRandomPadding(message []byte, originalLen int) ([]byte, int, error) {
	// Generate random padding length (0-256 bytes)
	var randomBytes [1]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return nil, 0, fmt.Errorf("failed to generate random length: %w", err)
	}

	paddingLen := int(randomBytes[0]) // 0-255

	padded := make([]byte, originalLen+paddingLen)
	copy(padded, message)

	// Fill padding with random data
	if paddingLen > 0 {
		if _, err := rand.Read(padded[originalLen:]); err != nil {
			return nil, 0, fmt.Errorf("failed to generate padding: %w", err)
		}
	}

	return padded, originalLen, nil
}

// RemovePadding removes padding from a message
// originalLen is the length before padding
func RemovePadding(padded []byte, originalLen int) ([]byte, error) {
	if originalLen > len(padded) {
		return nil, ErrInvalidPadding
	}

	if originalLen < 0 {
		return nil, ErrInvalidPadding
	}

	return padded[:originalLen], nil
}

// EstimatePaddedSize estimates the padded size for a message
func EstimatePaddedSize(messageLen int, scheme PaddingScheme) int {
	switch scheme {
	case PaddingNone:
		return messageLen

	case PaddingFixedSize:
		switch {
		case messageLen <= CellSize512:
			return CellSize512
		case messageLen <= CellSize1024:
			return CellSize1024
		case messageLen <= CellSize4096:
			return CellSize4096
		case messageLen <= CellSize8192:
			return CellSize8192
		default:
			return ((messageLen + CellSize8192 - 1) / CellSize8192) * CellSize8192
		}

	case PaddingRandom:
		return messageLen + 128 // Average padding

	default:
		return messageLen
	}
}
