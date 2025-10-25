package storage

import (
	"database/sql"
	"fmt"

	"github.com/zentalk/protocol/pkg/crypto"
	"github.com/zentalk/protocol/pkg/protocol"
)

// ===== MESSAGE OPERATIONS =====

// SaveMessage stores a message in the database
func (db *MessageDB) SaveMessage(msg *StoredMessage) error {
	// Encrypt content
	encryptedContent, err := crypto.AESEncrypt(msg.Content, db.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt content: %v", err)
	}

	// Encrypt MeshStorage encryption key if present
	var encryptedMeshKey []byte
	if len(msg.EncryptionKey) > 0 {
		encryptedMeshKey, err = crypto.AESEncrypt(msg.EncryptionKey, db.encryptionKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt mesh key: %v", err)
		}
	}

	query := `
		INSERT INTO messages (
			conversation_id, message_id, from_address, to_address,
			content, content_type, timestamp, status, is_outgoing,
			mesh_chunk_id, encryption_key, reply_to_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.db.Exec(
		query,
		msg.ConversationID,
		msg.MessageID,
		msg.FromAddress,
		msg.ToAddress,
		encryptedContent,
		msg.ContentType,
		msg.Timestamp,
		msg.Status,
		boolToInt(msg.IsOutgoing),
		msg.MeshChunkID,
		encryptedMeshKey,
		msg.ReplyToID,
	)

	if err != nil {
		return fmt.Errorf("failed to save message: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	msg.ID = id

	// Update conversation
	if err := db.updateConversation(msg); err != nil {
		return fmt.Errorf("failed to update conversation: %v", err)
	}

	return nil
}

// GetMessage retrieves a message by ID
func (db *MessageDB) GetMessage(messageID string) (*StoredMessage, error) {
	query := `
		SELECT id, conversation_id, message_id, from_address, to_address,
		       content, content_type, timestamp, status, is_outgoing,
		       mesh_chunk_id, encryption_key, reply_to_id
		FROM messages WHERE message_id = ?
	`

	row := db.db.QueryRow(query, messageID)

	var msg StoredMessage
	var encryptedContent []byte
	var encryptedMeshKey []byte
	var isOutgoing int

	err := row.Scan(
		&msg.ID,
		&msg.ConversationID,
		&msg.MessageID,
		&msg.FromAddress,
		&msg.ToAddress,
		&encryptedContent,
		&msg.ContentType,
		&msg.Timestamp,
		&msg.Status,
		&isOutgoing,
		&msg.MeshChunkID,
		&encryptedMeshKey,
		&msg.ReplyToID,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	msg.IsOutgoing = intToBool(isOutgoing)

	// Decrypt content
	msg.Content, err = crypto.AESDecrypt(encryptedContent, db.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt content: %v", err)
	}

	// Decrypt MeshStorage encryption key if present
	if len(encryptedMeshKey) > 0 {
		msg.EncryptionKey, err = crypto.AESDecrypt(encryptedMeshKey, db.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt mesh key: %v", err)
		}
	}

	return &msg, nil
}

// GetConversationMessages retrieves messages for a conversation
func (db *MessageDB) GetConversationMessages(conversationID string, limit, offset int) ([]*StoredMessage, error) {
	query := `
		SELECT id, conversation_id, message_id, from_address, to_address,
		       content, content_type, timestamp, status, is_outgoing,
		       mesh_chunk_id, encryption_key, reply_to_id
		FROM messages
		WHERE conversation_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := db.db.Query(query, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*StoredMessage

	for rows.Next() {
		var msg StoredMessage
		var encryptedContent []byte
		var encryptedMeshKey []byte
		var isOutgoing int

		err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.MessageID,
			&msg.FromAddress,
			&msg.ToAddress,
			&encryptedContent,
			&msg.ContentType,
			&msg.Timestamp,
			&msg.Status,
			&isOutgoing,
			&msg.MeshChunkID,
			&encryptedMeshKey,
			&msg.ReplyToID,
		)
		if err != nil {
			return nil, err
		}

		msg.IsOutgoing = intToBool(isOutgoing)

		// Decrypt content
		msg.Content, err = crypto.AESDecrypt(encryptedContent, db.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt content: %v", err)
		}

		// Decrypt MeshStorage encryption key if present
		if len(encryptedMeshKey) > 0 {
			msg.EncryptionKey, err = crypto.AESDecrypt(encryptedMeshKey, db.encryptionKey)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt mesh key: %v", err)
			}
		}

		messages = append(messages, &msg)
	}

	return messages, nil
}

// UpdateMessageStatus updates the delivery status of a message
func (db *MessageDB) UpdateMessageStatus(messageID string, status MessageStatus) error {
	query := `UPDATE messages SET status = ? WHERE message_id = ?`
	_, err := db.db.Exec(query, status, messageID)
	return err
}

// DeleteMessage deletes a message
func (db *MessageDB) DeleteMessage(messageID string) error {
	query := `DELETE FROM messages WHERE message_id = ?`
	_, err := db.db.Exec(query, messageID)
	return err
}

// SearchMessages searches for messages containing text
func (db *MessageDB) SearchMessages(searchText string, limit int) ([]*StoredMessage, error) {
	// Note: This requires decrypting all messages - not efficient for large DBs
	// In production, consider full-text search with encrypted indexes
	query := `
		SELECT id, conversation_id, message_id, from_address, to_address,
		       content, content_type, timestamp, status, is_outgoing,
		       mesh_chunk_id, encryption_key, reply_to_id
		FROM messages
		WHERE content_type = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := db.db.Query(query, protocol.ContentTypeText, limit*10) // Get more to filter
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*StoredMessage

	for rows.Next() {
		var msg StoredMessage
		var encryptedContent []byte
		var encryptedMeshKey []byte
		var isOutgoing int

		err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.MessageID,
			&msg.FromAddress,
			&msg.ToAddress,
			&encryptedContent,
			&msg.ContentType,
			&msg.Timestamp,
			&msg.Status,
			&isOutgoing,
			&msg.MeshChunkID,
			&encryptedMeshKey,
			&msg.ReplyToID,
		)
		if err != nil {
			return nil, err
		}

		msg.IsOutgoing = intToBool(isOutgoing)

		// Decrypt and search
		msg.Content, err = crypto.AESDecrypt(encryptedContent, db.encryptionKey)
		if err != nil {
			continue // Skip messages that can't be decrypted
		}

		// Check if content contains search text
		if contains(string(msg.Content), searchText) {
			messages = append(messages, &msg)
			if len(messages) >= limit {
				break
			}
		}
	}

	return messages, nil
}
