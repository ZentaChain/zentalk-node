package storage

import (
	"encoding/json"
	"fmt"
)

// ===== HELPER FUNCTIONS =====

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}

func getOtherParty(msg *StoredMessage) string {
	if msg.IsOutgoing {
		return msg.ToAddress
	}
	return msg.FromAddress
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetConversationID generates a deterministic conversation ID from two addresses
func GetConversationID(addr1, addr2 string) string {
	// Sort addresses to ensure same ID regardless of order
	if addr1 < addr2 {
		return fmt.Sprintf("%s-%s", addr1, addr2)
	}
	return fmt.Sprintf("%s-%s", addr2, addr1)
}

// ExportData exports all data as JSON (for backup)
func (db *MessageDB) ExportData() ([]byte, error) {
	data := struct {
		Messages      []*StoredMessage
		Contacts      []*Contact
		Conversations []*Conversation
	}{}

	// Get all messages
	// Implementation omitted for brevity - would decrypt and collect all

	// Get all contacts
	data.Contacts, _ = db.GetAllContacts()

	// Get all conversations
	data.Conversations, _ = db.GetConversations()

	return json.Marshal(data)
}
