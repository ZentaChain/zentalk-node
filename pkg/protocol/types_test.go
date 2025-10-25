package protocol

import (
	"bytes"
	"testing"
	"time"
)

func TestGenerateMessageID(t *testing.T) {
	// Generate multiple message IDs
	id1 := GenerateMessageID()
	id2 := GenerateMessageID()
	id3 := GenerateMessageID()

	// IDs should be different (probabilistically)
	if id1 == id2 {
		t.Error("GenerateMessageID() produced identical IDs (collision)")
	}
	if id2 == id3 {
		t.Error("GenerateMessageID() produced identical IDs (collision)")
	}
	if id1 == id3 {
		t.Error("GenerateMessageID() produced identical IDs (collision)")
	}

	// All bytes shouldn't be zero
	zeroID := MessageID{}
	if id1 == zeroID {
		t.Error("GenerateMessageID() produced zero ID")
	}
}

func TestGenerateMessageIDTimestampOrdering(t *testing.T) {
	// Generate IDs with small time gap
	id1 := GenerateMessageID()
	time.Sleep(2 * time.Millisecond) // Small delay
	id2 := GenerateMessageID()

	// First 8 bytes should reflect timestamp ordering
	// Later ID should have equal or greater timestamp bytes
	if bytes.Compare(id1[0:8], id2[0:8]) > 0 {
		t.Error("GenerateMessageID() timestamp ordering violated")
	}
}

func TestGenerateMessageIDUniqueness(t *testing.T) {
	// Generate many IDs to test uniqueness
	ids := make(map[MessageID]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := GenerateMessageID()
		if ids[id] {
			t.Errorf("GenerateMessageID() collision detected at iteration %d", i)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("GenerateMessageID() uniqueness failed: got %d unique IDs, want %d", len(ids), count)
	}
}

func TestIsZeroAddress(t *testing.T) {
	tests := []struct {
		name string
		addr Address
		want bool
	}{
		{
			name: "zero address",
			addr: Address{},
			want: true,
		},
		{
			name: "all zeros explicit",
			addr: Address{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			want: true,
		},
		{
			name: "non-zero address",
			addr: Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			want: false,
		},
		{
			name: "single non-zero byte at start",
			addr: Address{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			want: false,
		},
		{
			name: "single non-zero byte at end",
			addr: Address{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			want: false,
		},
		{
			name: "ethereum-like address",
			addr: Address{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0xba, 0xbe, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x11, 0x22, 0x33, 0x44},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsZeroAddress(tt.addr)
			if got != tt.want {
				t.Errorf("IsZeroAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNowUnixMilli(t *testing.T) {
	// Get current time using both methods
	now1 := NowUnixMilli()
	now2 := time.Now().UnixMilli()

	// They should be very close (within 10ms)
	diff := now1 - now2
	if diff < 0 {
		diff = -diff
	}

	if diff > 10 {
		t.Errorf("NowUnixMilli() = %d, time.Now().UnixMilli() = %d, diff = %d ms (too large)", now1, now2, diff)
	}

	// Value should be reasonable (after year 2020)
	minTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if now1 < minTime {
		t.Errorf("NowUnixMilli() = %d, which is before 2020", now1)
	}
}

func TestNowUnixMilliMonotonic(t *testing.T) {
	// Test that successive calls are monotonically increasing or equal
	prev := NowUnixMilli()

	for i := 0; i < 100; i++ {
		current := NowUnixMilli()
		if current < prev {
			t.Errorf("NowUnixMilli() not monotonic: prev = %d, current = %d", prev, current)
		}
		prev = current
		time.Sleep(1 * time.Millisecond)
	}
}

func TestAddressEquality(t *testing.T) {
	addr1 := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	addr2 := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	addr3 := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 21}

	if addr1 != addr2 {
		t.Error("Identical addresses not equal")
	}

	if addr1 == addr3 {
		t.Error("Different addresses equal")
	}
}

func TestMessageIDEquality(t *testing.T) {
	// Test that MessageID can be compared
	id1 := MessageID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	id2 := MessageID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	id3 := MessageID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 17}

	if id1 != id2 {
		t.Error("Identical MessageIDs not equal")
	}

	if id1 == id3 {
		t.Error("Different MessageIDs equal")
	}
}

func TestGroupIDEquality(t *testing.T) {
	// Test that GroupID can be compared
	gid1 := GroupID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	gid2 := GroupID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	gid3 := GroupID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 33}

	if gid1 != gid2 {
		t.Error("Identical GroupIDs not equal")
	}

	if gid1 == gid3 {
		t.Error("Different GroupIDs equal")
	}
}

func TestHashEquality(t *testing.T) {
	// Test that Hash can be compared
	hash1 := Hash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	hash2 := Hash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	hash3 := Hash{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 33}

	if hash1 != hash2 {
		t.Error("Identical Hashes not equal")
	}

	if hash1 == hash3 {
		t.Error("Different Hashes equal")
	}
}

func TestProtocolConstants(t *testing.T) {
	// Test that protocol constants are reasonable
	if ProtocolMagic == 0 {
		t.Error("ProtocolMagic is zero")
	}

	if ProtocolVersion == 0 {
		t.Error("ProtocolVersion is zero")
	}

	if HeaderSize != 32 {
		t.Errorf("HeaderSize = %d, want 32", HeaderSize)
	}
}

func TestMessageTypeConstants(t *testing.T) {
	// Verify message type categories
	// Connection management should be 0x00xx
	if MsgTypeHandshake&0xFF00 != 0x0000 {
		t.Error("MsgTypeHandshake not in 0x00xx range")
	}

	// Relay operations should be 0x01xx
	if MsgTypeRelayForward&0xFF00 != 0x0100 {
		t.Error("MsgTypeRelayForward not in 0x01xx range")
	}

	// User messages should be 0x02xx
	if MsgTypeDirectMessage&0xFF00 != 0x0200 {
		t.Error("MsgTypeDirectMessage not in 0x02xx range")
	}

	// Profile & groups should be 0x03xx
	if MsgTypeProfileUpdate&0xFF00 != 0x0300 {
		t.Error("MsgTypeProfileUpdate not in 0x03xx range")
	}

	// Media should be 0x04xx
	if MsgTypeMediaUpload&0xFF00 != 0x0400 {
		t.Error("MsgTypeMediaUpload not in 0x04xx range")
	}

	// System should be 0x05xx
	if MsgTypeError&0xFF00 != 0x0500 {
		t.Error("MsgTypeError not in 0x05xx range")
	}
}

func TestFlagConstants(t *testing.T) {
	// Verify flags are bit flags (powers of 2)
	flags := []uint16{
		FlagEncrypted,
		FlagCompressed,
		FlagFragmented,
		FlagUrgent,
		FlagRequiresAck,
	}

	for i, flag := range flags {
		// Check that flag is a power of 2
		if flag&(flag-1) != 0 && flag != 0 {
			t.Errorf("Flag %d (value %x) is not a power of 2", i, flag)
		}
	}

	// Check flags are unique
	for i := 0; i < len(flags); i++ {
		for j := i + 1; j < len(flags); j++ {
			if flags[i] == flags[j] {
				t.Errorf("Flags %d and %d have same value %x", i, j, flags[i])
			}
		}
	}
}
