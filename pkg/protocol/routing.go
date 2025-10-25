package protocol

import "encoding/binary"

// ===== HANDSHAKE =====

// HandshakeMessage represents a connection handshake
type HandshakeMessage struct {
	ProtocolVersion uint16  // Protocol version
	Address         Address // ETH address
	PublicKey       []byte  // RSA public key
	ClientType      uint8   // User or relay
	Timestamp       uint64  // Unix timestamp (ms)
	Signature       []byte  // Signature
}

// Encode encodes handshake to bytes
func (m *HandshakeMessage) Encode() []byte {
	size := 2 + 20 + 4 + len(m.PublicKey) + 1 + 8 + 4 + len(m.Signature)
	buf := make([]byte, size)
	offset := 0

	binary.BigEndian.PutUint16(buf[offset:], m.ProtocolVersion)
	offset += 2

	copy(buf[offset:], m.Address[:])
	offset += 20

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.PublicKey)))
	offset += 4

	copy(buf[offset:], m.PublicKey)
	offset += len(m.PublicKey)

	buf[offset] = m.ClientType
	offset++

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Signature)))
	offset += 4

	copy(buf[offset:], m.Signature)

	return buf
}

// Decode decodes handshake from bytes
func (m *HandshakeMessage) Decode(buf []byte) error {
	offset := 0

	m.ProtocolVersion = binary.BigEndian.Uint16(buf[offset:])
	offset += 2

	copy(m.Address[:], buf[offset:offset+20])
	offset += 20

	pkLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.PublicKey = make([]byte, pkLen)
	copy(m.PublicKey, buf[offset:offset+int(pkLen)])
	offset += int(pkLen)

	m.ClientType = buf[offset]
	offset++

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	sigLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Signature = make([]byte, sigLen)
	copy(m.Signature, buf[offset:offset+int(sigLen)])

	return nil
}

// ===== RELAY FORWARD =====

// RelayForward represents a message being forwarded through relays
type RelayForward struct {
	NextHop     Address // Next relay address (or zero for final delivery)
	TTL         uint8   // Time to live (hops remaining)
	Payload     []byte  // Encrypted next layer
	PayloadHash Hash    // BLAKE2b hash for integrity
}

// Encode encodes relay forward to bytes
func (m *RelayForward) Encode() []byte {
	size := 20 + 1 + 4 + len(m.Payload) + 32
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.NextHop[:])
	offset += 20

	buf[offset] = m.TTL
	offset++

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Payload)))
	offset += 4

	copy(buf[offset:], m.Payload)
	offset += len(m.Payload)

	copy(buf[offset:], m.PayloadHash[:])

	return buf
}

// Decode decodes relay forward from bytes
func (m *RelayForward) Decode(buf []byte) error {
	offset := 0

	copy(m.NextHop[:], buf[offset:offset+20])
	offset += 20

	m.TTL = buf[offset]
	offset++

	payloadLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Payload = make([]byte, payloadLen)
	copy(m.Payload, buf[offset:offset+int(payloadLen)])
	offset += int(payloadLen)

	copy(m.PayloadHash[:], buf[offset:offset+32])

	return nil
}
