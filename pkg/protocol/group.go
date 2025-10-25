package protocol

import "encoding/binary"

// ===== GROUP CREATE =====

// GroupCreateMessage represents a request to create a new group
type GroupCreateMessage struct {
	GroupID     GroupID   // Unique group identifier
	GroupName   string    // Group name
	CreatorAddr Address   // Creator's address
	Timestamp   uint64    // Unix timestamp (ms)
	Members     []Address // Initial member addresses
}

// Encode encodes group create message to bytes
func (m *GroupCreateMessage) Encode() []byte {
	nameBytes := []byte(m.GroupName)
	size := 32 + 4 + len(nameBytes) + 20 + 8 + 4 + (len(m.Members) * 20)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(nameBytes)))
	offset += 4

	copy(buf[offset:], nameBytes)
	offset += len(nameBytes)

	copy(buf[offset:], m.CreatorAddr[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Members)))
	offset += 4

	for _, member := range m.Members {
		copy(buf[offset:], member[:])
		offset += 20
	}

	return buf
}

// Decode decodes group create message from bytes
func (m *GroupCreateMessage) Decode(buf []byte) error {
	offset := 0

	copy(m.GroupID[:], buf[offset:offset+32])
	offset += 32

	nameLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.GroupName = string(buf[offset : offset+int(nameLen)])
	offset += int(nameLen)

	copy(m.CreatorAddr[:], buf[offset:offset+20])
	offset += 20

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	memberCount := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Members = make([]Address, memberCount)
	for i := uint32(0); i < memberCount; i++ {
		copy(m.Members[i][:], buf[offset:offset+20])
		offset += 20
	}

	return nil
}

// ===== GROUP JOIN =====

// GroupJoinMessage represents a request to join a group
type GroupJoinMessage struct {
	GroupID    GroupID // Group identifier
	MemberAddr Address // Member requesting to join
	Timestamp  uint64  // Unix timestamp (ms)
	Signature  []byte  // Signature from member
}

// EncodeForSigning encodes group join message without signature (for signing)
func (m *GroupJoinMessage) EncodeForSigning() []byte {
	size := 32 + 20 + 8
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	copy(buf[offset:], m.MemberAddr[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)

	return buf
}

// Encode encodes group join message to bytes
func (m *GroupJoinMessage) Encode() []byte {
	size := 32 + 20 + 8 + 4 + len(m.Signature)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	copy(buf[offset:], m.MemberAddr[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Signature)))
	offset += 4

	copy(buf[offset:], m.Signature)

	return buf
}

// Decode decodes group join message from bytes
func (m *GroupJoinMessage) Decode(buf []byte) error {
	offset := 0

	copy(m.GroupID[:], buf[offset:offset+32])
	offset += 32

	copy(m.MemberAddr[:], buf[offset:offset+20])
	offset += 20

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	sigLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Signature = make([]byte, sigLen)
	copy(m.Signature, buf[offset:offset+int(sigLen)])

	return nil
}

// ===== GROUP LEAVE =====

// GroupLeaveMessage represents a request to leave a group
type GroupLeaveMessage struct {
	GroupID    GroupID // Group identifier
	MemberAddr Address // Member leaving
	Timestamp  uint64  // Unix timestamp (ms)
	Signature  []byte  // Signature from member
}

// EncodeForSigning encodes group leave message without signature (for signing)
func (m *GroupLeaveMessage) EncodeForSigning() []byte {
	size := 32 + 20 + 8
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	copy(buf[offset:], m.MemberAddr[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)

	return buf
}

// Encode encodes group leave message to bytes
func (m *GroupLeaveMessage) Encode() []byte {
	size := 32 + 20 + 8 + 4 + len(m.Signature)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	copy(buf[offset:], m.MemberAddr[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Signature)))
	offset += 4

	copy(buf[offset:], m.Signature)

	return buf
}

// Decode decodes group leave message from bytes
func (m *GroupLeaveMessage) Decode(buf []byte) error {
	offset := 0

	copy(m.GroupID[:], buf[offset:offset+32])
	offset += 32

	copy(m.MemberAddr[:], buf[offset:offset+20])
	offset += 20

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	sigLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Signature = make([]byte, sigLen)
	copy(m.Signature, buf[offset:offset+int(sigLen)])

	return nil
}

// ===== GROUP UPDATE =====

// GroupUpdateMessage represents a group update (name change, add/remove members, etc.)
type GroupUpdateMessage struct {
	GroupID      GroupID // Group identifier
	UpdateType   uint8   // Update type (1=name, 2=add member, 3=remove member, 4=admin change)
	UpdatedBy    Address // Who made the update
	Timestamp    uint64  // Unix timestamp (ms)
	NewGroupName string  // New group name (if UpdateType=1)
	MemberAddr   Address // Member address (if UpdateType=2 or 3)
	Signature    []byte  // Signature
}

// Update types
const (
	GroupUpdateName         uint8 = 1
	GroupUpdateAddMember    uint8 = 2
	GroupUpdateRemoveMember uint8 = 3
	GroupUpdateAdminChange  uint8 = 4
)

// EncodeForSigning encodes group update message without signature (for signing)
func (m *GroupUpdateMessage) EncodeForSigning() []byte {
	nameBytes := []byte(m.NewGroupName)
	size := 32 + 1 + 20 + 8 + 4 + len(nameBytes) + 20
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	buf[offset] = m.UpdateType
	offset++

	copy(buf[offset:], m.UpdatedBy[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(nameBytes)))
	offset += 4

	copy(buf[offset:], nameBytes)
	offset += len(nameBytes)

	copy(buf[offset:], m.MemberAddr[:])

	return buf
}

// Encode encodes group update message to bytes
func (m *GroupUpdateMessage) Encode() []byte {
	nameBytes := []byte(m.NewGroupName)
	size := 32 + 1 + 20 + 8 + 4 + len(nameBytes) + 20 + 4 + len(m.Signature)
	buf := make([]byte, size)
	offset := 0

	copy(buf[offset:], m.GroupID[:])
	offset += 32

	buf[offset] = m.UpdateType
	offset++

	copy(buf[offset:], m.UpdatedBy[:])
	offset += 20

	binary.BigEndian.PutUint64(buf[offset:], m.Timestamp)
	offset += 8

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(nameBytes)))
	offset += 4

	copy(buf[offset:], nameBytes)
	offset += len(nameBytes)

	copy(buf[offset:], m.MemberAddr[:])
	offset += 20

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(m.Signature)))
	offset += 4

	copy(buf[offset:], m.Signature)

	return buf
}

// Decode decodes group update message from bytes
func (m *GroupUpdateMessage) Decode(buf []byte) error {
	offset := 0

	copy(m.GroupID[:], buf[offset:offset+32])
	offset += 32

	m.UpdateType = buf[offset]
	offset++

	copy(m.UpdatedBy[:], buf[offset:offset+20])
	offset += 20

	m.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	nameLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.NewGroupName = string(buf[offset : offset+int(nameLen)])
	offset += int(nameLen)

	copy(m.MemberAddr[:], buf[offset:offset+20])
	offset += 20

	sigLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	m.Signature = make([]byte, sigLen)
	copy(m.Signature, buf[offset:offset+int(sigLen)])

	return nil
}
