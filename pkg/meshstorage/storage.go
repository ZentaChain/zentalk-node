// Package meshstorage provides distributed storage for ZenTalk encrypted chat history
package meshstorage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// LocalStorage handles storing encrypted chunks locally using SQLite
type LocalStorage struct {
	db   *sql.DB
	path string
}

// Chunk represents a stored data chunk
type Chunk struct {
	UserAddr  string
	ChunkID   int
	Data      []byte
	StoredAt  time.Time
	Size      int
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(dataDir string) (*LocalStorage, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "chunks.db")

	// Check if this is a new database
	isNewDB := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		isNewDB = true
	}

	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if isNewDB {
		// New database - create initial schema
		fmt.Println("📊 Creating new database with current schema...")

		schema := `
			CREATE TABLE IF NOT EXISTS chunks (
				user_addr TEXT NOT NULL,
				chunk_id INTEGER NOT NULL,
				data BLOB NOT NULL,
				stored_at INTEGER NOT NULL,
				size INTEGER NOT NULL,
				PRIMARY KEY (user_addr, chunk_id)
			);
			CREATE INDEX IF NOT EXISTS idx_user_addr ON chunks(user_addr);
			CREATE INDEX IF NOT EXISTS idx_stored_at ON chunks(stored_at);
		`

		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create schema: %w", err)
		}

		// Run migrations to set version
		if err := RunMigrations(db, dataDir); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to initialize schema version: %w", err)
		}
	} else {
		// Existing database - check for pending migrations
		needsMigration, currentVersion, targetVersion, err := NeedsMigration(db)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to check migration status: %w", err)
		}

		if needsMigration {
			fmt.Printf("🔄 Database migration needed: v%d → v%d\n", currentVersion, targetVersion)
			if err := RunMigrations(db, dataDir); err != nil {
				db.Close()
				return nil, fmt.Errorf("failed to run migrations: %w", err)
			}
			// Schema is valid after successful migration
		} else {
			// Validate schema only if no migration was run
			if err := ValidateSchema(db); err != nil {
				db.Close()
				return nil, fmt.Errorf("schema validation failed: %w", err)
			}
		}
	}

	return &LocalStorage{
		db:   db,
		path: dbPath,
	}, nil
}

// StoreChunk stores an encrypted chunk for a user
func (s *LocalStorage) StoreChunk(userAddr string, chunkID int, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("cannot store empty chunk")
	}

	query := `INSERT OR REPLACE INTO chunks (user_addr, chunk_id, data, stored_at, size)
	          VALUES (?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query, userAddr, chunkID, data, time.Now().Unix(), len(data))
	if err != nil {
		return fmt.Errorf("failed to store chunk: %w", err)
	}

	return nil
}

// GetChunk retrieves an encrypted chunk for a user
func (s *LocalStorage) GetChunk(userAddr string, chunkID int) ([]byte, error) {
	query := `SELECT data FROM chunks WHERE user_addr = ? AND chunk_id = ?`

	var data []byte
	err := s.db.QueryRow(query, userAddr, chunkID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("chunk not found: user=%s chunk=%d", userAddr, chunkID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chunk: %w", err)
	}

	return data, nil
}

// ListChunks returns all chunk IDs for a user
func (s *LocalStorage) ListChunks(userAddr string) ([]int, error) {
	query := `SELECT chunk_id FROM chunks WHERE user_addr = ? ORDER BY chunk_id`

	rows, err := s.db.Query(query, userAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	var chunkIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan chunk ID: %w", err)
		}
		chunkIDs = append(chunkIDs, id)
	}

	return chunkIDs, rows.Err()
}

// GetAllChunks retrieves all chunks for a user
func (s *LocalStorage) GetAllChunks(userAddr string) (map[int][]byte, error) {
	query := `SELECT chunk_id, data FROM chunks WHERE user_addr = ? ORDER BY chunk_id`

	rows, err := s.db.Query(query, userAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	chunks := make(map[int][]byte)
	for rows.Next() {
		var chunkID int
		var data []byte
		if err := rows.Scan(&chunkID, &data); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunks[chunkID] = data
	}

	return chunks, rows.Err()
}

// DeleteChunk deletes a specific chunk
func (s *LocalStorage) DeleteChunk(userAddr string, chunkID int) error {
	query := `DELETE FROM chunks WHERE user_addr = ? AND chunk_id = ?`

	result, err := s.db.Exec(query, userAddr, chunkID)
	if err != nil {
		return fmt.Errorf("failed to delete chunk: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("chunk not found: user=%s chunk=%d", userAddr, chunkID)
	}

	return nil
}

// DeleteAllChunks deletes all chunks for a user
func (s *LocalStorage) DeleteAllChunks(userAddr string) error {
	query := `DELETE FROM chunks WHERE user_addr = ?`

	_, err := s.db.Exec(query, userAddr)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	return nil
}

// GetStorageSize returns the total size of all stored chunks
func (s *LocalStorage) GetStorageSize() (int64, error) {
	query := `SELECT COALESCE(SUM(size), 0) FROM chunks`

	var totalSize int64
	err := s.db.QueryRow(query).Scan(&totalSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get storage size: %w", err)
	}

	return totalSize, nil
}

// GetStorageSizeForUser returns the total size of chunks for a specific user
func (s *LocalStorage) GetStorageSizeForUser(userAddr string) (int64, error) {
	query := `SELECT COALESCE(SUM(size), 0) FROM chunks WHERE user_addr = ?`

	var totalSize int64
	err := s.db.QueryRow(query, userAddr).Scan(&totalSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get user storage size: %w", err)
	}

	return totalSize, nil
}

// GetChunkCount returns the total number of chunks stored
func (s *LocalStorage) GetChunkCount() (int, error) {
	query := `SELECT COUNT(*) FROM chunks`

	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get chunk count: %w", err)
	}

	return count, nil
}

// GetUserCount returns the number of unique users with stored chunks
func (s *LocalStorage) GetUserCount() (int, error) {
	query := `SELECT COUNT(DISTINCT user_addr) FROM chunks`

	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get user count: %w", err)
	}

	return count, nil
}

// GetStats returns storage statistics
type StorageStats struct {
	TotalChunks int
	TotalUsers  int
	TotalSize   int64
}

func (s *LocalStorage) GetStats() (*StorageStats, error) {
	chunks, err := s.GetChunkCount()
	if err != nil {
		return nil, err
	}

	users, err := s.GetUserCount()
	if err != nil {
		return nil, err
	}

	size, err := s.GetStorageSize()
	if err != nil {
		return nil, err
	}

	return &StorageStats{
		TotalChunks: chunks,
		TotalUsers:  users,
		TotalSize:   size,
	}, nil
}

// Close closes the database connection
func (s *LocalStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Path returns the database file path
func (s *LocalStorage) Path() string {
	return s.path
}

// Cleanup removes old chunks (optional garbage collection)
// maxAge is the maximum age in seconds before chunks are deleted
func (s *LocalStorage) Cleanup(maxAge int64) (int, error) {
	cutoff := time.Now().Unix() - maxAge

	query := `DELETE FROM chunks WHERE stored_at < ?`

	result, err := s.db.Exec(query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old chunks: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return int(rows), nil
}

// ListAllChunks returns all chunks from all users
func (s *LocalStorage) ListAllChunks() ([]Chunk, error) {
	query := `SELECT user_addr, chunk_id, data, stored_at, size FROM chunks ORDER BY stored_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all chunks: %w", err)
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var chunk Chunk
		var storedAt int64
		if err := rows.Scan(&chunk.UserAddr, &chunk.ChunkID, &chunk.Data, &storedAt, &chunk.Size); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunk.StoredAt = time.Unix(storedAt, 0)
		chunks = append(chunks, chunk)
	}

	return chunks, rows.Err()
}
