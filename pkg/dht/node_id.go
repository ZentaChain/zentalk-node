package dht

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"math/big"
)

// NodeID is a 160-bit Kademlia node identifier (SHA-1 hash)
type NodeID [20]byte

// ZeroID returns a zero node ID
func ZeroID() NodeID {
	return NodeID{}
}

// NewNodeID creates a NodeID from raw bytes
func NewNodeID(data []byte) NodeID {
	hash := sha1.Sum(data)
	return NodeID(hash)
}

// NewNodeIDFromHex creates a NodeID from a hex string
func NewNodeIDFromHex(s string) (NodeID, error) {
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return NodeID{}, err
	}
	var id NodeID
	copy(id[:], bytes)
	return id, nil
}

// RandomNodeID generates a random node ID (for testing)
func RandomNodeID() NodeID {
	var id NodeID
	rand.Read(id[:])
	return id
}

// String returns the hex representation of the node ID
func (id NodeID) String() string {
	return hex.EncodeToString(id[:])
}

// Equals checks if two node IDs are equal
func (id NodeID) Equals(other NodeID) bool {
	return id == other
}

// Less checks if this ID is less than another (for sorting)
func (id NodeID) Less(other NodeID) bool {
	for i := 0; i < 20; i++ {
		if id[i] < other[i] {
			return true
		}
		if id[i] > other[i] {
			return false
		}
	}
	return false
}

// Xor returns the XOR distance between two node IDs
func (id NodeID) Xor(other NodeID) NodeID {
	var result NodeID
	for i := 0; i < 20; i++ {
		result[i] = id[i] ^ other[i]
	}
	return result
}

// Distance returns the XOR distance as a big.Int
func (id NodeID) Distance(other NodeID) *big.Int {
	xor := id.Xor(other)
	return new(big.Int).SetBytes(xor[:])
}

// CommonPrefixLen returns the number of leading bits that match
// This is used to determine which k-bucket a node belongs to
func (id NodeID) CommonPrefixLen(other NodeID) int {
	xor := id.Xor(other)

	// Count leading zero bits
	for i := 0; i < 20; i++ {
		if xor[i] != 0 {
			// Found first non-zero byte
			// Count leading zeros in this byte
			b := xor[i]
			for j := 7; j >= 0; j-- {
				if (b & (1 << uint(j))) != 0 {
					return i*8 + (7 - j)
				}
			}
		}
	}

	// All bits match
	return 160
}

// PrefixLen returns the length of the prefix (number of leading zero bits in XOR)
// Used to determine k-bucket index
func (id NodeID) PrefixLen(other NodeID) int {
	xor := id.Xor(other)

	// Count leading zero bits
	prefixLen := 0
	for i := 0; i < 20; i++ {
		b := xor[i]
		if b == 0 {
			prefixLen += 8
		} else {
			// Count leading zeros in this byte
			for j := 7; j >= 0; j-- {
				if (b & (1 << uint(j))) != 0 {
					return prefixLen
				}
				prefixLen++
			}
			return prefixLen
		}
	}
	return prefixLen
}

// CloserTo returns true if this ID is closer to target than other is
func (id NodeID) CloserTo(target, other NodeID) bool {
	for i := 0; i < 20; i++ {
		d1 := id[i] ^ target[i]
		d2 := other[i] ^ target[i]
		if d1 < d2 {
			return true
		}
		if d1 > d2 {
			return false
		}
	}
	return false
}
