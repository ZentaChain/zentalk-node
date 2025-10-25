package storage

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrDatabaseLocked     = errors.New("database locked")
	ErrConversationExists = errors.New("conversation already exists")
)

// MessageStatus represents message delivery status
type MessageStatus string

const (
	MessageStatusSending   MessageStatus = "sending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

// MessageDB manages encrypted local message storage
type MessageDB struct {
	db            *sql.DB
	encryptionKey []byte // Derived from user password
}

// StoredMessage represents a message in the database
type StoredMessage struct {
	ID             int64
	ConversationID string
	MessageID      string
	FromAddress    string
	ToAddress      string
	Content        []byte
	ContentType    uint8
	Timestamp      int64
	Status         MessageStatus
	IsOutgoing     bool
	MeshChunkID    uint64
	EncryptionKey  []byte
	ReplyToID      string
}

// Contact represents a contact in the database
type Contact struct {
	Address       string
	Username      string
	Bio           string
	AvatarChunkID uint64
	AvatarKey     []byte
	PublicKey     []byte
	AddedAt       int64
	LastSeen      int64
	IsBlocked     bool
	IsFavorite    bool
}

// Conversation represents a conversation thread
type Conversation struct {
	ID             string
	ContactAddress string
	LastMessageID  string
	LastMessage    string
	LastTimestamp  int64
	UnreadCount    int
	IsMuted        bool
	IsPinned       bool
}

// NewMessageDB creates a new encrypted message database
func NewMessageDB(dbPath string, password string) (*MessageDB, error) {
	// Derive encryption key from password using PBKDF2
	encryptionKey := deriveKey(password)

	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %v", err)
	}

	mdb := &MessageDB{
		db:            db,
		encryptionKey: encryptionKey,
	}

	// Initialize schema
	if err := mdb.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return mdb, nil
}

// deriveKey derives an encryption key from password using SHA-256
// In production, use PBKDF2 with salt
func deriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// initSchema creates database tables
func (db *MessageDB) initSchema() error {
	schema := `
	-- Messages table
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL,
		message_id TEXT UNIQUE NOT NULL,
		from_address TEXT NOT NULL,
		to_address TEXT NOT NULL,
		content BLOB NOT NULL,
		content_type INTEGER NOT NULL,
		timestamp INTEGER NOT NULL,
		status TEXT NOT NULL,
		is_outgoing INTEGER NOT NULL,
		mesh_chunk_id INTEGER DEFAULT 0,
		encryption_key BLOB,
		reply_to_id TEXT,
		created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
	);

	-- Contacts table
	CREATE TABLE IF NOT EXISTS contacts (
		address TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		bio TEXT,
		avatar_chunk_id INTEGER DEFAULT 0,
		avatar_key BLOB,
		public_key BLOB,
		added_at INTEGER NOT NULL,
		last_seen INTEGER,
		is_blocked INTEGER NOT NULL DEFAULT 0,
		is_favorite INTEGER NOT NULL DEFAULT 0
	);

	-- Conversations table
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		contact_address TEXT NOT NULL,
		last_message_id TEXT,
		last_message TEXT,
		last_timestamp INTEGER,
		unread_count INTEGER NOT NULL DEFAULT 0,
		is_muted INTEGER NOT NULL DEFAULT 0,
		is_pinned INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (contact_address) REFERENCES contacts(address)
	);

	-- Indexes for performance
	CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_conversations_last_timestamp ON conversations(last_timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_contacts_username ON contacts(username);
	`

	_, err := db.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %v", err)
	}

	return nil
}

// Close closes the database connection
func (db *MessageDB) Close() error {
	return db.db.Close()
}
