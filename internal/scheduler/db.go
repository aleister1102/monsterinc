package scheduler

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// DB wraps the SQL database connection and provides methods for interacting with scan history.
type DB struct {
	db     *sql.DB
	logger zerolog.Logger
}

// ScanHistoryEntry represents a record in the scan_history table.
type ScanHistoryEntry struct {
	ID             int64
	ScanStartTime  time.Time
	ScanEndTime    sql.NullTime
	Status         string
	TargetSource   string
	ReportFilePath sql.NullString
	LogSummary     sql.NullString
}

const createTableQuery = `
CREATE TABLE IF NOT EXISTS scan_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scan_session_id TEXT UNIQUE,
	scan_start_time DATETIME NOT NULL,
	scan_end_time DATETIME,
	status TEXT NOT NULL,
	target_source TEXT NOT NULL,
	num_targets INTEGER,
	report_file_path TEXT,
	log_summary TEXT,
	new_urls INTEGER DEFAULT 0,
	old_urls INTEGER DEFAULT 0,
	existing_urls INTEGER DEFAULT 0
);`

// NewDB initializes a new DB connection and ensures the schema is set up.
func NewDB(dataSourceName string, logger zerolog.Logger) (*DB, error) {
	if err := ensureDBDirectory(dataSourceName); err != nil {
		return nil, err
	}

	dbInstance, err := openDatabase(dataSourceName)
	if err != nil {
		return nil, err
	}

	db := &DB{
		db:     dbInstance,
		logger: logger,
	}

	if err := db.InitSchema(); err != nil {
		if err := db.Close(); err != nil {
			logger.Error().Err(err).Msg("Failed to close database connection during initialization")
		}
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

func ensureDBDirectory(dataSourceName string) error {
	dbDir := filepath.Dir(dataSourceName)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create scheduler database directory %s: %w", dbDir, err)
	}
	return nil
}

func openDatabase(dataSourceName string) (*sql.DB, error) {
	dbInstance, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("sql.Open failed for %s: %w", dataSourceName, err)
	}
	return dbInstance, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// InitSchema creates the scan_history table if it doesn't already exist.
func (d *DB) InitSchema() error {
	_, err := d.db.Exec(createTableQuery)
	if err != nil {
		return err
	}
	return nil
}

// RecordScanStart inserts a new record into scan_history with status "STARTED"
// and returns the ID of the newly inserted row.
func (d *DB) RecordScanStart(scanSessionID string, targetSource string, numTargets int, startTime time.Time) (int64, error) {
	query := `INSERT INTO scan_history (scan_session_id, target_source, num_targets, scan_start_time, status) VALUES (?, ?, ?, ?, ?)`

	result, err := d.db.Exec(query, scanSessionID, targetSource, numTargets, startTime, ScanStatusStarted)
	if err != nil {
		return 0, fmt.Errorf("failed to insert scan start record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// UpdateScanCompletion updates an existing scan_history record with completion details.
func (d *DB) UpdateScanCompletion(dbScanID int64, endTime time.Time, status string, logSummary string, newURLs int, oldURLs int, existingURLs int, reportPath string) error {
	query := `UPDATE scan_history SET scan_end_time = ?, status = ?, log_summary = ?, new_urls = ?, old_urls = ?, existing_urls = ?, report_file_path = ? WHERE id = ?`

	_, err := d.db.Exec(
		query,
		endTime,
		status,
		createNullString(logSummary),
		newURLs,
		oldURLs,
		existingURLs,
		createNullString(reportPath),
		dbScanID,
	)

	if err != nil {
		return fmt.Errorf("failed to update scan completion for ID %d: %w", dbScanID, err)
	}

	return nil
}

func createNullString(value string) sql.NullString {
	return sql.NullString{
		String: value,
		Valid:  value != "",
	}
}

// GetLastScanTime retrieves the scan_start_time of the most recent scan attempt
func (d *DB) GetLastScanTime() (*time.Time, error) {
	query := `SELECT scan_start_time FROM scan_history WHERE status = ? ORDER BY scan_start_time DESC LIMIT 1`

	var scanStartTime time.Time
	err := d.db.QueryRow(query, ScanStatusCompleted).Scan(&scanStartTime)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to query last scan start time: %w", err)
	}

	return &scanStartTime, nil
}
