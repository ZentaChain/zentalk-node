package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrInvalidMagic   = errors.New("invalid protocol magic")
	ErrInvalidVersion = errors.New("unsupported protocol version")
	ErrInvalidHeader  = errors.New("invalid header")
)

// Header represents the protocol message header
type Header struct {
	Magic     uint32    // Magic number (0x5A54414C)
	Version   uint16    // Protocol version
	Type      uint16    // Message type
	Length    uint32    // Payload length
	Flags     uint16    // Feature flags
	MessageID MessageID // Unique message ID
	Reserved  uint16    // Reserved for future use
}

// Encode encodes the header to bytes
func (h *Header) Encode() []byte {
	buf := make([]byte, HeaderSize)

	binary.BigEndian.PutUint32(buf[0:4], h.Magic)
	binary.BigEndian.PutUint16(buf[4:6], h.Version)
	binary.BigEndian.PutUint16(buf[6:8], h.Type)
	binary.BigEndian.PutUint32(buf[8:12], h.Length)
	binary.BigEndian.PutUint16(buf[12:14], h.Flags)
	copy(buf[14:30], h.MessageID[:])
	binary.BigEndian.PutUint16(buf[30:32], h.Reserved)

	return buf
}

// Decode decodes the header from bytes
func (h *Header) Decode(buf []byte) error {
	if len(buf) < HeaderSize {
		return ErrInvalidHeader
	}

	h.Magic = binary.BigEndian.Uint32(buf[0:4])
	h.Version = binary.BigEndian.Uint16(buf[4:6])
	h.Type = binary.BigEndian.Uint16(buf[6:8])
	h.Length = binary.BigEndian.Uint32(buf[8:12])
	h.Flags = binary.BigEndian.Uint16(buf[12:14])
	copy(h.MessageID[:], buf[14:30])
	h.Reserved = binary.BigEndian.Uint16(buf[30:32])

	return nil
}

// Validate validates the header
func (h *Header) Validate() error {
	if h.Magic != ProtocolMagic {
		return ErrInvalidMagic
	}

	if h.Version != ProtocolVersion {
		return ErrInvalidVersion
	}

	return nil
}

// HasFlag checks if a flag is set
func (h *Header) HasFlag(flag uint16) bool {
	return (h.Flags & flag) != 0
}

// SetFlag sets a flag
func (h *Header) SetFlag(flag uint16) {
	h.Flags |= flag
}

// ClearFlag clears a flag
func (h *Header) ClearFlag(flag uint16) {
	h.Flags &^= flag
}

// ReadHeader reads a header from an io.Reader
func ReadHeader(r io.Reader) (*Header, error) {
	buf := make([]byte, HeaderSize)

	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	header := &Header{}
	if err := header.Decode(buf); err != nil {
		return nil, err
	}

	if err := header.Validate(); err != nil {
		return nil, err
	}

	return header, nil
}

// WriteHeader writes a header to an io.Writer
func WriteHeader(w io.Writer, h *Header) error {
	buf := h.Encode()
	_, err := w.Write(buf)
	return err
}
