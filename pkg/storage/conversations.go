package storage

// ===== CONVERSATION OPERATIONS =====

// updateConversation updates conversation metadata after new message
func (db *MessageDB) updateConversation(msg *StoredMessage) error {
	// Extract preview text
	preview := string(msg.Content)
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}

	query := `
		INSERT INTO conversations (
			id, contact_address, last_message_id, last_message,
			last_timestamp, unread_count
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_message_id = excluded.last_message_id,
			last_message = excluded.last_message,
			last_timestamp = excluded.last_timestamp,
			unread_count = CASE
				WHEN excluded.last_message_id != conversations.last_message_id
				AND ? = 0
				THEN conversations.unread_count + 1
				ELSE conversations.unread_count
			END
	`

	_, err := db.db.Exec(
		query,
		msg.ConversationID,
		getOtherParty(msg),
		msg.MessageID,
		preview,
		msg.Timestamp,
		0, // Initial unread count
		boolToInt(msg.IsOutgoing),
	)

	return err
}

// GetConversations retrieves all conversations
func (db *MessageDB) GetConversations() ([]*Conversation, error) {
	query := `
		SELECT id, contact_address, last_message_id, last_message,
		       last_timestamp, unread_count, is_muted, is_pinned
		FROM conversations
		ORDER BY is_pinned DESC, last_timestamp DESC
	`

	rows, err := db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*Conversation

	for rows.Next() {
		var conv Conversation
		var isMuted, isPinned int

		err := rows.Scan(
			&conv.ID,
			&conv.ContactAddress,
			&conv.LastMessageID,
			&conv.LastMessage,
			&conv.LastTimestamp,
			&conv.UnreadCount,
			&isMuted,
			&isPinned,
		)
		if err != nil {
			return nil, err
		}

		conv.IsMuted = intToBool(isMuted)
		conv.IsPinned = intToBool(isPinned)

		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// MarkConversationRead marks all messages in conversation as read
func (db *MessageDB) MarkConversationRead(conversationID string) error {
	query := `UPDATE conversations SET unread_count = 0 WHERE id = ?`
	_, err := db.db.Exec(query, conversationID)
	return err
}
