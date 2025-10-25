package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// ===== DOUBLE RATCHET =====
// Implements the Double Ratchet algorithm for forward secrecy
// https://signal.org/docs/specifications/doubleratchet/

// Constants
const (
	// Key lengths
	RootKeyLen    = 32 // Root key length (256 bits)
	ChainKeyLen   = 32 // Chain key length (256 bits)
	MessageKeyLen = 32 // Message key length (256 bits)
	DHKeyLen      = 32 // X25519 public key length (256 bits)

	// KDF info strings for HKDF
	KDFRootInfo  = "ZenTalk Double Ratchet Root"
	KDFChainInfo = "ZenTalk Double Ratchet Chain"
)

// RootKey represents the root key in the ratchet
type RootKey [RootKeyLen]byte

// ChainKey represents a chain key (sending or receiving)
type ChainKey [ChainKeyLen]byte

// MessageKey represents a message encryption key
type MessageKey [MessageKeyLen]byte

// DHPublicKey represents a Diffie-Hellman public key (X25519)
type DHPublicKey [DHKeyLen]byte

// DHPrivateKey represents a Diffie-Hellman private key (X25519)
type DHPrivateKey [DHKeyLen]byte

// RatchetState represents the complete state of a Double Ratchet session
type RatchetState struct {
	// Root chain
	RootKey RootKey // RK: Root key

	// Sending chain
	SendingChainKey ChainKey // CKs: Sending chain key
	SendingMsgNum   uint32   // Ns: Number of messages in sending chain

	// Receiving chain
	ReceivingChainKey ChainKey // CKr: Receiving chain key
	ReceivingMsgNum   uint32   // Nr: Number of messages in receiving chain

	// DH ratchet state
	DHSendingPrivate  DHPrivateKey // DHs: Our current DH private key
	DHSendingPublic   DHPublicKey  // DHs_pub: Our current DH public key
	DHReceivingPublic DHPublicKey  // DHr: Their current DH public key

	// Message tracking
	PreviousChainLen uint32 // PN: Number of messages in previous sending chain

	// Out-of-order message handling
	SkippedMessageKeys map[MessageKeyID]MessageKey // Skipped message keys

	// Identity (for debugging)
	LocalAddress  Address // Our address
	RemoteAddress Address // Their address
}

// MessageKeyID uniquely identifies a message key for out-of-order delivery
type MessageKeyID struct {
	DHPublicKey DHPublicKey // The DH public key from the message header
	MessageNum  uint32      // The message number
}

// MessageHeader is sent with each encrypted message
type MessageHeader struct {
	DHPublicKey      DHPublicKey // Current DH public key
	PreviousChainLen uint32      // Number of messages in previous sending chain
	MessageNum       uint32      // Message number in current chain
}

// EncodeMessageHeader encodes a message header to bytes
func (h *MessageHeader) Encode() []byte {
	buf := make([]byte, DHKeyLen+4+4) // 32 + 4 + 4 = 40 bytes
	copy(buf[0:], h.DHPublicKey[:])
	binary.BigEndian.PutUint32(buf[32:], h.PreviousChainLen)
	binary.BigEndian.PutUint32(buf[36:], h.MessageNum)
	return buf
}

// DecodeMessageHeader decodes a message header from bytes
func (h *MessageHeader) Decode(buf []byte) error {
	if len(buf) < 40 {
		return fmt.Errorf("buffer too short for message header")
	}

	copy(h.DHPublicKey[:], buf[0:32])
	h.PreviousChainLen = binary.BigEndian.Uint32(buf[32:36])
	h.MessageNum = binary.BigEndian.Uint32(buf[36:40])
	return nil
}

// ===== KEY DERIVATION FUNCTIONS =====

// KDF_RK performs the root key KDF
// Derives a new root key and chain key from the current root key and DH output
// Returns: (new_root_key, new_chain_key)
func KDF_RK(rootKey RootKey, dhOutput []byte) (RootKey, ChainKey, error) {
	// Use HKDF with SHA-256
	// Input: rootKey as salt, dhOutput as input key material
	hkdf := hkdf.New(sha256.New, dhOutput, rootKey[:], []byte(KDFRootInfo))

	// Derive 64 bytes (32 for root key + 32 for chain key)
	output := make([]byte, 64)
	if _, err := hkdf.Read(output); err != nil {
		return RootKey{}, ChainKey{}, err
	}

	var newRootKey RootKey
	var newChainKey ChainKey
	copy(newRootKey[:], output[0:32])
	copy(newChainKey[:], output[32:64])

	return newRootKey, newChainKey, nil
}

// KDF_CK performs the chain key KDF
// Derives a new chain key and message key from the current chain key
// Returns: (new_chain_key, message_key)
func KDF_CK(chainKey ChainKey) (ChainKey, MessageKey) {
	// Use HMAC-SHA256 for chain key derivation
	// This is simpler and faster than HKDF for symmetric ratcheting

	// Derive message key: HMAC(chainKey, 0x01)
	msgKeyHMAC := sha256.New()
	msgKeyHMAC.Write(chainKey[:])
	msgKeyHMAC.Write([]byte{0x01})
	msgKeyHash := msgKeyHMAC.Sum(nil)

	var messageKey MessageKey
	copy(messageKey[:], msgKeyHash[:32])

	// Derive new chain key: HMAC(chainKey, 0x02)
	chainKeyHMAC := sha256.New()
	chainKeyHMAC.Write(chainKey[:])
	chainKeyHMAC.Write([]byte{0x02})
	chainKeyHash := chainKeyHMAC.Sum(nil)

	var newChainKey ChainKey
	copy(newChainKey[:], chainKeyHash[:32])

	return newChainKey, messageKey
}

// ===== RATCHET STATE MANAGEMENT =====

// NewRatchetState initializes a new ratchet state for the initiator (Alice)
// sharedSecret: The initial shared secret from X3DH or similar key agreement
// remoteDHPublic: Bob's initial DH public key
// localDHPrivate, localDHPublic: Alice's initial DH key pair
// Returns nil and error if initialization fails
func NewRatchetState(
	sharedSecret []byte,
	remoteDHPublic DHPublicKey,
	localDHPrivate DHPrivateKey,
	localDHPublic DHPublicKey,
	localAddr Address,
	remoteAddr Address,
) (*RatchetState, error) {
	state := &RatchetState{
		DHSendingPrivate:   localDHPrivate,
		DHSendingPublic:    localDHPublic,
		DHReceivingPublic:  remoteDHPublic,
		SkippedMessageKeys: make(map[MessageKeyID]MessageKey),
		LocalAddress:       localAddr,
		RemoteAddress:      remoteAddr,
	}

	// Initialize root key from shared secret
	copy(state.RootKey[:], sharedSecret[:32])

	// Perform initial DH to derive sending chain key
	dhOutput, err := DH(state.DHSendingPrivate, state.DHReceivingPublic)
	if err != nil {
		return nil, fmt.Errorf("initial DH failed: %w", err)
	}

	// Derive root key and sending chain key
	newRootKey, sendingChainKey, err := KDF_RK(state.RootKey, dhOutput)
	if err != nil {
		return nil, fmt.Errorf("initial KDF failed: %w", err)
	}

	state.RootKey = newRootKey
	state.SendingChainKey = sendingChainKey

	return state, nil
}

// NewRatchetStateReceiver initializes a new ratchet state for the receiver (Bob)
// sharedSecret: The initial shared secret from X3DH or similar key agreement
// localDHPrivate, localDHPublic: Bob's initial DH key pair
func NewRatchetStateReceiver(
	sharedSecret []byte,
	localDHPrivate DHPrivateKey,
	localDHPublic DHPublicKey,
	localAddr Address,
	remoteAddr Address,
) *RatchetState {
	state := &RatchetState{
		DHSendingPrivate:   localDHPrivate,
		DHSendingPublic:    localDHPublic,
		SkippedMessageKeys: make(map[MessageKeyID]MessageKey),
		LocalAddress:       localAddr,
		RemoteAddress:      remoteAddr,
	}

	// Initialize root key from shared secret
	copy(state.RootKey[:], sharedSecret[:32])

	return state
}

// ===== DIFFIE-HELLMAN OPERATIONS =====

// GenerateDHKeyPair generates a new X25519 key pair
func GenerateDHKeyPair() (DHPrivateKey, DHPublicKey, error) {
	var private DHPrivateKey
	var public DHPublicKey

	// Generate random private key
	if _, err := rand.Read(private[:]); err != nil {
		return private, public, err
	}

	// Compute public key
	curve25519.ScalarBaseMult((*[32]byte)(&public), (*[32]byte)(&private))

	return private, public, nil
}

// DH performs X25519 Diffie-Hellman
// Returns the shared secret
func DH(privateKey DHPrivateKey, publicKey DHPublicKey) ([]byte, error) {
	var sharedSecret [32]byte
	curve25519.ScalarMult(&sharedSecret, (*[32]byte)(&privateKey), (*[32]byte)(&publicKey))
	return sharedSecret[:], nil
}

// ===== RATCHET OPERATIONS =====

// DHRatchet performs a DH ratchet step
// This is called when we receive a message with a new DH public key
func (s *RatchetState) DHRatchet(remoteDHPublic DHPublicKey) error {
	// Save previous sending chain length
	s.PreviousChainLen = s.SendingMsgNum

	// Reset sending message number
	s.SendingMsgNum = 0
	s.ReceivingMsgNum = 0

	// Update remote DH public key
	s.DHReceivingPublic = remoteDHPublic

	// Perform DH with remote's new public key
	dhOutput, err := DH(s.DHSendingPrivate, s.DHReceivingPublic)
	if err != nil {
		return err
	}

	// Derive new root key and receiving chain key
	newRootKey, newReceivingChainKey, err := KDF_RK(s.RootKey, dhOutput)
	if err != nil {
		return err
	}

	s.RootKey = newRootKey
	s.ReceivingChainKey = newReceivingChainKey

	// Generate new DH key pair for sending
	newPrivate, newPublic, err := GenerateDHKeyPair()
	if err != nil {
		return err
	}

	s.DHSendingPrivate = newPrivate
	s.DHSendingPublic = newPublic

	// Perform DH with our new key pair and their public key
	dhOutput2, err := DH(s.DHSendingPrivate, s.DHReceivingPublic)
	if err != nil {
		return err
	}

	// Derive new root key and sending chain key
	newRootKey2, newSendingChainKey, err := KDF_RK(s.RootKey, dhOutput2)
	if err != nil {
		return err
	}

	s.RootKey = newRootKey2
	s.SendingChainKey = newSendingChainKey

	return nil
}

// RatchetEncrypt encrypts a plaintext message
// Returns: (header, ciphertext, error)
func (s *RatchetState) RatchetEncrypt(plaintext []byte, aesEncrypt func([]byte, []byte) ([]byte, error)) ([]byte, []byte, error) {
	// Derive message key from sending chain
	newChainKey, messageKey := KDF_CK(s.SendingChainKey)
	s.SendingChainKey = newChainKey

	// Create message header
	header := &MessageHeader{
		DHPublicKey:      s.DHSendingPublic,
		PreviousChainLen: s.PreviousChainLen,
		MessageNum:       s.SendingMsgNum,
	}

	// Increment sending message number
	s.SendingMsgNum++

	// Encrypt plaintext with message key using AES-256-GCM
	ciphertext, err := aesEncrypt(plaintext, messageKey[:])
	if err != nil {
		return nil, nil, err
	}

	return header.Encode(), ciphertext, nil
}

// RatchetDecrypt decrypts a ciphertext message
// Returns: (plaintext, error)
func (s *RatchetState) RatchetDecrypt(headerBytes []byte, ciphertext []byte, aesDecrypt func([]byte, []byte) ([]byte, error)) ([]byte, error) {
	// Decode header
	var header MessageHeader
	if err := header.Decode(headerBytes); err != nil {
		return nil, err
	}

	// Check if we need to perform a DH ratchet step
	// (if the DH public key in the header is different from our receiving key)
	if header.DHPublicKey != s.DHReceivingPublic {
		// Skip message keys from the current receiving chain
		if err := s.SkipMessageKeys(s.DHReceivingPublic, s.ReceivingMsgNum, header.PreviousChainLen); err != nil {
			return nil, err
		}

		// Perform DH ratchet
		if err := s.DHRatchet(header.DHPublicKey); err != nil {
			return nil, err
		}
	}

	// Skip message keys if needed (for out-of-order messages)
	if header.MessageNum > s.ReceivingMsgNum {
		if err := s.SkipMessageKeys(header.DHPublicKey, s.ReceivingMsgNum, header.MessageNum); err != nil {
			return nil, err
		}
	}

	// Try to use a skipped message key first
	keyID := MessageKeyID{
		DHPublicKey: header.DHPublicKey,
		MessageNum:  header.MessageNum,
	}

	if messageKey, ok := s.SkippedMessageKeys[keyID]; ok {
		// Use skipped key
		delete(s.SkippedMessageKeys, keyID)
		return aesDecrypt(ciphertext, messageKey[:])
	}

	// Derive message key from receiving chain
	newChainKey, messageKey := KDF_CK(s.ReceivingChainKey)
	s.ReceivingChainKey = newChainKey
	s.ReceivingMsgNum++

	// Decrypt ciphertext with message key
	return aesDecrypt(ciphertext, messageKey[:])
}

// SkipMessageKeys stores message keys for skipped messages
// This handles out-of-order message delivery
func (s *RatchetState) SkipMessageKeys(dhPublicKey DHPublicKey, fromMsgNum uint32, toMsgNum uint32) error {
	// Limit the number of skipped message keys to prevent DoS
	const MaxSkip = 1000

	if toMsgNum-fromMsgNum > MaxSkip {
		return fmt.Errorf("skipping too many message keys (%d)", toMsgNum-fromMsgNum)
	}

	// Derive and store message keys for all skipped messages
	chainKey := s.ReceivingChainKey
	for i := fromMsgNum; i < toMsgNum; i++ {
		newChainKey, messageKey := KDF_CK(chainKey)
		chainKey = newChainKey

		keyID := MessageKeyID{
			DHPublicKey: dhPublicKey,
			MessageNum:  i,
		}
		s.SkippedMessageKeys[keyID] = messageKey
	}

	// Update receiving chain key
	s.ReceivingChainKey = chainKey

	return nil
}
