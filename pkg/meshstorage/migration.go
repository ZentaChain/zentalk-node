// Package meshstorage provides distributed storage for ZenTalk encrypted chat history
package meshstorage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Storage schema version constants
const (
	// CurrentSchemaVersion is the current database schema version
	CurrentSchemaVersion = 1

	// MinSchemaVersion is the minimum supported schema version
	MinSchemaVersion = 1
)

// SchemaVersion represents the database schema version metadata
type SchemaVersion struct {
	Version   int
	AppliedAt time.Time
	Comment   string
}

// MigrationFunc is a function that performs a schema migration
type MigrationFunc func(db *sql.DB) error

// Migration represents a single database migration
type Migration struct {
	Version     int
	Description string
	Up          MigrationFunc
	Down        MigrationFunc
}

// migrations is the list of all migrations in order
var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema with version tracking",
		Up:          migration1Up,
		Down:        migration1Down,
	},
	// Future migrations will be added here:
	// {
	//     Version:     2,
	//     Description: "Add compression support",
	//     Up:          migration2Up,
	//     Down:        migration2Down,
	// },
}

// GetSchemaVersion returns the current schema version from the database
func GetSchemaVersion(db *sql.DB) (int, error) {
	// Check if schema_version table exists
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version'`
	var tableName string
	err := db.QueryRow(query).Scan(&tableName)
	if err == sql.ErrNoRows {
		// No schema_version table = version 0 (needs initialization)
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to check schema_version table: %w", err)
	}

	// Get current version (order by ROWID to get the most recently inserted row)
	query = `SELECT version FROM schema_version ORDER BY ROWID DESC LIMIT 1`
	var version int
	err = db.QueryRow(query).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get schema version: %w", err)
	}

	return version, nil
}

// setSchemaVersion records a new schema version
func setSchemaVersion(db *sql.DB, version int, comment string) error {
	query := `INSERT INTO schema_version (version, applied_at, comment) VALUES (?, ?, ?)`
	_, err := db.Exec(query, version, time.Now().Unix(), comment)
	if err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}
	return nil
}

// NeedsMigration checks if the database needs migration
func NeedsMigration(db *sql.DB) (bool, int, int, error) {
	currentVersion, err := GetSchemaVersion(db)
	if err != nil {
		return false, 0, 0, err
	}

	targetVersion := CurrentSchemaVersion
	needsMigration := currentVersion < targetVersion

	return needsMigration, currentVersion, targetVersion, nil
}

// RunMigrations runs all pending migrations
func RunMigrations(db *sql.DB, dataDir string) error {
	currentVersion, err := GetSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	fmt.Printf("📊 Current schema version: %d\n", currentVersion)
	fmt.Printf("📊 Target schema version: %d\n", CurrentSchemaVersion)

	if currentVersion == CurrentSchemaVersion {
		fmt.Println("✅ Database is up to date")
		return nil
	}

	if currentVersion > CurrentSchemaVersion {
		return fmt.Errorf("database schema version (%d) is newer than supported version (%d) - please upgrade software",
			currentVersion, CurrentSchemaVersion)
	}

	// Create backup before migration
	backupPath, err := createBackup(dataDir)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	fmt.Printf("💾 Created backup: %s\n", backupPath)

	// Run migrations in order
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			// Already applied
			continue
		}

		if migration.Version > CurrentSchemaVersion {
			// Future migration, skip
			break
		}

		fmt.Printf("🔄 Running migration %d: %s\n", migration.Version, migration.Description)

		// Run migration (not in transaction for SQLite compatibility)
		if err := migration.Up(db); err != nil {
			return fmt.Errorf("migration %d failed: %w (backup available at %s)",
				migration.Version, err, backupPath)
		}

		// Record migration
		if err := setSchemaVersion(db, migration.Version, migration.Description); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		fmt.Printf("✅ Migration %d completed\n", migration.Version)
	}

	fmt.Printf("✅ All migrations completed successfully\n")
	fmt.Printf("💡 Backup kept at: %s (you can delete it after verifying everything works)\n", backupPath)

	return nil
}

// createBackup creates a backup of the database and data directory
func createBackup(dataDir string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	backupDir := fmt.Sprintf("%s.backup_%s", dataDir, timestamp)

	// Copy entire data directory
	if err := copyDir(dataDir, backupDir); err != nil {
		return "", fmt.Errorf("failed to copy data directory: %w", err)
	}

	return backupDir, nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Skip backup directories
			if filepath.Base(srcPath) == ".backup" ||
				filepath.Ext(srcPath) == ".backup" ||
				contains([]string{filepath.Base(srcPath)}, "backup") {
				continue
			}
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0644)
}

// RestoreFromBackup restores the database from a backup
func RestoreFromBackup(backupDir, targetDir string) error {
	fmt.Printf("🔄 Restoring from backup: %s\n", backupDir)

	// Check if backup exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup directory does not exist: %s", backupDir)
	}

	// Remove current data directory
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("failed to remove current data directory: %w", err)
	}

	// Restore from backup
	if err := copyDir(backupDir, targetDir); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	fmt.Printf("✅ Restored from backup successfully\n")
	return nil
}

// ValidateSchema checks if the database schema is valid
func ValidateSchema(db *sql.DB) error {
	version, err := GetSchemaVersion(db)
	if err != nil {
		return err
	}

	if version < MinSchemaVersion {
		return fmt.Errorf("schema version %d is too old (minimum: %d) - migration required", version, MinSchemaVersion)
	}

	if version > CurrentSchemaVersion {
		return fmt.Errorf("schema version %d is too new (current: %d) - software upgrade required", version, CurrentSchemaVersion)
	}

	// Check required tables exist
	requiredTables := []string{"chunks", "schema_version"}
	for _, table := range requiredTables {
		query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
		var tableName string
		err := db.QueryRow(query, table).Scan(&tableName)
		if err == sql.ErrNoRows {
			return fmt.Errorf("required table missing: %s", table)
		}
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
	}

	return nil
}

// ========================================
// Migration Definitions
// ========================================

// migration1Up creates the initial schema with version tracking
func migration1Up(db *sql.DB) error {
	// Create schema_version table
	schema := `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER NOT NULL,
			applied_at INTEGER NOT NULL,
			comment TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_schema_version ON schema_version(version);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	return nil
}

// migration1Down rolls back migration 1
func migration1Down(db *sql.DB) error {
	_, err := db.Exec(`DROP TABLE IF EXISTS schema_version`)
	return err
}

// Example future migration (commented out):
// func migration2Up(db *sql.DB) error {
//     // Add compression field to chunks table
//     _, err := db.Exec(`ALTER TABLE chunks ADD COLUMN compression TEXT DEFAULT 'none'`)
//     return err
// }
//
// func migration2Down(db *sql.DB) error {
//     // SQLite doesn't support DROP COLUMN, so we'd need to:
//     // 1. Create new table without compression column
//     // 2. Copy data
//     // 3. Drop old table
//     // 4. Rename new table
//     return fmt.Errorf("downgrade from v2 to v1 not supported")
// }
