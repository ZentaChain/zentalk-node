package protocol

import (
	"bytes"
	"testing"
)

func TestHeaderEncodeDecode(t *testing.T) {
	msgID := GenerateMessageID()

	tests := []struct {
		name   string
		header *Header
	}{
		{
			name: "standard header",
			header: &Header{
				Magic:     ProtocolMagic,
				Version:   ProtocolVersion,
				Type:      MsgTypeDirectMessage,
				Length:    1024,
				Flags:     FlagEncrypted,
				MessageID: msgID,
				Reserved:  0,
			},
		},
		{
			name: "header with multiple flags",
			header: &Header{
				Magic:     ProtocolMagic,
				Version:   ProtocolVersion,
				Type:      MsgTypeGroupMessage,
				Length:    2048,
				Flags:     FlagEncrypted | FlagCompressed | FlagRequiresAck,
				MessageID: msgID,
				Reserved:  0,
			},
		},
		{
			name: "header with zero length",
			header: &Header{
				Magic:     ProtocolMagic,
				Version:   ProtocolVersion,
				Type:      MsgTypePing,
				Length:    0,
				Flags:     0,
				MessageID: msgID,
				Reserved:  0,
			},
		},
		{
			name: "relay forward header",
			header: &Header{
				Magic:     ProtocolMagic,
				Version:   ProtocolVersion,
				Type:      MsgTypeRelayForward,
				Length:    4096,
				Flags:     FlagEncrypted | FlagUrgent,
				MessageID: msgID,
				Reserved:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := tt.header.Encode()

			// Verify encoded size
			if len(encoded) != HeaderSize {
				t.Errorf("Encode() length = %d, want %d", len(encoded), HeaderSize)
			}

			// Decode
			decoded := &Header{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			// Verify all fields match
			if decoded.Magic != tt.header.Magic {
				t.Errorf("Magic = %x, want %x", decoded.Magic, tt.header.Magic)
			}
			if decoded.Version != tt.header.Version {
				t.Errorf("Version = %x, want %x", decoded.Version, tt.header.Version)
			}
			if decoded.Type != tt.header.Type {
				t.Errorf("Type = %x, want %x", decoded.Type, tt.header.Type)
			}
			if decoded.Length != tt.header.Length {
				t.Errorf("Length = %d, want %d", decoded.Length, tt.header.Length)
			}
			if decoded.Flags != tt.header.Flags {
				t.Errorf("Flags = %x, want %x", decoded.Flags, tt.header.Flags)
			}
			if decoded.MessageID != tt.header.MessageID {
				t.Errorf("MessageID mismatch")
			}
			if decoded.Reserved != tt.header.Reserved {
				t.Errorf("Reserved = %d, want %d", decoded.Reserved, tt.header.Reserved)
			}
		})
	}
}

func TestHeaderDecodeTooShort(t *testing.T) {
	shortBuf := make([]byte, HeaderSize-1)

	header := &Header{}
	err := header.Decode(shortBuf)
	if err != ErrInvalidHeader {
		t.Errorf("Decode() error = %v, want %v", err, ErrInvalidHeader)
	}
}

func TestHeaderValidate(t *testing.T) {
	tests := []struct {
		name    string
		header  *Header
		wantErr error
	}{
		{
			name: "valid header",
			header: &Header{
				Magic:   ProtocolMagic,
				Version: ProtocolVersion,
			},
			wantErr: nil,
		},
		{
			name: "invalid magic",
			header: &Header{
				Magic:   0x12345678,
				Version: ProtocolVersion,
			},
			wantErr: ErrInvalidMagic,
		},
		{
			name: "invalid version",
			header: &Header{
				Magic:   ProtocolMagic,
				Version: 0x9999,
			},
			wantErr: ErrInvalidVersion,
		},
		{
			name: "both invalid",
			header: &Header{
				Magic:   0xFFFFFFFF,
				Version: 0xFFFF,
			},
			wantErr: ErrInvalidMagic, // Should fail on magic first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.header.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestHeaderFlags(t *testing.T) {
	header := &Header{
		Flags: 0,
	}

	// Test SetFlag
	header.SetFlag(FlagEncrypted)
	if !header.HasFlag(FlagEncrypted) {
		t.Error("HasFlag(FlagEncrypted) = false after SetFlag, want true")
	}

	// Test multiple flags
	header.SetFlag(FlagCompressed)
	if !header.HasFlag(FlagEncrypted) {
		t.Error("HasFlag(FlagEncrypted) = false after setting second flag")
	}
	if !header.HasFlag(FlagCompressed) {
		t.Error("HasFlag(FlagCompressed) = false after SetFlag")
	}

	// Test HasFlag on unset flag
	if header.HasFlag(FlagUrgent) {
		t.Error("HasFlag(FlagUrgent) = true for unset flag")
	}

	// Test ClearFlag
	header.ClearFlag(FlagEncrypted)
	if header.HasFlag(FlagEncrypted) {
		t.Error("HasFlag(FlagEncrypted) = true after ClearFlag, want false")
	}

	// Verify other flag still set
	if !header.HasFlag(FlagCompressed) {
		t.Error("HasFlag(FlagCompressed) = false after clearing different flag")
	}

	// Clear all flags
	header.ClearFlag(FlagCompressed)
	if header.Flags != 0 {
		t.Errorf("Flags = %x after clearing all, want 0", header.Flags)
	}
}

func TestHeaderFlagCombinations(t *testing.T) {
	header := &Header{
		Flags: FlagEncrypted | FlagCompressed | FlagRequiresAck,
	}

	// Check all flags are set
	if !header.HasFlag(FlagEncrypted) {
		t.Error("FlagEncrypted not set")
	}
	if !header.HasFlag(FlagCompressed) {
		t.Error("FlagCompressed not set")
	}
	if !header.HasFlag(FlagRequiresAck) {
		t.Error("FlagRequiresAck not set")
	}

	// Check unset flags
	if header.HasFlag(FlagFragmented) {
		t.Error("FlagFragmented incorrectly set")
	}
	if header.HasFlag(FlagUrgent) {
		t.Error("FlagUrgent incorrectly set")
	}
}

func TestReadWriteHeader(t *testing.T) {
	msgID := GenerateMessageID()

	originalHeader := &Header{
		Magic:     ProtocolMagic,
		Version:   ProtocolVersion,
		Type:      MsgTypeDirectMessage,
		Length:    1234,
		Flags:     FlagEncrypted | FlagRequiresAck,
		MessageID: msgID,
		Reserved:  0,
	}

	// Write to buffer
	buf := &bytes.Buffer{}
	err := WriteHeader(buf, originalHeader)
	if err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}

	// Verify buffer size
	if buf.Len() != HeaderSize {
		t.Errorf("WriteHeader() buffer size = %d, want %d", buf.Len(), HeaderSize)
	}

	// Read from buffer
	readHeader, err := ReadHeader(buf)
	if err != nil {
		t.Fatalf("ReadHeader() error = %v", err)
	}

	// Verify all fields match
	if readHeader.Magic != originalHeader.Magic {
		t.Errorf("Magic = %x, want %x", readHeader.Magic, originalHeader.Magic)
	}
	if readHeader.Version != originalHeader.Version {
		t.Errorf("Version = %x, want %x", readHeader.Version, originalHeader.Version)
	}
	if readHeader.Type != originalHeader.Type {
		t.Errorf("Type = %x, want %x", readHeader.Type, originalHeader.Type)
	}
	if readHeader.Length != originalHeader.Length {
		t.Errorf("Length = %d, want %d", readHeader.Length, originalHeader.Length)
	}
	if readHeader.Flags != originalHeader.Flags {
		t.Errorf("Flags = %x, want %x", readHeader.Flags, originalHeader.Flags)
	}
	if readHeader.MessageID != originalHeader.MessageID {
		t.Error("MessageID mismatch")
	}
}

func TestReadHeaderInvalid(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty buffer",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "too short",
			data:    make([]byte, HeaderSize-1),
			wantErr: true,
		},
		{
			name: "invalid magic",
			data: func() []byte {
				h := &Header{
					Magic:     0xDEADBEEF,
					Version:   ProtocolVersion,
					Type:      MsgTypeDirectMessage,
					MessageID: GenerateMessageID(),
				}
				return h.Encode()
			}(),
			wantErr: true,
		},
		{
			name: "invalid version",
			data: func() []byte {
				h := &Header{
					Magic:     ProtocolMagic,
					Version:   0x9999,
					Type:      MsgTypeDirectMessage,
					MessageID: GenerateMessageID(),
				}
				return h.Encode()
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tt.data)
			_, err := ReadHeader(buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHeaderEncodeDecodeConsistency(t *testing.T) {
	// Create multiple headers and verify encode/decode is consistent
	for i := 0; i < 10; i++ {
		header := &Header{
			Magic:     ProtocolMagic,
			Version:   ProtocolVersion,
			Type:      uint16(i + 1),
			Length:    uint32(i * 100),
			Flags:     uint16(i),
			MessageID: GenerateMessageID(),
			Reserved:  uint16(i % 2),
		}

		// Encode
		encoded1 := header.Encode()
		encoded2 := header.Encode()

		// Multiple encodes should produce identical results
		if !bytes.Equal(encoded1, encoded2) {
			t.Errorf("Encode() not deterministic for iteration %d", i)
		}

		// Decode
		decoded := &Header{}
		decoded.Decode(encoded1)

		// Re-encode and verify
		reencoded := decoded.Encode()
		if !bytes.Equal(encoded1, reencoded) {
			t.Errorf("Encode/Decode roundtrip failed for iteration %d", i)
		}
	}
}
