package protocol

import (
	"encoding/binary"
	"fmt"
)

// Message represents a complete protocol message
type Message struct {
	Header  *Header
	Payload []byte
}

// NewMessage creates a new message
func NewMessage(msgType uint16, payload []byte) *Message {
	return &Message{
		Header: &Header{
			Magic:     ProtocolMagic,
			Version:   ProtocolVersion,
			Type:      msgType,
			Length:    uint32(len(payload)),
			Flags:     0,
			MessageID: GenerateMessageID(),
			Reserved:  0,
		},
		Payload: payload,
	}
}

// ===== DIRECT MESSAGE =====

// DirectMessage represents a 1-to-1 message
type DirectMessage struct {
	From           Address   // Sender address
	To             Address   // Recipient address
	Timestamp      uint64    // Unix timestamp (ms)
	SequenceNumber uint64    // Message sequence number (for ordering)
	ContentType    uint8     // Content type
	ReplyTo        MessageID // Optional: message being replied to
	Content        []byte    // Encrypted content
	Signature      []byte    // Signature
}

// Encode encodes direct message to bytes
func (m *DirectMessage) Encode() []byte {
	size := 20 + 20 + 8 + 8 + 1 + 16 + 4 + len(m.Content) + 4 + len(m.Signature)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.From[:])
	offset += 20

	copy(buf[offset:], m.To[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint64(buf[offset:], m.SequenceNumber)
	offset += 8

	buf[offset] = m.ContentType
	offset++

	copy(buf[offset:], m.ReplyTo[:])
	offset += 16

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Content)))
	offset += 4

	copy(buf[offset:], m.Content)
	offset += len(m.Content)

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Signature)))
	offset += 4

	copy(buf[offset:], m.Signature)

	return buf
}

// Decode decodes direct message from bytes
func (m *DirectMessage) Decode(buf []byte) error {
	offset := 0

	copy(m.From[:], buf[offset:offset+20])
	offset += 20

	copy(m.To[:], buf[offset:offset+20])
	offset += 20

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	m.SequenceNumber = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	m.ContentType = buf[offset]
	offset++

	copy(m.ReplyTo[:], buf[offset:offset+16])
	offset += 16

	contentLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Content = make([]byte, contentLen)
	copy(m.Content, buf[offset:offset+int(contentLen)])
	offset += int(contentLen)

	sigLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Signature = make([]byte, sigLen)
	copy(m.Signature, buf[offset:offset+int(sigLen)])

	return nil
}

// ===== ACK/NACK MESSAGES =====

// AckMessage represents a message acknowledgment
type AckMessage struct {
	From           Address   // Sender of the ACK
	To             Address   // Recipient of the ACK
	MessageID      MessageID // Message being acknowledged
	SequenceNumber uint64    // Sequence number being acknowledged
	Timestamp      uint64    // Unix timestamp (ms)
}

// Encode encodes ACK message to bytes
func (a *AckMessage) Encode() []byte {
	buf := make([]byte, 20+20+16+8+8)
	offset := 0

	copy(buf[offset:], a.From[:])
	offset += 20

	copy(buf[offset:], a.To[:])
	offset += 20

	copy(buf[offset:], a.MessageID[:])
	offset += 16

	binary.BigEndian.PutUint64(buf[offset:], a.SequenceNumber)
	offset += 8

	binary.BigEndian.PutUint64(buf[offset:], a.Timestamp)

	return buf
}

// Decode decodes ACK message from bytes
func (a *AckMessage) Decode(buf []byte) error {
	if len(buf) < 72 {
		return fmt.Errorf("buffer too short for ACK message")
	}

	offset := 0

	copy(a.From[:], buf[offset:offset+20])
	offset += 20

	copy(a.To[:], buf[offset:offset+20])
	offset += 20

	copy(a.MessageID[:], buf[offset:offset+16])
	offset += 16

	a.SequenceNumber = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	a.Timestamp = binary.BigEndian.Uint64(buf[offset:])

	return nil
}

// NackMessage represents a negative acknowledgment (message error)
type NackMessage struct {
	From           Address   // Sender of the NACK
	To             Address   // Recipient of the NACK
	MessageID      MessageID // Message that failed
	SequenceNumber uint64    // Sequence number that failed
	Timestamp      uint64    // Unix timestamp (ms)
	ErrorCode      uint8     // Error code
	ErrorMessage   []byte    // Optional error description
}

// Error codes for NACK
const (
	NackErrorDecryption uint8 = 0x01 // Decryption failed
	NackErrorDelivery   uint8 = 0x02 // Delivery failed
	NackErrorInvalidSeq uint8 = 0x03 // Invalid sequence number
	NackErrorTimeout    uint8 = 0x04 // Message timeout
	NackErrorUnknown    uint8 = 0xFF // Unknown error
)

// Encode encodes NACK message to bytes
func (n *NackMessage) Encode() []byte {
	size := 20 + 20 + 16 + 8 + 8 + 1 + 2 + len(n.ErrorMessage)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], n.From[:])
	offset += 20

	copy(buf[offset:], n.To[:])
	offset += 20

	copy(buf[offset:], n.MessageID[:])
	offset += 16

	binary.BigEndian.PutUint64(buf[offset:], n.SequenceNumber)
	offset += 8

	binary.BigEndian.PutUint64(buf[offset:], n.Timestamp)
	offset += 8

	buf[offset] = n.ErrorCode
	offset++

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(n.ErrorMessage)))
	offset += 2

	copy(buf[offset:], n.ErrorMessage)

	return buf
}

// Decode decodes NACK message from bytes
func (n *NackMessage) Decode(buf []byte) error {
	if len(buf) < 75 {
		return fmt.Errorf("buffer too short for NACK message")
	}

	offset := 0

	copy(n.From[:], buf[offset:offset+20])
	offset += 20

	copy(n.To[:], buf[offset:offset+20])
	offset += 20

	copy(n.MessageID[:], buf[offset:offset+16])
	offset += 16

	n.SequenceNumber = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	n.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	n.ErrorCode = buf[offset]
	offset++

	errorMsgLen := binary.BigEndian.Uint16(buf[offset:])
	offset += 2

	if len(buf) < offset+int(errorMsgLen) {
		return fmt.Errorf("buffer too short for error message")
	}

	n.ErrorMessage = make([]byte, errorMsgLen)
	copy(n.ErrorMessage, buf[offset:offset+int(errorMsgLen)])

	return nil
}

// ===== GROUP MESSAGE =====

// GroupMessage represents a group chat message
type GroupMessage struct {
	From        Address // Sender address
	GroupID     GroupID // Group identifier
	Timestamp   uint64  // Unix timestamp (ms)
	ContentType uint8   // Content type
	Content     []byte  // Encrypted with group key
	Signature   []byte  // Signature
}

// Encode encodes group message to bytes
func (m *GroupMessage) Encode() []byte {
	size := 20 + 32 + 8 + 1 + 4 + len(m.Content) + 4 + len(m.Signature)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.From[:])
	offset += 20

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	buf[offset] = m.ContentType
	offset++

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Content)))
	offset += 4

	copy(buf[offset:], m.Content)
	offset += len(m.Content)

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Signature)))
	offset += 4

	copy(buf[offset:], m.Signature)

	return buf
}

// Decode decodes group message from bytes
func (m *GroupMessage) Decode(buf []byte) error {
	offset := 0

	copy(m.From[:], buf[offset:offset+20])
	offset += 20

	copy(m.GroupID[:], buf[offset:offset+32])
	offset += 32

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	m.ContentType = buf[offset]
	offset++

	contentLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Content = make([]byte, contentLen)
	copy(m.Content, buf[offset:offset+int(contentLen)])
	offset += int(contentLen)

	sigLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Signature = make([]byte, sigLen)
	copy(m.Signature, buf[offset:offset+int(sigLen)])

	return nil
}

// ===== READ RECEIPT =====

// ReadReceipt represents a message read acknowledgment
type ReadReceipt struct {
	From       Address   // Sender of the receipt (who read the message)
	To         Address   // Recipient of the receipt (original sender)
	MessageID  MessageID // Message that was read
	Timestamp  uint64    // When message was read (Unix timestamp ms)
	ReadStatus uint8     // 0=delivered, 1=read, 2=seen
}

// Read status constants
const (
	ReadStatusDelivered uint8 = 0
	ReadStatusRead      uint8 = 1
	ReadStatusSeen      uint8 = 2
)

// Encode encodes read receipt to bytes
func (r *ReadReceipt) Encode() []byte {
	buf := make([]byte, 1+20+20+16+8+1) // Added 1 byte for message type
	offset := 0

	// Message type identifier
	buf[offset] = 0x02 // Type: Read Receipt
	offset++

	copy(buf[offset:], r.From[:])
	offset += 20

	copy(buf[offset:], r.To[:])
	offset += 20

	copy(buf[offset:], r.MessageID[:])
	offset += 16

	binary.BigEndian.PutUint64(buf[offset:], r.Timestamp)
	offset += 8

	buf[offset] = r.ReadStatus

	return buf
}

// Decode decodes read receipt from bytes
func (r *ReadReceipt) Decode(buf []byte) error {
	if len(buf) < 66 {
		return fmt.Errorf("buffer too short for read receipt")
	}

	offset := 0

	// Check message type
	if buf[offset] != 0x02 {
		return fmt.Errorf("invalid message type for read receipt")
	}
	offset++

	copy(r.From[:], buf[offset:offset+20])
	offset += 20

	copy(r.To[:], buf[offset:offset+20])
	offset += 20

	copy(r.MessageID[:], buf[offset:offset+16])
	offset += 16

	r.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	r.ReadStatus = buf[offset]

	return nil
}
