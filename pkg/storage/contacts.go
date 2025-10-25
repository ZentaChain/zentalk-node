package storage

import (
	"database/sql"
	"fmt"

	"github.com/ZentaChain/zentalk-node/pkg/crypto"
)

// ===== CONTACT OPERATIONS =====

// SaveContact adds or updates a contact
func (db *MessageDB) SaveContact(contact *Contact) error {
	// Encrypt sensitive fields
	encryptedAvatarKey, err := crypto.AESEncrypt(contact.AvatarKey, db.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt avatar key: %v", err)
	}

	query := `
		INSERT INTO contacts (
			address, username, bio, avatar_chunk_id, avatar_key,
			public_key, added_at, last_seen, is_blocked, is_favorite
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(address) DO UPDATE SET
			username = excluded.username,
			bio = excluded.bio,
			avatar_chunk_id = excluded.avatar_chunk_id,
			avatar_key = excluded.avatar_key,
			public_key = excluded.public_key,
			last_seen = excluded.last_seen,
			is_blocked = excluded.is_blocked,
			is_favorite = excluded.is_favorite
	`

	_, err = db.db.Exec(
		query,
		contact.Address,
		contact.Username,
		contact.Bio,
		contact.AvatarChunkID,
		encryptedAvatarKey,
		contact.PublicKey,
		contact.AddedAt,
		contact.LastSeen,
		boolToInt(contact.IsBlocked),
		boolToInt(contact.IsFavorite),
	)

	return err
}

// GetContact retrieves a contact by address
func (db *MessageDB) GetContact(address string) (*Contact, error) {
	query := `
		SELECT address, username, bio, avatar_chunk_id, avatar_key,
		       public_key, added_at, last_seen, is_blocked, is_favorite
		FROM contacts WHERE address = ?
	`

	row := db.db.QueryRow(query, address)

	var contact Contact
	var encryptedAvatarKey []byte
	var isBlocked, isFavorite int

	err := row.Scan(
		&contact.Address,
		&contact.Username,
		&contact.Bio,
		&contact.AvatarChunkID,
		&encryptedAvatarKey,
		&contact.PublicKey,
		&contact.AddedAt,
		&contact.LastSeen,
		&isBlocked,
		&isFavorite,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	contact.IsBlocked = intToBool(isBlocked)
	contact.IsFavorite = intToBool(isFavorite)

	// Decrypt avatar key
	if len(encryptedAvatarKey) > 0 {
		contact.AvatarKey, err = crypto.AESDecrypt(encryptedAvatarKey, db.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt avatar key: %v", err)
		}
	}

	return &contact, nil
}

// GetAllContacts retrieves all contacts
func (db *MessageDB) GetAllContacts() ([]*Contact, error) {
	query := `
		SELECT address, username, bio, avatar_chunk_id, avatar_key,
		       public_key, added_at, last_seen, is_blocked, is_favorite
		FROM contacts
		ORDER BY username ASC
	`

	rows, err := db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []*Contact

	for rows.Next() {
		var contact Contact
		var encryptedAvatarKey []byte
		var isBlocked, isFavorite int

		err := rows.Scan(
			&contact.Address,
			&contact.Username,
			&contact.Bio,
			&contact.AvatarChunkID,
			&encryptedAvatarKey,
			&contact.PublicKey,
			&contact.AddedAt,
			&contact.LastSeen,
			&isBlocked,
			&isFavorite,
		)
		if err != nil {
			return nil, err
		}

		contact.IsBlocked = intToBool(isBlocked)
		contact.IsFavorite = intToBool(isFavorite)

		// Decrypt avatar key
		if len(encryptedAvatarKey) > 0 {
			contact.AvatarKey, err = crypto.AESDecrypt(encryptedAvatarKey, db.encryptionKey)
			if err != nil {
				continue // Skip contacts that can't be decrypted
			}
		}

		contacts = append(contacts, &contact)
	}

	return contacts, nil
}

// DeleteContact removes a contact
func (db *MessageDB) DeleteContact(address string) error {
	query := `DELETE FROM contacts WHERE address = ?`
	_, err := db.db.Exec(query, address)
	return err
}

// BlockContact blocks a contact
func (db *MessageDB) BlockContact(address string) error {
	query := `UPDATE contacts SET is_blocked = 1 WHERE address = ?`
	_, err := db.db.Exec(query, address)
	return err
}

// UnblockContact unblocks a contact
func (db *MessageDB) UnblockContact(address string) error {
	query := `UPDATE contacts SET is_blocked = 0 WHERE address = ?`
	_, err := db.db.Exec(query, address)
	return err
}
