package storage

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ZentaChain/zentalk-node/pkg/protocol"
)

// QueuedMessage represents a message waiting for delivery
type QueuedMessage struct {
	ID              int64
	RecipientAddr   string // Hex-encoded address
	MessageID       string // Unique message identifier
	EncryptedPayload []byte // Full encrypted onion-routed message
	Timestamp       int64  // When message was queued
	ExpiresAt       int64  // When message expires (TTL)
	Attempts        int    // Delivery attempt count
}

// RelayMessageQueue manages offline message storage for a relay
type RelayMessageQueue struct {
	db  *sql.DB
	ttl time.Duration // Message time-to-live
}

// NewRelayMessageQueue creates a new relay message queue
// ttl: Time-to-live for queued messages (default: 30 days)
func NewRelayMessageQueue(dbPath string, ttl time.Duration) (*RelayMessageQueue, error) {
	if ttl == 0 {
		ttl = 30 * 24 * time.Hour // 30 days default
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open queue database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL: %v", err)
	}

	queue := &RelayMessageQueue{
		db:  db,
		ttl: ttl,
	}

	if err := queue.initSchema(); err != nil {
		return nil, err
	}

	// Start background cleanup goroutine
	go queue.cleanupExpiredMessages()

	return queue, nil
}

// initSchema creates the database schema
func (q *RelayMessageQueue) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS queued_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		recipient_addr TEXT NOT NULL,
		message_id TEXT UNIQUE NOT NULL,
		encrypted_payload BLOB NOT NULL,
		timestamp INTEGER NOT NULL,
		expires_at INTEGER NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
	);

	-- Index for fast lookup by recipient
	CREATE INDEX IF NOT EXISTS idx_recipient ON queued_messages(recipient_addr);

	-- Index for expiration cleanup
	CREATE INDEX IF NOT EXISTS idx_expires ON queued_messages(expires_at);

	-- Index for deduplication
	CREATE INDEX IF NOT EXISTS idx_message_id ON queued_messages(message_id);
	`

	if _, err := q.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %v", err)
	}

	return nil
}

// QueueMessage adds a message to the queue for an offline recipient
func (q *RelayMessageQueue) QueueMessage(recipientAddr protocol.Address, messageID [16]byte, encryptedPayload []byte) error {
	recipientHex := hex.EncodeToString(recipientAddr[:])
	messageIDHex := hex.EncodeToString(messageID[:])
	now := time.Now().Unix()
	expiresAt := now + int64(q.ttl.Seconds())

	query := `
		INSERT INTO queued_messages (recipient_addr, message_id, encrypted_payload, timestamp, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := q.db.Exec(query, recipientHex, messageIDHex, encryptedPayload, now, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to queue message: %v", err)
	}

	log.Printf("📬 Queued message %s for offline user %x (expires in %v)", messageIDHex[:8], recipientAddr[:8], q.ttl)
	return nil
}

// GetQueuedMessages retrieves all queued messages for a recipient
func (q *RelayMessageQueue) GetQueuedMessages(recipientAddr protocol.Address) ([]*QueuedMessage, error) {
	recipientHex := hex.EncodeToString(recipientAddr[:])

	query := `
		SELECT id, recipient_addr, message_id, encrypted_payload, timestamp, expires_at, attempts
		FROM queued_messages
		WHERE recipient_addr = ? AND expires_at > ?
		ORDER BY timestamp ASC
	`

	now := time.Now().Unix()
	rows, err := q.db.Query(query, recipientHex, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get queued messages: %v", err)
	}
	defer rows.Close()

	var messages []*QueuedMessage
	for rows.Next() {
		msg := &QueuedMessage{}
		if err := rows.Scan(&msg.ID, &msg.RecipientAddr, &msg.MessageID, &msg.EncryptedPayload, &msg.Timestamp, &msg.ExpiresAt, &msg.Attempts); err != nil {
			return nil, fmt.Errorf("failed to scan message: %v", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// DeleteMessage removes a message from the queue (after successful delivery)
func (q *RelayMessageQueue) DeleteMessage(messageID string) error {
	query := `DELETE FROM queued_messages WHERE message_id = ?`
	_, err := q.db.Exec(query, messageID)
	if err != nil {
		return fmt.Errorf("failed to delete message: %v", err)
	}
	return nil
}

// DeleteMessagesForRecipient deletes all queued messages for a recipient
func (q *RelayMessageQueue) DeleteMessagesForRecipient(recipientAddr protocol.Address) error {
	recipientHex := hex.EncodeToString(recipientAddr[:])
	query := `DELETE FROM queued_messages WHERE recipient_addr = ?`

	result, err := q.db.Exec(query, recipientHex)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %v", err)
	}

	count, _ := result.RowsAffected()
	log.Printf("🗑️  Deleted %d queued messages for %x", count, recipientAddr[:8])
	return nil
}

// IncrementAttempts increments the delivery attempt counter
func (q *RelayMessageQueue) IncrementAttempts(messageID string) error {
	query := `UPDATE queued_messages SET attempts = attempts + 1 WHERE message_id = ?`
	_, err := q.db.Exec(query, messageID)
	return err
}

// GetQueuedMessageCount returns the number of queued messages for a recipient
func (q *RelayMessageQueue) GetQueuedMessageCount(recipientAddr protocol.Address) (int, error) {
	recipientHex := hex.EncodeToString(recipientAddr[:])
	now := time.Now().Unix()

	query := `SELECT COUNT(*) FROM queued_messages WHERE recipient_addr = ? AND expires_at > ?`

	var count int
	err := q.db.QueryRow(query, recipientHex, now).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get message count: %v", err)
	}

	return count, nil
}

// GetTotalQueueSize returns the total number of queued messages
func (q *RelayMessageQueue) GetTotalQueueSize() (int, error) {
	now := time.Now().Unix()
	query := `SELECT COUNT(*) FROM queued_messages WHERE expires_at > ?`

	var count int
	err := q.db.QueryRow(query, now).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get queue size: %v", err)
	}

	return count, nil
}

// cleanupExpiredMessages periodically removes expired messages
func (q *RelayMessageQueue) cleanupExpiredMessages() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()
		query := `DELETE FROM queued_messages WHERE expires_at <= ?`

		result, err := q.db.Exec(query, now)
		if err != nil {
			log.Printf("Failed to cleanup expired messages: %v", err)
			continue
		}

		count, _ := result.RowsAffected()
		if count > 0 {
			log.Printf("🧹 Cleaned up %d expired messages", count)
		}
	}
}

// GetOldestMessageTime returns the timestamp of the oldest message in queue
func (q *RelayMessageQueue) GetOldestMessageTime(recipientAddr protocol.Address) (int64, error) {
	recipientHex := hex.EncodeToString(recipientAddr[:])
	now := time.Now().Unix()

	query := `SELECT MIN(timestamp) FROM queued_messages WHERE recipient_addr = ? AND expires_at > ?`

	var oldest sql.NullInt64
	err := q.db.QueryRow(query, recipientHex, now).Scan(&oldest)
	if err != nil {
		return 0, fmt.Errorf("failed to get oldest message time: %v", err)
	}

	if !oldest.Valid {
		return 0, nil // No messages
	}

	return oldest.Int64, nil
}

// GetQueueStats returns statistics about the message queue
func (q *RelayMessageQueue) GetQueueStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total messages
	total, err := q.GetTotalQueueSize()
	if err != nil {
		return nil, err
	}
	stats["total_messages"] = total

	// Messages by recipient
	query := `
		SELECT recipient_addr, COUNT(*) as count
		FROM queued_messages
		WHERE expires_at > ?
		GROUP BY recipient_addr
	`

	now := time.Now().Unix()
	rows, err := q.db.Query(query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recipientCounts := make(map[string]int)
	for rows.Next() {
		var addr string
		var count int
		if err := rows.Scan(&addr, &count); err != nil {
			return nil, err
		}
		recipientCounts[addr] = count
	}
	stats["by_recipient"] = recipientCounts

	return stats, nil
}

// Close closes the database connection
func (q *RelayMessageQueue) Close() error {
	return q.db.Close()
}
