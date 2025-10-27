package protocol

import (
	"testing"
)

func TestAddPaddingFixedSize(t *testing.T) {
	testCases := []struct {
		name           string
		inputSize      int
		expectedSize   int
		scheme         PaddingScheme
	}{
		{"Small message to 512", 100, 512, PaddingFixedSize},
		{"Medium message to 1024", 600, 1024, PaddingFixedSize},
		{"Large message to 4096", 2000, 4096, PaddingFixedSize},
		{"Very large message to 8192", 5000, 8192, PaddingFixedSize},
		{"Exact 512", 512, 512, PaddingFixedSize},
		{"Just over 512", 513, 1024, PaddingFixedSize},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test message
			originalPayload := make([]byte, tc.inputSize)
			msg := NewMessage(MsgTypeDirectMessage, originalPayload)

			// Add padding
			padded, err := AddMessagePadding(msg, tc.scheme)
			if err != nil {
				t.Fatalf("AddMessagePadding failed: %v", err)
			}

			// Check padded flag is set
			if !padded.Header.HasFlag(FlagPadded) {
				t.Error("FlagPadded not set after padding")
			}

			// Check size (4 bytes for length prefix + padded data)
			expectedTotal := 4 + tc.expectedSize
			if len(padded.Payload) != expectedTotal {
				t.Errorf("Padded size mismatch: got %d, want %d", len(padded.Payload), expectedTotal)
			}

			t.Logf("✅ %s: %d → %d bytes (padded to %d)", tc.name, tc.inputSize, len(padded.Payload), tc.expectedSize)
		})
	}
}

func TestRemovePadding(t *testing.T) {
	originalPayload := []byte("Hello, this is a test message!")
	originalLen := len(originalPayload)

	msg := NewMessage(MsgTypeDirectMessage, originalPayload)

	// Add padding
	padded, err := AddMessagePadding(msg, PaddingFixedSize)
	if err != nil {
		t.Fatalf("AddMessagePadding failed: %v", err)
	}

	t.Logf("Original: %d bytes, Padded: %d bytes", originalLen, len(padded.Payload))

	// Remove padding
	unpadded, err := RemoveMessagePadding(padded)
	if err != nil {
		t.Fatalf("RemoveMessagePadding failed: %v", err)
	}

	// Check padded flag is cleared
	if unpadded.Header.HasFlag(FlagPadded) {
		t.Error("FlagPadded still set after unpadding")
	}

	// Check payload matches original
	if string(unpadded.Payload) != string(originalPayload) {
		t.Errorf("Payload mismatch after unpadding:\ngot:  %s\nwant: %s", unpadded.Payload, originalPayload)
	}

	t.Logf("✅ Successfully removed padding: %d → %d bytes", len(padded.Payload), len(unpadded.Payload))
}

func TestPaddingRoundTrip(t *testing.T) {
	testMessages := [][]byte{
		[]byte("Short"),
		[]byte("Medium length message for testing"),
		[]byte("A much longer message that should be padded to a larger cell size for better privacy and security"),
		make([]byte, 3000), // Large message
	}

	for i, original := range testMessages {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			msg := NewMessage(MsgTypeDirectMessage, original)

			// Add padding
			padded, err := AddMessagePadding(msg, PaddingFixedSize)
			if err != nil {
				t.Fatalf("AddMessagePadding failed: %v", err)
			}

			// Remove padding
			unpadded, err := RemoveMessagePadding(padded)
			if err != nil {
				t.Fatalf("RemoveMessagePadding failed: %v", err)
			}

			// Verify original data is intact
			if string(unpadded.Payload) != string(original) {
				t.Errorf("Round-trip failed: data corrupted")
			}

			t.Logf("✅ Round-trip OK: %d → %d → %d bytes", len(original), len(padded.Payload), len(unpadded.Payload))
		})
	}
}

func TestNoPadding(t *testing.T) {
	payload := []byte("Test message")
	msg := NewMessage(MsgTypeDirectMessage, payload)

	// Add no padding
	result, err := AddMessagePadding(msg, PaddingNone)
	if err != nil {
		t.Fatalf("AddMessagePadding failed: %v", err)
	}

	// Should return same message
	if len(result.Payload) != len(payload) {
		t.Errorf("PaddingNone changed payload size: %d → %d", len(payload), len(result.Payload))
	}

	if result.Header.HasFlag(FlagPadded) {
		t.Error("FlagPadded set with PaddingNone")
	}

	t.Logf("✅ PaddingNone preserved original: %d bytes", len(result.Payload))
}

func TestRandomPadding(t *testing.T) {
	payload := []byte("Test message")
	msg := NewMessage(MsgTypeDirectMessage, payload)

	// Add random padding
	padded, err := AddMessagePadding(msg, PaddingRandom)
	if err != nil {
		t.Fatalf("AddMessagePadding failed: %v", err)
	}

	// Should be larger than original
	if len(padded.Payload) <= len(payload) {
		t.Error("Random padding didn't increase size")
	}

	// Remove and verify
	unpadded, err := RemoveMessagePadding(padded)
	if err != nil {
		t.Fatalf("RemoveMessagePadding failed: %v", err)
	}

	if string(unpadded.Payload) != string(payload) {
		t.Error("Random padding round-trip failed")
	}

	t.Logf("✅ Random padding: %d → %d → %d bytes", len(payload), len(padded.Payload), len(unpadded.Payload))
}

func TestShouldPadMessage(t *testing.T) {
	tests := []struct {
		msgType      uint16
		shouldPad    bool
		description  string
	}{
		{MsgTypeDirectMessage, true, "Direct messages should be padded"},
		{MsgTypeGroupMessage, true, "Group messages should be padded"},
		{MsgTypeHandshake, true, "Handshakes should be padded"},
		{MsgTypePing, false, "Pings should not be padded"},
		{MsgTypePong, false, "Pongs should not be padded"},
		{MsgTypeRelayForward, false, "Relay forwards already have onion padding"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := ShouldPadMessage(tt.msgType)
			if result != tt.shouldPad {
				t.Errorf("ShouldPadMessage(%d) = %v, want %v", tt.msgType, result, tt.shouldPad)
			} else {
				t.Logf("✅ %s: %v", tt.description, result)
			}
		})
	}
}

func TestGetRecommendedPaddingScheme(t *testing.T) {
	tests := []struct {
		msgType  uint16
		expected PaddingScheme
	}{
		{MsgTypeDirectMessage, PaddingFixedSize},
		{MsgTypeGroupMessage, PaddingFixedSize},
		{MsgTypeHandshake, PaddingFixedSize},
		{MsgTypeMediaUpload, PaddingRandom},
		{MsgTypeMediaDownload, PaddingRandom},
	}

	for _, tt := range tests {
		scheme := GetRecommendedPaddingScheme(tt.msgType)
		if scheme != tt.expected {
			t.Errorf("GetRecommendedPaddingScheme(%d) = %v, want %v", tt.msgType, scheme, tt.expected)
		} else {
			t.Logf("✅ MsgType %d → Scheme %v", tt.msgType, scheme)
		}
	}
}

func BenchmarkAddPaddingSmall(b *testing.B) {
	payload := make([]byte, 100)
	msg := NewMessage(MsgTypeDirectMessage, payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = AddMessagePadding(msg, PaddingFixedSize)
	}
}

func BenchmarkAddPaddingLarge(b *testing.B) {
	payload := make([]byte, 3000)
	msg := NewMessage(MsgTypeDirectMessage, payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = AddMessagePadding(msg, PaddingFixedSize)
	}
}

func BenchmarkRemovePadding(b *testing.B) {
	payload := make([]byte, 100)
	msg := NewMessage(MsgTypeDirectMessage, payload)
	padded, _ := AddMessagePadding(msg, PaddingFixedSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RemoveMessagePadding(padded)
	}
}
