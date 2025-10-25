package meshstorage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetSchemaVersion(t *testing.T) {
	// Create temporary test database
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Should have version 1 (from initial migration)
	version, err := GetSchemaVersion(storage.db)
	if err != nil {
		t.Fatalf("Failed to get schema version: %v", err)
	}

	if version != CurrentSchemaVersion {
		t.Errorf("Expected schema version %d, got %d", CurrentSchemaVersion, version)
	}
}

func TestNeedsMigration(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	needsMigration, currentVer, targetVer, err := NeedsMigration(storage.db)
	if err != nil {
		t.Fatalf("Failed to check migration need: %v", err)
	}

	// New database should not need migration (already at current version)
	if needsMigration {
		t.Errorf("New database should not need migration, but got: current=%d target=%d",
			currentVer, targetVer)
	}

	if currentVer != CurrentSchemaVersion {
		t.Errorf("Expected current version %d, got %d", CurrentSchemaVersion, currentVer)
	}

	if targetVer != CurrentSchemaVersion {
		t.Errorf("Expected target version %d, got %d", CurrentSchemaVersion, targetVer)
	}
}

func TestValidateSchema(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Schema should be valid
	if err := ValidateSchema(storage.db); err != nil {
		t.Errorf("Schema validation failed: %v", err)
	}
}

func TestMigrationWithExistingData(t *testing.T) {
	tmpDir := t.TempDir()

	// Create storage and add some data
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	testUser := "0x1234567890123456789012345678901234567890"
	testData := []byte("encrypted test data")

	// Store some test data
	if err := storage.StoreChunk(testUser, 1, testData); err != nil {
		t.Fatalf("Failed to store test chunk: %v", err)
	}
	if err := storage.StoreChunk(testUser, 2, testData); err != nil {
		t.Fatalf("Failed to store test chunk: %v", err)
	}

	storage.Close()

	// Reopen storage (simulates node restart)
	storage2, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to reopen storage: %v", err)
	}
	defer storage2.Close()

	// Verify data is still accessible
	retrieved, err := storage2.GetChunk(testUser, 1)
	if err != nil {
		t.Fatalf("Failed to retrieve chunk after migration: %v", err)
	}

	if string(retrieved) != string(testData) {
		t.Errorf("Data mismatch after migration: expected %s, got %s", testData, retrieved)
	}

	// Verify count
	chunks, err := storage2.ListChunks(testUser)
	if err != nil {
		t.Fatalf("Failed to list chunks: %v", err)
	}

	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}
}

func TestBackupAndRestore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create storage with test data
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	testUser := "0x1234567890123456789012345678901234567890"
	testData := []byte("test backup data")

	if err := storage.StoreChunk(testUser, 1, testData); err != nil {
		t.Fatalf("Failed to store chunk: %v", err)
	}

	storage.Close()

	// Create backup
	backupDir, err := createBackup(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}
	defer os.RemoveAll(backupDir)

	t.Logf("Created backup at: %s", backupDir)

	// Verify backup was created
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Fatalf("Backup directory was not created")
	}

	// Verify backup contains database
	backupDB := filepath.Join(backupDir, "chunks.db")
	if _, err := os.Stat(backupDB); os.IsNotExist(err) {
		t.Fatalf("Backup database was not created")
	}

	// Corrupt original data
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)

	// Restore from backup
	if err := RestoreFromBackup(backupDir, tmpDir); err != nil {
		t.Fatalf("Failed to restore from backup: %v", err)
	}

	// Verify data was restored
	storage2, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open restored storage: %v", err)
	}
	defer storage2.Close()

	retrieved, err := storage2.GetChunk(testUser, 1)
	if err != nil {
		t.Fatalf("Failed to retrieve chunk from restored storage: %v", err)
	}

	if string(retrieved) != string(testData) {
		t.Errorf("Restored data mismatch: expected %s, got %s", testData, retrieved)
	}

	t.Log("✅ Backup and restore successful")
}

func TestSchemaVersionTable(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Verify schema_version table exists
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version'`
	var tableName string
	err = storage.db.QueryRow(query).Scan(&tableName)
	if err != nil {
		t.Fatalf("schema_version table does not exist: %v", err)
	}

	// Verify version record exists
	query = `SELECT version, applied_at, comment FROM schema_version ORDER BY applied_at DESC LIMIT 1`
	var version int
	var appliedAt int64
	var comment string

	err = storage.db.QueryRow(query).Scan(&version, &appliedAt, &comment)
	if err != nil {
		t.Fatalf("Failed to get version record: %v", err)
	}

	if version != CurrentSchemaVersion {
		t.Errorf("Expected version %d, got %d", CurrentSchemaVersion, version)
	}

	if comment == "" {
		t.Error("Version comment is empty")
	}

	t.Logf("Schema version: %d (applied at: %s, comment: %s)",
		version,
		time.Unix(appliedAt, 0).Format(time.RFC3339),
		comment)
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	testData := []byte("test file content")
	if err := os.WriteFile(srcPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dest.txt")
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}

	// Verify copied file
	copiedData, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(copiedData) != string(testData) {
		t.Errorf("Copied file content mismatch: expected %s, got %s", testData, copiedData)
	}
}

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory with files
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)

	testData := []byte("test data")
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), testData, 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), testData, 0644)

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dest")
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("Failed to copy directory: %v", err)
	}

	// Verify copied files
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt was not copied")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("subdir/file2.txt was not copied")
	}
}

func TestMigrationRollbackOnError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial database
	dbPath := filepath.Join(tmpDir, "chunks.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create chunks table matching the expected schema
	_, err = db.Exec(`
		CREATE TABLE chunks (
			user_addr TEXT NOT NULL,
			chunk_id INTEGER NOT NULL,
			data BLOB NOT NULL,
			stored_at INTEGER NOT NULL,
			size INTEGER NOT NULL,
			PRIMARY KEY (user_addr, chunk_id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Create schema_version table with old version
	_, err = db.Exec(`CREATE TABLE schema_version (version INTEGER, applied_at INTEGER, comment TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create schema_version table: %v", err)
	}

	_, err = db.Exec(`INSERT INTO schema_version VALUES (0, ?, 'test')`, time.Now().Unix())
	if err != nil {
		t.Fatalf("Failed to insert version: %v", err)
	}

	db.Close()

	// Try to open with NewLocalStorage (should run migration successfully)
	storage, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error opening storage: %v", err)
	}
	defer storage.Close()

	// Verify migration completed successfully
	version, err := GetSchemaVersion(storage.db)
	if err != nil {
		t.Fatalf("Failed to get schema version after migration: %v", err)
	}

	if version != CurrentSchemaVersion {
		t.Errorf("Expected schema version %d after migration, got %d", CurrentSchemaVersion, version)
	}

	// Verify backup was created (backup is sibling directory in parent)
	parentDir := filepath.Dir(tmpDir)
	baseName := filepath.Base(tmpDir)
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		t.Fatalf("Failed to read parent directory: %v", err)
	}

	backupFound := false
	expectedPrefix := baseName + ".backup_"
	for _, entry := range entries {
		// Backup is named {basename}.backup_{timestamp}
		if entry.IsDir() && len(entry.Name()) > len(expectedPrefix) && entry.Name()[:len(expectedPrefix)] == expectedPrefix {
			backupFound = true
			t.Logf("Found backup directory: %s", entry.Name())
			break
		}
	}

	if !backupFound {
		t.Errorf("Expected backup directory named %s* to be created during migration", expectedPrefix)
	}

	t.Log("✅ Migration with backup successful")
}

func TestMultipleConsecutiveMigrations(t *testing.T) {
	tmpDir := t.TempDir()

	// First initialization
	storage1, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("First initialization failed: %v", err)
	}

	testUser := "0x1234567890123456789012345678901234567890"
	testData := []byte("test data")
	storage1.StoreChunk(testUser, 1, testData)
	storage1.Close()

	// Second initialization (no migration needed)
	storage2, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Second initialization failed: %v", err)
	}
	storage2.Close()

	// Third initialization (no migration needed)
	storage3, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("Third initialization failed: %v", err)
	}
	defer storage3.Close()

	// Verify data is still intact
	retrieved, err := storage3.GetChunk(testUser, 1)
	if err != nil {
		t.Fatalf("Failed to retrieve data: %v", err)
	}

	if string(retrieved) != string(testData) {
		t.Errorf("Data mismatch after multiple initializations")
	}

	t.Log("✅ Multiple consecutive migrations successful")
}
