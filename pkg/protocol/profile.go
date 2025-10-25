package protocol

import "encoding/binary"

// ===== PROFILE UPDATE =====

// ProfileUpdate represents a profile update
type ProfileUpdate struct {
	Address        Address   // User address
	Username       [32]byte  // Username (UTF-8, max 32 bytes)
	AvatarChunkID  uint64    // MeshStorage chunk ID for encrypted avatar
	AvatarKey      [32]byte  // AES-256 key to decrypt avatar from MeshStorage
	Bio            [256]byte // Bio text
	PublicKey      []byte    // RSA public key
	Timestamp      uint64    // Update timestamp
	Signature      []byte    // Signature
}

// EncodeForSigning encodes profile update without signature (for signing)
func (m *ProfileUpdate) EncodeForSigning() []byte {
	size := 20 + 32 + 8 + 32 + 256 + 4 + len(m.PublicKey) + 8
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.Address[:])
	offset += 20

	copy(buf[offset:], m.Username[:])
	offset += 32

	binary.BigEndian.PutUint64(buf[offset:], m.AvatarChunkID)
	offset += 8

	copy(buf[offset:], m.AvatarKey[:])
	offset += 32

	copy(buf[offset:], m.Bio[:])
	offset += 256

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.PublicKey)))
	offset += 4

	copy(buf[offset:], m.PublicKey)
	offset += len(m.PublicKey)

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)

	return buf
}

// Encode encodes profile update to bytes
func (m *ProfileUpdate) Encode() []byte {
	size := 20 + 32 + 8 + 32 + 256 + 4 + len(m.PublicKey) + 8 + 4 + len(m.Signature)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.Address[:])
	offset += 20

	copy(buf[offset:], m.Username[:])
	offset += 32

	binary.BigEndian.PutUint64(buf[offset:], m.AvatarChunkID)
	offset += 8

	copy(buf[offset:], m.AvatarKey[:])
	offset += 32

	copy(buf[offset:], m.Bio[:])
	offset += 256

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.PublicKey)))
	offset += 4

	copy(buf[offset:], m.PublicKey)
	offset += len(m.PublicKey)

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Signature)))
	offset += 4

	copy(buf[offset:], m.Signature)

	return buf
}

// Decode decodes profile update from bytes
func (m *ProfileUpdate) Decode(buf []byte) error {
	offset := 0

	copy(m.Address[:], buf[offset:offset+20])
	offset += 20

	copy(m.Username[:], buf[offset:offset+32])
	offset += 32

	m.AvatarChunkID = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	copy(m.AvatarKey[:], buf[offset:offset+32])
	offset += 32

	copy(m.Bio[:], buf[offset:offset+256])
	offset += 256

	pkLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.PublicKey = make([]byte, pkLen)
	copy(m.PublicKey, buf[offset:offset+int(pkLen)])
	offset += int(pkLen)

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	sigLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Signature = make([]byte, sigLen)
	copy(m.Signature, buf[offset:offset+int(sigLen)])

	return nil
}
