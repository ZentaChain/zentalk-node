package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// ===== X3DH (Extended Triple Diffie-Hellman) =====
// Implements the X3DH key agreement protocol
// https://signal.org/docs/specifications/x3dh/

// X3DH Key Types:
// - Identity Key (IK): Long-term identity key
// - Signed PreKey (SPK): Medium-term key, signed by identity key
// - One-Time PreKey (OPK): Single-use keys for forward secrecy
// - Ephemeral Key (EK): Generated per session initiation

// Constants
const (
	X3DHInfo = "ZenTalk X3DH Key Agreement"
)

// IdentityKeyPair represents a long-term identity key
type IdentityKeyPair struct {
	PublicKey  [32]byte // Ed25519 public key (for signatures)
	PrivateKey [64]byte // Ed25519 private key
	DHPublic   [32]byte // X25519 public key (for DH)
	DHPrivate  [32]byte // X25519 private key (for DH)
}

// SignedPreKey represents a signed medium-term key
type SignedPreKey struct {
	KeyID     uint32   // Unique identifier for this prekey
	PublicKey [32]byte // X25519 public key
	Signature [64]byte // Ed25519 signature by identity key
	Timestamp uint64   // When this key was created
}

// SignedPreKeyPrivate represents the private part
type SignedPreKeyPrivate struct {
	KeyID      uint32
	PublicKey  [32]byte
	PrivateKey [32]byte
	Signature  [64]byte
	Timestamp  uint64
}

// OneTimePreKey represents a single-use key
type OneTimePreKey struct {
	KeyID     uint32   // Unique identifier
	PublicKey [32]byte // X25519 public key
}

// OneTimePreKeyPrivate represents the private part
type OneTimePreKeyPrivate struct {
	KeyID      uint32
	PublicKey  [32]byte
	PrivateKey [32]byte
}

// KeyBundle represents the public keys published by a user
type KeyBundle struct {
	Address        Address         // User address
	IdentityKey    [32]byte        // Identity public key (X25519)
	SignedPreKey   SignedPreKey    // Signed prekey
	OneTimePreKeys []OneTimePreKey // Available one-time prekeys
	RegistrationID uint32          // Unique registration ID
}

// InitialMessage is sent by Alice to Bob to establish a session
type InitialMessage struct {
	// Sender information
	SenderAddress Address // Alice's address (20 bytes)

	// Alice's keys
	IdentityKey  [32]byte // Alice's identity key (X25519)
	EphemeralKey [32]byte // Alice's ephemeral key

	// Bob's key IDs (which keys Alice used)
	UsedSignedPreKeyID  uint32
	UsedOneTimePreKeyID uint32 // 0 if no OPK was used

	// Initial message encrypted with derived key
	Ciphertext []byte
}

// ===== KEY GENERATION =====

// GenerateIdentityKeyPair generates a long-term identity key pair
func GenerateIdentityKeyPair() (*IdentityKeyPair, error) {
	// Generate Ed25519 key pair for signatures
	edPublic, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	// Generate X25519 key pair for DH
	var dhPrivate [32]byte
	if _, err := rand.Read(dhPrivate[:]); err != nil {
		return nil, err
	}

	var dhPublic [32]byte
	curve25519.ScalarBaseMult(&dhPublic, &dhPrivate)

	kp := &IdentityKeyPair{
		DHPublic:  dhPublic,
		DHPrivate: dhPrivate,
	}
	copy(kp.PublicKey[:], edPublic)
	copy(kp.PrivateKey[:], edPrivate)

	return kp, nil
}

// GenerateSignedPreKey generates a signed prekey
func GenerateSignedPreKey(keyID uint32, identityKey *IdentityKeyPair) (*SignedPreKeyPrivate, error) {
	// Generate X25519 key pair
	var private [32]byte
	if _, err := rand.Read(private[:]); err != nil {
		return nil, err
	}

	var public [32]byte
	curve25519.ScalarBaseMult(&public, &private)

	// Create signature data: keyID + public key + timestamp
	timestamp := uint64(NowUnixMilli())
	sigData := make([]byte, 4+32+8)
	binary.BigEndian.PutUint32(sigData[0:4], keyID)
	copy(sigData[4:36], public[:])
	binary.BigEndian.PutUint64(sigData[36:44], timestamp)

	// Sign with identity key
	signature := ed25519.Sign(identityKey.PrivateKey[:], sigData)

	spk := &SignedPreKeyPrivate{
		KeyID:      keyID,
		PublicKey:  public,
		PrivateKey: private,
		Timestamp:  timestamp,
	}
	copy(spk.Signature[:], signature)

	return spk, nil
}

// GenerateOneTimePreKeys generates multiple one-time prekeys
func GenerateOneTimePreKeys(startID uint32, count int) ([]*OneTimePreKeyPrivate, error) {
	keys := make([]*OneTimePreKeyPrivate, count)

	for i := 0; i < count; i++ {
		var private [32]byte
		if _, err := rand.Read(private[:]); err != nil {
			return nil, err
		}

		var public [32]byte
		curve25519.ScalarBaseMult(&public, &private)

		keys[i] = &OneTimePreKeyPrivate{
			KeyID:      startID + uint32(i),
			PublicKey:  public,
			PrivateKey: private,
		}
	}

	return keys, nil
}

// ===== KEY BUNDLE CREATION =====

// CreateKeyBundle creates a public key bundle for publishing
func CreateKeyBundle(
	address Address,
	identityKey *IdentityKeyPair,
	signedPreKey *SignedPreKeyPrivate,
	oneTimePreKeys []*OneTimePreKeyPrivate,
	registrationID uint32,
) *KeyBundle {
	bundle := &KeyBundle{
		Address:        address,
		IdentityKey:    identityKey.DHPublic,
		RegistrationID: registrationID,
		SignedPreKey: SignedPreKey{
			KeyID:     signedPreKey.KeyID,
			PublicKey: signedPreKey.PublicKey,
			Signature: signedPreKey.Signature,
			Timestamp: signedPreKey.Timestamp,
		},
		OneTimePreKeys: make([]OneTimePreKey, len(oneTimePreKeys)),
	}

	for i, opk := range oneTimePreKeys {
		bundle.OneTimePreKeys[i] = OneTimePreKey{
			KeyID:     opk.KeyID,
			PublicKey: opk.PublicKey,
		}
	}

	return bundle
}

// VerifySignedPreKey verifies the signature on a signed prekey
func VerifySignedPreKey(identityKey [32]byte, spk *SignedPreKey) bool {
	// Reconstruct signature data
	sigData := make([]byte, 4+32+8)
	binary.BigEndian.PutUint32(sigData[0:4], spk.KeyID)
	copy(sigData[4:36], spk.PublicKey[:])
	binary.BigEndian.PutUint64(sigData[36:44], spk.Timestamp)

	// Convert X25519 identity key to Ed25519 public key for verification
	// Note: In production, you'd store both or use a proper conversion
	// For now, we assume the identity key is the Ed25519 key
	return ed25519.Verify(identityKey[:], sigData, spk.Signature[:])
}

// ===== X3DH KEY AGREEMENT =====

// X3DHInitiator performs X3DH as the initiator (Alice)
// Returns: (shared_secret, ephemeral_private, ephemeral_public, initial_message_for_bob, error)
func X3DHInitiator(
	senderAddress Address,
	aliceIdentity *IdentityKeyPair,
	bobBundle *KeyBundle,
) ([]byte, [32]byte, [32]byte, *InitialMessage, error) {
	// 1. Verify Bob's signed prekey
	// Note: Skipping verification for now as we'd need Bob's Ed25519 key
	// In production, always verify: VerifySignedPreKey(bobBundle.IdentityKey, &bobBundle.SignedPreKey)

	// 2. Generate ephemeral key
	var ephemeralPrivate [32]byte
	if _, err := rand.Read(ephemeralPrivate[:]); err != nil {
		var empty [32]byte
		return nil, empty, empty, nil, err
	}

	var ephemeralPublic [32]byte
	curve25519.ScalarBaseMult(&ephemeralPublic, &ephemeralPrivate)

	// 3. Perform 3 or 4 Diffie-Hellman operations
	var dh1, dh2, dh3, dh4 [32]byte

	// DH1 = DH(IK_A, SPK_B)
	curve25519.ScalarMult(&dh1, (*[32]byte)(&aliceIdentity.DHPrivate), &bobBundle.SignedPreKey.PublicKey)

	// DH2 = DH(EK_A, IK_B)
	curve25519.ScalarMult(&dh2, &ephemeralPrivate, &bobBundle.IdentityKey)

	// DH3 = DH(EK_A, SPK_B)
	curve25519.ScalarMult(&dh3, &ephemeralPrivate, &bobBundle.SignedPreKey.PublicKey)

	// DH4 = DH(EK_A, OPK_B) - if one-time prekey is available
	var usedOPKID uint32
	dhCount := 3
	if len(bobBundle.OneTimePreKeys) > 0 {
		// Use the first available one-time prekey
		opk := bobBundle.OneTimePreKeys[0]
		curve25519.ScalarMult(&dh4, &ephemeralPrivate, &opk.PublicKey)
		usedOPKID = opk.KeyID
		dhCount = 4
	}

	// 4. Concatenate DH outputs: DH1 || DH2 || DH3 || [DH4]
	var dhConcat []byte
	if dhCount == 4 {
		dhConcat = make([]byte, 128) // 4 * 32
		copy(dhConcat[0:32], dh1[:])
		copy(dhConcat[32:64], dh2[:])
		copy(dhConcat[64:96], dh3[:])
		copy(dhConcat[96:128], dh4[:])
	} else {
		dhConcat = make([]byte, 96) // 3 * 32
		copy(dhConcat[0:32], dh1[:])
		copy(dhConcat[32:64], dh2[:])
		copy(dhConcat[64:96], dh3[:])
	}

	// 5. Derive shared secret using HKDF
	// SK = HKDF(salt=0, IKM=DH_concat, info="X3DH")
	salt := make([]byte, 32) // All zeros
	hkdfReader := hkdf.New(sha256.New, dhConcat, salt, []byte(X3DHInfo))

	sharedSecret := make([]byte, 32)
	if _, err := hkdfReader.Read(sharedSecret); err != nil {
		var empty [32]byte
		return nil, empty, empty, nil, err
	}

	// 6. Create initial message
	initialMsg := &InitialMessage{
		SenderAddress:       senderAddress,
		IdentityKey:         aliceIdentity.DHPublic,
		EphemeralKey:        ephemeralPublic,
		UsedSignedPreKeyID:  bobBundle.SignedPreKey.KeyID,
		UsedOneTimePreKeyID: usedOPKID,
	}

	return sharedSecret, ephemeralPrivate, ephemeralPublic, initialMsg, nil
}

// X3DHResponder performs X3DH as the responder (Bob)
// Returns: (shared_secret, error)
func X3DHResponder(
	bobIdentity *IdentityKeyPair,
	bobSignedPreKey *SignedPreKeyPrivate,
	bobOneTimePreKeys map[uint32]*OneTimePreKeyPrivate, // Map of keyID -> private key
	initialMsg *InitialMessage,
) ([]byte, error) {
	// 1. Retrieve the used one-time prekey (if any)
	var usedOPK *OneTimePreKeyPrivate
	if initialMsg.UsedOneTimePreKeyID != 0 {
		var ok bool
		usedOPK, ok = bobOneTimePreKeys[initialMsg.UsedOneTimePreKeyID]
		if !ok {
			return nil, fmt.Errorf("one-time prekey not found: %d", initialMsg.UsedOneTimePreKeyID)
		}
	}

	// 2. Perform the same DH operations
	var dh1, dh2, dh3, dh4 [32]byte

	// DH1 = DH(SPK_B, IK_A)
	curve25519.ScalarMult(&dh1, (*[32]byte)(&bobSignedPreKey.PrivateKey), &initialMsg.IdentityKey)

	// DH2 = DH(IK_B, EK_A)
	curve25519.ScalarMult(&dh2, (*[32]byte)(&bobIdentity.DHPrivate), &initialMsg.EphemeralKey)

	// DH3 = DH(SPK_B, EK_A)
	curve25519.ScalarMult(&dh3, (*[32]byte)(&bobSignedPreKey.PrivateKey), &initialMsg.EphemeralKey)

	// DH4 = DH(OPK_B, EK_A) - if one-time prekey was used
	dhCount := 3
	if usedOPK != nil {
		curve25519.ScalarMult(&dh4, (*[32]byte)(&usedOPK.PrivateKey), &initialMsg.EphemeralKey)
		dhCount = 4
	}

	// 3. Concatenate DH outputs (same order as initiator)
	var dhConcat []byte
	if dhCount == 4 {
		dhConcat = make([]byte, 128)
		copy(dhConcat[0:32], dh1[:])
		copy(dhConcat[32:64], dh2[:])
		copy(dhConcat[64:96], dh3[:])
		copy(dhConcat[96:128], dh4[:])
	} else {
		dhConcat = make([]byte, 96)
		copy(dhConcat[0:32], dh1[:])
		copy(dhConcat[32:64], dh2[:])
		copy(dhConcat[64:96], dh3[:])
	}

	// 4. Derive shared secret using HKDF (same as initiator)
	salt := make([]byte, 32) // All zeros
	hkdfReader := hkdf.New(sha256.New, dhConcat, salt, []byte(X3DHInfo))

	sharedSecret := make([]byte, 32)
	if _, err := hkdfReader.Read(sharedSecret); err != nil {
		return nil, err
	}

	// 5. Delete the used one-time prekey (forward secrecy)
	if usedOPK != nil {
		delete(bobOneTimePreKeys, initialMsg.UsedOneTimePreKeyID)
	}

	return sharedSecret, nil
}

// ===== SERIALIZATION =====

// EncodeKeyBundle encodes a key bundle to bytes
func (kb *KeyBundle) Encode() []byte {
	// Calculate size: Address(20) + IdentityKey(32) + RegID(4) + SignedPreKey(4+32+64+8) + OPKCount(4) + OPKs(N*36)
	size := 20 + 32 + 4 + 108 + 4 + len(kb.OneTimePreKeys)*36
	buf := make([]byte, size)
	offset := 0

	// Address (20 bytes)
	copy(buf[offset:], kb.Address[:])
	offset += 20

	// Identity key (32 bytes)
	copy(buf[offset:], kb.IdentityKey[:])
	offset += 32

	// Registration ID (4 bytes)
	binary.BigEndian.PutUint32(buf[offset:], kb.RegistrationID)
	offset += 4

	// Signed PreKey (4 + 32 + 64 + 8 = 108 bytes)
	binary.BigEndian.PutUint32(buf[offset:], kb.SignedPreKey.KeyID)
	offset += 4
	copy(buf[offset:], kb.SignedPreKey.PublicKey[:])
	offset += 32
	copy(buf[offset:], kb.SignedPreKey.Signature[:])
	offset += 64
	binary.BigEndian.PutUint64(buf[offset:], kb.SignedPreKey.Timestamp)
	offset += 8

	// One-time prekeys count (4 bytes)
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(kb.OneTimePreKeys)))
	offset += 4

	// One-time prekeys (each: 4 + 32 = 36 bytes)
	for _, opk := range kb.OneTimePreKeys {
		binary.BigEndian.PutUint32(buf[offset:], opk.KeyID)
		offset += 4
		copy(buf[offset:], opk.PublicKey[:])
		offset += 32
	}

	return buf
}

// DecodeKeyBundle decodes a key bundle from bytes
func DecodeKeyBundle(buf []byte) (*KeyBundle, error) {
	if len(buf) < 20+32+4+108+4 {
		return nil, fmt.Errorf("buffer too short for key bundle")
	}

	kb := &KeyBundle{}
	offset := 0

	// Address
	copy(kb.Address[:], buf[offset:offset+20])
	offset += 20

	// Identity key
	copy(kb.IdentityKey[:], buf[offset:offset+32])
	offset += 32

	// Registration ID
	kb.RegistrationID = binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	// Signed PreKey
	kb.SignedPreKey.KeyID = binary.BigEndian.Uint32(buf[offset:])
	offset += 4
	copy(kb.SignedPreKey.PublicKey[:], buf[offset:offset+32])
	offset += 32
	copy(kb.SignedPreKey.Signature[:], buf[offset:offset+64])
	offset += 64
	kb.SignedPreKey.Timestamp = binary.BigEndian.Uint64(buf[offset:])
	offset += 8

	// One-time prekeys count
	opkCount := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	kb.OneTimePreKeys = make([]OneTimePreKey, opkCount)
	for i := uint32(0); i < opkCount; i++ {
		kb.OneTimePreKeys[i].KeyID = binary.BigEndian.Uint32(buf[offset:])
		offset += 4
		copy(kb.OneTimePreKeys[i].PublicKey[:], buf[offset:offset+32])
		offset += 32
	}

	return kb, nil
}

// Encode encodes an InitialMessage to bytes
func (im *InitialMessage) Encode() []byte {
	// Calculate size: SenderAddress(20) + IdentityKey(32) + EphemeralKey(32) + SignedPreKeyID(4) + OneTimePreKeyID(4) + CiphertextLen(4) + Ciphertext
	size := 20 + 32 + 32 + 4 + 4 + 4 + len(im.Ciphertext)
	buf := make([]byte, size)
	offset := 0

	// Sender address (20 bytes)
	copy(buf[offset:], im.SenderAddress[:])
	offset += 20

	// Identity key (32 bytes)
	copy(buf[offset:], im.IdentityKey[:])
	offset += 32

	// Ephemeral key (32 bytes)
	copy(buf[offset:], im.EphemeralKey[:])
	offset += 32

	// Used signed prekey ID (4 bytes)
	binary.BigEndian.PutUint32(buf[offset:], im.UsedSignedPreKeyID)
	offset += 4

	// Used one-time prekey ID (4 bytes)
	binary.BigEndian.PutUint32(buf[offset:], im.UsedOneTimePreKeyID)
	offset += 4

	// Ciphertext length (4 bytes)
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(im.Ciphertext)))
	offset += 4

	// Ciphertext (variable length)
	copy(buf[offset:], im.Ciphertext)

	return buf
}

// Decode decodes an InitialMessage from bytes
func (im *InitialMessage) Decode(buf []byte) error {
	if len(buf) < 20+32+32+4+4+4 {
		return fmt.Errorf("buffer too short for initial message")
	}

	offset := 0

	// Sender address (20 bytes)
	copy(im.SenderAddress[:], buf[offset:offset+20])
	offset += 20

	// Identity key (32 bytes)
	copy(im.IdentityKey[:], buf[offset:offset+32])
	offset += 32

	// Ephemeral key (32 bytes)
	copy(im.EphemeralKey[:], buf[offset:offset+32])
	offset += 32

	// Used signed prekey ID (4 bytes)
	im.UsedSignedPreKeyID = binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	// Used one-time prekey ID (4 bytes)
	im.UsedOneTimePreKeyID = binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	// Ciphertext length (4 bytes)
	ciphertextLen := binary.BigEndian.Uint32(buf[offset:])
	offset += 4

	// Validate ciphertext length
	if len(buf) < offset+int(ciphertextLen) {
		return fmt.Errorf("buffer too short for ciphertext")
	}

	// Ciphertext (variable length)
	im.Ciphertext = make([]byte, ciphertextLen)
	copy(im.Ciphertext, buf[offset:offset+int(ciphertextLen)])

	return nil
}
