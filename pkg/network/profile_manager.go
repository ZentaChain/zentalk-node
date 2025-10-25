package network

import (
	"crypto/rsa"
	"log"
	"time"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/protocol"
)

// UpdateProfile updates user profile and broadcasts to contacts
// avatarChunkID: MeshStorage chunk ID of the encrypted avatar
// avatarKey: AES-256 key to decrypt the avatar (32 bytes)
func (c *Client) UpdateProfile(username, bio string, avatarChunkID uint64, avatarKey []byte) (*protocol.ProfileUpdate, error) {
	if !c.connected {
		return nil, ErrNotConnected
	}

	// Create profile update
	profile := &protocol.ProfileUpdate{
		Address:       c.Address,
		AvatarChunkID: avatarChunkID,
		Timestamp:     uint64(time.Now().UnixMilli()),
	}

	// Set username (max 32 bytes)
	copy(profile.Username[:], []byte(username))

	// Set bio (max 256 bytes)
	copy(profile.Bio[:], []byte(bio))

	// Set avatar encryption key (32 bytes for AES-256)
	if len(avatarKey) > 0 {
		copy(profile.AvatarKey[:], avatarKey)
	}

	// Export public key
	pubKeyPEM, err := crypto.ExportPublicKeyPEM(c.PublicKey)
	if err != nil {
		return nil, err
	}
	profile.PublicKey = pubKeyPEM

	// Sign the profile (all fields except signature)
	dataToSign := profile.EncodeForSigning()
	signature, err := crypto.SignData(dataToSign, c.PrivateKey)
	if err != nil {
		return nil, err
	}
	profile.Signature = signature

	log.Printf("Profile created for user: %s (avatar chunk: %d, encrypted: %v)", username, avatarChunkID, len(avatarKey) > 0)
	return profile, nil
}

// BroadcastProfile sends profile update to a specific user
func (c *Client) BroadcastProfile(profile *protocol.ProfileUpdate, toAddr protocol.Address, toPubKey *rsa.PublicKey, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Encode profile
	profilePayload := profile.Encode()

	log.Printf("Broadcasting profile to %x", toAddr)

	// Use hybrid encryption for large profiles
	// Generate AES key
	aesKey, err := crypto.GenerateAESKey()
	if err != nil {
		return err
	}

	// Encrypt profile with AES
	encryptedProfile, err := crypto.AESEncrypt(profilePayload, aesKey)
	if err != nil {
		return err
	}

	// Encrypt AES key with RSA
	encryptedKey, err := crypto.RSAEncrypt(aesKey, toPubKey)
	if err != nil {
		return err
	}

	// Combine: [key length (2 bytes)] + [encrypted AES key] + [encrypted profile]
	keyLen := uint16(len(encryptedKey))
	combined := make([]byte, 2+len(encryptedKey)+len(encryptedProfile))
	combined[0] = byte(keyLen >> 8)
	combined[1] = byte(keyLen)
	copy(combined[2:], encryptedKey)
	copy(combined[2+len(encryptedKey):], encryptedProfile)

	// Build onion layers
	onion, err := crypto.BuildOnionLayers(relayPath, toAddr, combined)
	if err != nil {
		return err
	}

	// Create relay forward message
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeRelayForward,
		Length:    uint32(len(onion)),
		Flags:     protocol.FlagEncrypted,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send to relay
	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		return err
	}

	if _, err := c.relayConn.Write(onion); err != nil {
		return err
	}

	log.Printf("✅ Profile sent to %x", toAddr)
	return nil
}

// RequestProfile requests a profile from another user
func (c *Client) RequestProfile(targetAddr protocol.Address, targetPubKey *rsa.PublicKey, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create a profile request message (simple text message for now)
	msg := &protocol.DirectMessage{
		From:        c.Address,
		To:          targetAddr,
		Timestamp:   uint64(time.Now().UnixMilli()),
		ContentType: protocol.ContentTypeText,
		Content:     []byte("PROFILE_REQUEST"),
	}

	msgPayload := msg.Encode()

	// Encrypt with target's public key
	encryptedMsg, err := crypto.RSAEncrypt(msgPayload, targetPubKey)
	if err != nil {
		return err
	}

	// Build onion layers
	onion, err := crypto.BuildOnionLayers(relayPath, targetAddr, encryptedMsg)
	if err != nil {
		return err
	}

	// Create relay forward message
	header := &protocol.Header{
		Magic:     protocol.ProtocolMagic,
		Version:   protocol.ProtocolVersion,
		Type:      protocol.MsgTypeRelayForward,
		Length:    uint32(len(onion)),
		Flags:     protocol.FlagEncrypted,
		MessageID: protocol.GenerateMessageID(),
	}

	// Send to relay
	if err := protocol.WriteHeader(c.relayConn, header); err != nil {
		return err
	}

	if _, err := c.relayConn.Write(onion); err != nil {
		return err
	}

	log.Printf("✅ Profile request sent to %x", targetAddr)
	return nil
}
