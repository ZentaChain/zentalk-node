package protocol

import (
	"bytes"
	"testing"
)

func TestNewMessage(t *testing.T) {
	payload := []byte("test payload")
	msg := NewMessage(MsgTypeDirectMessage, payload)

	if msg.Header == nil {
		t.Fatal("NewMessage() returned nil header")
	}

	if msg.Header.Magic != ProtocolMagic {
		t.Errorf("Header.Magic = %x, want %x", msg.Header.Magic, ProtocolMagic)
	}

	if msg.Header.Version != ProtocolVersion {
		t.Errorf("Header.Version = %x, want %x", msg.Header.Version, ProtocolVersion)
	}

	if msg.Header.Type != MsgTypeDirectMessage {
		t.Errorf("Header.Type = %x, want %x", msg.Header.Type, MsgTypeDirectMessage)
	}

	if msg.Header.Length != uint32(len(payload)) {
		t.Errorf("Header.Length = %d, want %d", msg.Header.Length, len(payload))
	}

	if !bytes.Equal(msg.Payload, payload) {
		t.Error("Payload mismatch")
	}
}

func TestDirectMessageEncodeDecode(t *testing.T) {
	fromAddr := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	toAddr := Address{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	replyTo := GenerateMessageID()

	tests := []struct {
		name string
		msg  *DirectMessage
	}{
		{
			name: "text message",
			msg: &DirectMessage{
				From:           fromAddr,
				To:             toAddr,
				Timestamp:      uint64(NowUnixMilli()),
				SequenceNumber: 1,
				ContentType:    ContentTypeText,
				ReplyTo:        replyTo,
				Content:        []byte("Hello, World!"),
				Signature:      []byte("signature_data_here"),
			},
		},
		{
			name: "image message",
			msg: &DirectMessage{
				From:           fromAddr,
				To:             toAddr,
				Timestamp:      uint64(NowUnixMilli()),
				SequenceNumber: 2,
				ContentType:    ContentTypeImage,
				ReplyTo:        MessageID{},
				Content:        bytes.Repeat([]byte{0xFF}, 1000),
				Signature:      bytes.Repeat([]byte{0xAB}, 512),
			},
		},
		{
			name: "empty content",
			msg: &DirectMessage{
				From:           fromAddr,
				To:             toAddr,
				Timestamp:      uint64(NowUnixMilli()),
				SequenceNumber: 3,
				ContentType:    ContentTypeText,
				ReplyTo:        MessageID{},
				Content:        []byte{},
				Signature:      []byte{},
			},
		},
		{
			name: "large message",
			msg: &DirectMessage{
				From:           fromAddr,
				To:             toAddr,
				Timestamp:      uint64(NowUnixMilli()),
				SequenceNumber: 4,
				ContentType:    ContentTypeFile,
				ReplyTo:        replyTo,
				Content:        bytes.Repeat([]byte("A"), 10000),
				Signature:      bytes.Repeat([]byte{0x01}, 512),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := tt.msg.Encode()

			if len(encoded) == 0 {
				t.Fatal("Encode() returned empty bytes")
			}

			// Decode
			decoded := &DirectMessage{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			// Verify all fields
			if decoded.From != tt.msg.From {
				t.Errorf("From = %x, want %x", decoded.From, tt.msg.From)
			}
			if decoded.To != tt.msg.To {
				t.Errorf("To = %x, want %x", decoded.To, tt.msg.To)
			}
			if decoded.Timestamp != tt.msg.Timestamp {
				t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, tt.msg.Timestamp)
			}
			if decoded.SequenceNumber != tt.msg.SequenceNumber {
				t.Errorf("SequenceNumber = %d, want %d", decoded.SequenceNumber, tt.msg.SequenceNumber)
			}
			if decoded.ContentType != tt.msg.ContentType {
				t.Errorf("ContentType = %d, want %d", decoded.ContentType, tt.msg.ContentType)
			}
			if decoded.ReplyTo != tt.msg.ReplyTo {
				t.Error("ReplyTo mismatch")
			}
			if !bytes.Equal(decoded.Content, tt.msg.Content) {
				t.Errorf("Content length = %d, want %d", len(decoded.Content), len(tt.msg.Content))
			}
			if !bytes.Equal(decoded.Signature, tt.msg.Signature) {
				t.Errorf("Signature length = %d, want %d", len(decoded.Signature), len(tt.msg.Signature))
			}
		})
	}
}

func TestAckMessageEncodeDecode(t *testing.T) {
	fromAddr := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	toAddr := Address{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	msgID := GenerateMessageID()

	ack := &AckMessage{
		From:           fromAddr,
		To:             toAddr,
		MessageID:      msgID,
		SequenceNumber: 42,
		Timestamp:      uint64(NowUnixMilli()),
	}

	// Encode
	encoded := ack.Encode()

	// Verify size (20 + 20 + 16 + 8 + 8 = 72 bytes)
	if len(encoded) != 72 {
		t.Errorf("Encode() length = %d, want 72", len(encoded))
	}

	// Decode
	decoded := &AckMessage{}
	err := decoded.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	// Verify fields
	if decoded.From != ack.From {
		t.Errorf("From = %x, want %x", decoded.From, ack.From)
	}
	if decoded.To != ack.To {
		t.Errorf("To = %x, want %x", decoded.To, ack.To)
	}
	if decoded.MessageID != ack.MessageID {
		t.Error("MessageID mismatch")
	}
	if decoded.SequenceNumber != ack.SequenceNumber {
		t.Errorf("SequenceNumber = %d, want %d", decoded.SequenceNumber, ack.SequenceNumber)
	}
	if decoded.Timestamp != ack.Timestamp {
		t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, ack.Timestamp)
	}
}

func TestAckMessageDecodeTooShort(t *testing.T) {
	shortBuf := make([]byte, 50) // Too short

	ack := &AckMessage{}
	err := ack.Decode(shortBuf)
	if err == nil {
		t.Error("Decode() expected error for short buffer, got nil")
	}
}

func TestNackMessageEncodeDecode(t *testing.T) {
	fromAddr := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	toAddr := Address{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	msgID := GenerateMessageID()

	tests := []struct {
		name string
		nack *NackMessage
	}{
		{
			name: "decryption error",
			nack: &NackMessage{
				From:           fromAddr,
				To:             toAddr,
				MessageID:      msgID,
				SequenceNumber: 10,
				Timestamp:      uint64(NowUnixMilli()),
				ErrorCode:      NackErrorDecryption,
				ErrorMessage:   []byte("Decryption failed"),
			},
		},
		{
			name: "delivery error",
			nack: &NackMessage{
				From:           fromAddr,
				To:             toAddr,
				MessageID:      msgID,
				SequenceNumber: 20,
				Timestamp:      uint64(NowUnixMilli()),
				ErrorCode:      NackErrorDelivery,
				ErrorMessage:   []byte("Recipient offline"),
			},
		},
		{
			name: "empty error message",
			nack: &NackMessage{
				From:           fromAddr,
				To:             toAddr,
				MessageID:      msgID,
				SequenceNumber: 30,
				Timestamp:      uint64(NowUnixMilli()),
				ErrorCode:      NackErrorUnknown,
				ErrorMessage:   []byte{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := tt.nack.Encode()

			// Decode
			decoded := &NackMessage{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			// Verify fields
			if decoded.From != tt.nack.From {
				t.Errorf("From = %x, want %x", decoded.From, tt.nack.From)
			}
			if decoded.To != tt.nack.To {
				t.Errorf("To = %x, want %x", decoded.To, tt.nack.To)
			}
			if decoded.MessageID != tt.nack.MessageID {
				t.Error("MessageID mismatch")
			}
			if decoded.SequenceNumber != tt.nack.SequenceNumber {
				t.Errorf("SequenceNumber = %d, want %d", decoded.SequenceNumber, tt.nack.SequenceNumber)
			}
			if decoded.Timestamp != tt.nack.Timestamp {
				t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, tt.nack.Timestamp)
			}
			if decoded.ErrorCode != tt.nack.ErrorCode {
				t.Errorf("ErrorCode = %d, want %d", decoded.ErrorCode, tt.nack.ErrorCode)
			}
			if !bytes.Equal(decoded.ErrorMessage, tt.nack.ErrorMessage) {
				t.Errorf("ErrorMessage = %s, want %s", decoded.ErrorMessage, tt.nack.ErrorMessage)
			}
		})
	}
}

func TestNackMessageDecodeTooShort(t *testing.T) {
	shortBuf := make([]byte, 50) // Too short

	nack := &NackMessage{}
	err := nack.Decode(shortBuf)
	if err == nil {
		t.Error("Decode() expected error for short buffer, got nil")
	}
}

func TestGroupMessageEncodeDecode(t *testing.T) {
	fromAddr := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	groupID := GroupID{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41}

	tests := []struct {
		name string
		msg  *GroupMessage
	}{
		{
			name: "text group message",
			msg: &GroupMessage{
				From:        fromAddr,
				GroupID:     groupID,
				Timestamp:   uint64(NowUnixMilli()),
				ContentType: ContentTypeText,
				Content:     []byte("Hello group!"),
				Signature:   []byte("group_signature"),
			},
		},
		{
			name: "media group message",
			msg: &GroupMessage{
				From:        fromAddr,
				GroupID:     groupID,
				Timestamp:   uint64(NowUnixMilli()),
				ContentType: ContentTypeImage,
				Content:     bytes.Repeat([]byte{0xAB}, 500),
				Signature:   bytes.Repeat([]byte{0xCD}, 512),
			},
		},
		{
			name: "empty content",
			msg: &GroupMessage{
				From:        fromAddr,
				GroupID:     groupID,
				Timestamp:   uint64(NowUnixMilli()),
				ContentType: ContentTypeText,
				Content:     []byte{},
				Signature:   []byte{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := tt.msg.Encode()

			// Decode
			decoded := &GroupMessage{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			// Verify fields
			if decoded.From != tt.msg.From {
				t.Errorf("From = %x, want %x", decoded.From, tt.msg.From)
			}
			if decoded.GroupID != tt.msg.GroupID {
				t.Error("GroupID mismatch")
			}
			if decoded.Timestamp != tt.msg.Timestamp {
				t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, tt.msg.Timestamp)
			}
			if decoded.ContentType != tt.msg.ContentType {
				t.Errorf("ContentType = %d, want %d", decoded.ContentType, tt.msg.ContentType)
			}
			if !bytes.Equal(decoded.Content, tt.msg.Content) {
				t.Errorf("Content length = %d, want %d", len(decoded.Content), len(tt.msg.Content))
			}
			if !bytes.Equal(decoded.Signature, tt.msg.Signature) {
				t.Errorf("Signature length = %d, want %d", len(decoded.Signature), len(tt.msg.Signature))
			}
		})
	}
}

func TestReadReceiptEncodeDecode(t *testing.T) {
	fromAddr := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	toAddr := Address{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}
	msgID := GenerateMessageID()

	tests := []struct {
		name    string
		receipt *ReadReceipt
	}{
		{
			name: "delivered status",
			receipt: &ReadReceipt{
				From:       fromAddr,
				To:         toAddr,
				MessageID:  msgID,
				Timestamp:  uint64(NowUnixMilli()),
				ReadStatus: ReadStatusDelivered,
			},
		},
		{
			name: "read status",
			receipt: &ReadReceipt{
				From:       fromAddr,
				To:         toAddr,
				MessageID:  msgID,
				Timestamp:  uint64(NowUnixMilli()),
				ReadStatus: ReadStatusRead,
			},
		},
		{
			name: "seen status",
			receipt: &ReadReceipt{
				From:       fromAddr,
				To:         toAddr,
				MessageID:  msgID,
				Timestamp:  uint64(NowUnixMilli()),
				ReadStatus: ReadStatusSeen,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := tt.receipt.Encode()

			// Verify size (1 + 20 + 20 + 16 + 8 + 1 = 66 bytes)
			if len(encoded) != 66 {
				t.Errorf("Encode() length = %d, want 66", len(encoded))
			}

			// Decode
			decoded := &ReadReceipt{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			// Verify fields
			if decoded.From != tt.receipt.From {
				t.Errorf("From = %x, want %x", decoded.From, tt.receipt.From)
			}
			if decoded.To != tt.receipt.To {
				t.Errorf("To = %x, want %x", decoded.To, tt.receipt.To)
			}
			if decoded.MessageID != tt.receipt.MessageID {
				t.Error("MessageID mismatch")
			}
			if decoded.Timestamp != tt.receipt.Timestamp {
				t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, tt.receipt.Timestamp)
			}
			if decoded.ReadStatus != tt.receipt.ReadStatus {
				t.Errorf("ReadStatus = %d, want %d", decoded.ReadStatus, tt.receipt.ReadStatus)
			}
		})
	}
}

func TestReadReceiptDecodeTooShort(t *testing.T) {
	shortBuf := make([]byte, 50) // Too short

	receipt := &ReadReceipt{}
	err := receipt.Decode(shortBuf)
	if err == nil {
		t.Error("Decode() expected error for short buffer, got nil")
	}
}

func TestReadReceiptDecodeInvalidType(t *testing.T) {
	// Create valid-sized buffer with wrong type
	buf := make([]byte, 66)
	buf[0] = 0xFF // Invalid type

	receipt := &ReadReceipt{}
	err := receipt.Decode(buf)
	if err == nil {
		t.Error("Decode() expected error for invalid message type, got nil")
	}
}

func TestMessageEncodeDecodeConsistency(t *testing.T) {
	// Test that encode/decode is deterministic and consistent
	fromAddr := Address{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	toAddr := Address{21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40}

	msg := &DirectMessage{
		From:           fromAddr,
		To:             toAddr,
		Timestamp:      12345678,
		SequenceNumber: 99,
		ContentType:    ContentTypeText,
		ReplyTo:        MessageID{},
		Content:        []byte("consistency test"),
		Signature:      []byte("sig"),
	}

	// Encode twice
	encoded1 := msg.Encode()
	encoded2 := msg.Encode()

	// Should produce identical results
	if !bytes.Equal(encoded1, encoded2) {
		t.Error("Encode() not deterministic")
	}

	// Decode and re-encode
	decoded := &DirectMessage{}
	decoded.Decode(encoded1)
	reencoded := decoded.Encode()

	// Should match original
	if !bytes.Equal(encoded1, reencoded) {
		t.Error("Encode/Decode roundtrip not consistent")
	}
}

func TestNackErrorCodes(t *testing.T) {
	// Test that error codes are defined and unique
	codes := []uint8{
		NackErrorDecryption,
		NackErrorDelivery,
		NackErrorInvalidSeq,
		NackErrorTimeout,
		NackErrorUnknown,
	}

	// Check all are non-zero (except potentially some valid edge cases)
	for i, code := range codes {
		if code == 0 && i < len(codes)-1 {
			t.Errorf("Error code %d is zero", i)
		}
	}

	// Check uniqueness
	seen := make(map[uint8]bool)
	for i, code := range codes {
		if seen[code] {
			t.Errorf("Duplicate error code %x at index %d", code, i)
		}
		seen[code] = true
	}
}

func TestReadStatusConstants(t *testing.T) {
	// Verify read status constants are sequential
	if ReadStatusDelivered != 0 {
		t.Errorf("ReadStatusDelivered = %d, want 0", ReadStatusDelivered)
	}
	if ReadStatusRead != 1 {
		t.Errorf("ReadStatusRead = %d, want 1", ReadStatusRead)
	}
	if ReadStatusSeen != 2 {
		t.Errorf("ReadStatusSeen = %d, want 2", ReadStatusSeen)
	}
}
