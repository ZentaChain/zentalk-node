package network

import (
	"crypto/rsa"
	"log"
	"time"

	"github.com/ZentaChain/zentalk-node/pkg/crypto"
	"github.com/ZentaChain/zentalk-node/pkg/protocol"
)

// GroupMember represents a member of a group
type GroupMember struct {
	Address   protocol.Address
	PublicKey *rsa.PublicKey
}

// Group represents a chat group
type Group struct {
	ID      protocol.GroupID
	Name    string
	Members []*GroupMember
}

// SendGroupMessage sends a message to all group members through onion routing
func (c *Client) SendGroupMessage(group *Group, content string, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create group message
	groupMsg := &protocol.GroupMessage{
		From:        c.Address,
		GroupID:     group.ID,
		Timestamp:   uint64(time.Now().UnixMilli()),
		ContentType: protocol.ContentTypeText,
		Content:     []byte(content),
	}

	// Encode the group message once
	groupMsgPayload := groupMsg.Encode()

	log.Printf("Sending group message to %d members", len(group.Members))

	// Send to each member individually (client-side fan-out)
	for _, member := range group.Members {
		// Skip sending to yourself
		if member.Address == c.Address {
			continue
		}

		// Encrypt group message with each member's public key (E2E encryption)
		encryptedMsg, err := crypto.RSAEncrypt(groupMsgPayload, member.PublicKey)
		if err != nil {
			log.Printf("Failed to encrypt for member %x: %v", member.Address, err)
			continue
		}

		// Build onion layers for this member
		onion, err := crypto.BuildOnionLayers(relayPath, member.Address, encryptedMsg)
		if err != nil {
			log.Printf("Failed to build onion for member %x: %v", member.Address, err)
			continue
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
			log.Printf("Failed to send header for member %x: %v", member.Address, err)
			continue
		}

		if _, err := c.relayConn.Write(onion); err != nil {
			log.Printf("Failed to send payload for member %x: %v", member.Address, err)
			continue
		}

		log.Printf("✅ Group message sent to member %x", member.Address)
	}

	log.Printf("Group message broadcast complete to group %x", group.ID)
	return nil
}

// CreateGroup creates a new group and notifies all members
func (c *Client) CreateGroup(groupID protocol.GroupID, groupName string, members []*GroupMember, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create group create message
	memberAddrs := make([]protocol.Address, len(members))
	for i, m := range members {
		memberAddrs[i] = m.Address
	}

	createMsg := &protocol.GroupCreateMessage{
		GroupID:     groupID,
		GroupName:   groupName,
		CreatorAddr: c.Address,
		Timestamp:   uint64(time.Now().UnixMilli()),
		Members:     memberAddrs,
	}

	createPayload := createMsg.Encode()

	log.Printf("Creating group '%s' with %d members", groupName, len(members))

	// Send group create notification to each member
	for _, member := range members {
		// Skip sending to yourself
		if member.Address == c.Address {
			continue
		}

		// Encrypt notification with member's public key
		encryptedMsg, err := crypto.RSAEncrypt(createPayload, member.PublicKey)
		if err != nil {
			log.Printf("Failed to encrypt for member %x: %v", member.Address, err)
			continue
		}

		// Build onion layers
		onion, err := crypto.BuildOnionLayers(relayPath, member.Address, encryptedMsg)
		if err != nil {
			log.Printf("Failed to build onion for member %x: %v", member.Address, err)
			continue
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
			log.Printf("Failed to send header for member %x: %v", member.Address, err)
			continue
		}

		if _, err := c.relayConn.Write(onion); err != nil {
			log.Printf("Failed to send payload for member %x: %v", member.Address, err)
			continue
		}

		log.Printf("✅ Group create notification sent to member %x", member.Address)
	}

	log.Printf("Group '%s' created successfully", groupName)
	return nil
}

// LeaveGroup leaves a group and notifies all members
func (c *Client) LeaveGroup(groupID protocol.GroupID, members []*GroupMember, relayPath []*crypto.RelayInfo) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create group leave message
	leaveMsg := &protocol.GroupLeaveMessage{
		GroupID:    groupID,
		MemberAddr: c.Address,
		Timestamp:  uint64(time.Now().UnixMilli()),
	}

	// Sign the leave message
	dataToSign := leaveMsg.EncodeForSigning()
	signature, err := crypto.SignData(dataToSign, c.PrivateKey)
	if err != nil {
		return err
	}
	leaveMsg.Signature = signature

	leavePayload := leaveMsg.Encode()

	log.Printf("Leaving group %x", groupID)

	// Notify all members
	for _, member := range members {
		// Skip sending to yourself
		if member.Address == c.Address {
			continue
		}

		// Encrypt notification with member's public key
		encryptedMsg, err := crypto.RSAEncrypt(leavePayload, member.PublicKey)
		if err != nil {
			log.Printf("Failed to encrypt for member %x: %v", member.Address, err)
			continue
		}

		// Build onion layers
		onion, err := crypto.BuildOnionLayers(relayPath, member.Address, encryptedMsg)
		if err != nil {
			log.Printf("Failed to build onion for member %x: %v", member.Address, err)
			continue
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
			log.Printf("Failed to send header for member %x: %v", member.Address, err)
			continue
		}

		if _, err := c.relayConn.Write(onion); err != nil {
			log.Printf("Failed to send payload for member %x: %v", member.Address, err)
			continue
		}

		log.Printf("✅ Group leave notification sent to member %x", member.Address)
	}

	log.Printf("Left group %x successfully", groupID)
	return nil
}

// UpdateGroup updates group settings (name, members, admin)
// updateType: 1=name, 2=add member, 3=remove member, 4=admin change
func (c *Client) UpdateGroup(
	groupID protocol.GroupID,
	updateType uint8,
	members []*GroupMember,
	relayPath []*crypto.RelayInfo,
	newGroupName string,
	targetMember *GroupMember,
) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create group update message
	updateMsg := &protocol.GroupUpdateMessage{
		GroupID:      groupID,
		UpdateType:   updateType,
		UpdatedBy:    c.Address,
		Timestamp:    uint64(time.Now().UnixMilli()),
		NewGroupName: newGroupName,
	}

	// Set member address if adding/removing
	if targetMember != nil {
		updateMsg.MemberAddr = targetMember.Address
	}

	// Sign the update message
	dataToSign := updateMsg.EncodeForSigning()
	signature, err := crypto.SignData(dataToSign, c.PrivateKey)
	if err != nil {
		return err
	}
	updateMsg.Signature = signature

	updatePayload := updateMsg.Encode()

	// Log the update type
	switch updateType {
	case protocol.GroupUpdateName:
		log.Printf("Updating group name to '%s'", newGroupName)
	case protocol.GroupUpdateAddMember:
		log.Printf("Adding member %x to group", targetMember.Address)
	case protocol.GroupUpdateRemoveMember:
		log.Printf("Removing member %x from group", targetMember.Address)
	case protocol.GroupUpdateAdminChange:
		log.Printf("Changing admin to %x", targetMember.Address)
	}

	// Notify all members about the update
	for _, member := range members {
		// Skip sending to yourself
		if member.Address == c.Address {
			continue
		}

		// Encrypt notification with member's public key
		encryptedMsg, err := crypto.RSAEncrypt(updatePayload, member.PublicKey)
		if err != nil {
			log.Printf("Failed to encrypt for member %x: %v", member.Address, err)
			continue
		}

		// Build onion layers
		onion, err := crypto.BuildOnionLayers(relayPath, member.Address, encryptedMsg)
		if err != nil {
			log.Printf("Failed to build onion for member %x: %v", member.Address, err)
			continue
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
			log.Printf("Failed to send header for member %x: %v", member.Address, err)
			continue
		}

		if _, err := c.relayConn.Write(onion); err != nil {
			log.Printf("Failed to send payload for member %x: %v", member.Address, err)
			continue
		}

		log.Printf("✅ Group update notification sent to member %x", member.Address)
	}

	log.Printf("Group update completed successfully")
	return nil
}

// JoinGroup sends a request to join a group (for public/invite-only groups)
func (c *Client) JoinGroup(
	groupID protocol.GroupID,
	adminAddr protocol.Address,
	adminPubKey *rsa.PublicKey,
	relayPath []*crypto.RelayInfo,
) error {
	if !c.connected {
		return ErrNotConnected
	}

	// Create group join request
	joinMsg := &protocol.GroupJoinMessage{
		GroupID:    groupID,
		MemberAddr: c.Address,
		Timestamp:  uint64(time.Now().UnixMilli()),
	}

	// Sign the join request
	dataToSign := joinMsg.EncodeForSigning()
	signature, err := crypto.SignData(dataToSign, c.PrivateKey)
	if err != nil {
		return err
	}
	joinMsg.Signature = signature

	joinPayload := joinMsg.Encode()

	log.Printf("Requesting to join group %x", groupID)

	// Encrypt request with admin's public key
	encryptedMsg, err := crypto.RSAEncrypt(joinPayload, adminPubKey)
	if err != nil {
		return err
	}

	// Build onion layers to admin
	onion, err := crypto.BuildOnionLayers(relayPath, adminAddr, encryptedMsg)
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

	log.Printf("✅ Join request sent to group admin %x", adminAddr)
	return nil
}
