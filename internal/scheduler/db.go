package scheduler

import (
	"database/sql"
	// "log"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog" // Added
	_ "modernc.org/sqlite"  // SQLite driver, CGO-free
)

// DB wraps the SQL database connection and provides methods for interacting with scan history.
type DB struct {
	db     *sql.DB
	logger zerolog.Logger // Changed to zerolog.Logger
}

// ScanHistoryEntry represents a record in the scan_history table.
type ScanHistoryEntry struct {
	ID             int64
	ScanStartTime  time.Time
	ScanEndTime    sql.NullTime // Use sql.NullTime for nullable time fields
	Status         string
	TargetSource   string
	ReportFilePath sql.NullString // Use sql.NullString for nullable string fields
	LogSummary     sql.NullString // Use sql.NullString for nullable string fields
}

// NewDB initializes a new DB connection and ensures the schema is set up.
// The logger passed here should ideally be a logger instance already contextualized for the scheduler or DB operations.
func NewDB(dataSourceName string, logger zerolog.Logger) (*DB, error) {
	logger.Info().Str("db_path", dataSourceName).Msg("Initializing scheduler database connection")

	// Ensure the directory for the SQLite database exists
	dbDir := filepath.Dir(dataSourceName)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		logger.Error().Err(err).Str("directory", dbDir).Msg("Failed to create scheduler database directory")
		return nil, fmt.Errorf("failed to create scheduler database directory %s: %w", dbDir, err)
	}

	dbInstance, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		logger.Error().Err(err).Str("db_path", dataSourceName).Msg("Failed to open scheduler database")
		return nil, fmt.Errorf("sql.Open failed for %s: %w", dataSourceName, err)
	}

	db := &DB{
		db:     dbInstance,
		logger: logger, // Use the passed logger directly (already contextualized or base scheduler logger)
	}

	if err := db.InitSchema(); err != nil {
		db.Close() // Close the DB if schema initialization fails
		logger.Error().Err(err).Msg("Failed to initialize database schema")
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	logger.Info().Str("path", dataSourceName).Msg("Database initialized and schema verified.")
	return db, nil
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
	query := `
	CREATE TABLE IF NOT EXISTS scan_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		scan_session_id TEXT UNIQUE, -- Added for better tracking of a full scan session
		scan_start_time DATETIME NOT NULL,
		scan_end_time DATETIME,
		status TEXT NOT NULL,
		target_source TEXT NOT NULL,
		num_targets INTEGER,
		report_file_path TEXT,
		log_summary TEXT, -- General errors or notes
		new_urls INTEGER DEFAULT 0,
		old_urls INTEGER DEFAULT 0,
		existing_urls INTEGER DEFAULT 0
	);
	`
	_, err := d.db.Exec(query)
	if err != nil {
		d.logger.Error().Err(err).Msg("DB: Failed to initialize schema") // Changed logger call
		return err
	}
	d.logger.Info().Msg("DB: Schema initialized successfully (scan_history table ensured).") // Changed logger call
	return nil
}

// RecordScanStart inserts a new record into scan_history with status "STARTED"
// and returns the ID of the newly inserted row.
func (d *DB) RecordScanStart(scanSessionID string, targetSource string, numTargets int, startTime time.Time) (int64, error) {
	query := `INSERT INTO scan_history (scan_session_id, target_source, num_targets, scan_start_time, status) VALUES (?, ?, ?, ?, ?)`
	result, err := d.db.Exec(query, scanSessionID, targetSource, numTargets, startTime, "STARTED")
	if err != nil {
		d.logger.Error().Err(err).Str("query", query).Msg("Failed to record scan start")
		return 0, fmt.Errorf("failed to insert scan start record: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		d.logger.Error().Err(err).Msg("Failed to get last insert ID for scan start")
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}
	d.logger.Info().Int64("db_id", id).Str("scan_session_id", scanSessionID).Msg("Recorded scan start in DB")
	return id, nil
}

// UpdateScanCompletion updates an existing scan_history record with completion details.
func (d *DB) UpdateScanCompletion(dbScanID int64, endTime time.Time, status string, logSummary string, newURLs int, oldURLs int, existingURLs int, reportPath string) error {
	query := `UPDATE scan_history SET scan_end_time = ?, status = ?, log_summary = ?, new_urls = ?, old_urls = ?, existing_urls = ?, report_file_path = ? WHERE id = ?`
	_, err := d.db.Exec(query, endTime, status, sql.NullString{String: logSummary, Valid: logSummary != ""}, newURLs, oldURLs, existingURLs, sql.NullString{String: reportPath, Valid: reportPath != ""}, dbScanID)
	if err != nil {
		d.logger.Error().Err(err).Int64("db_id", dbScanID).Str("query", query).Msg("Failed to update scan completion")
		return fmt.Errorf("failed to update scan completion for ID %d: %w", dbScanID, err)
	}
	d.logger.Info().Int64("db_id", dbScanID).Str("status", status).Msg("Updated scan completion in DB")
	return nil
}

// GetLastScanTime retrieves the scan_end_time of the most recent scan attempt
// (either successfully completed or the last retry that failed).
// Returns nil if no scan history is found.
func (d *DB) GetLastScanTime() (*time.Time, error) {
	query := `SELECT scan_end_time FROM scan_history WHERE status = ? ORDER BY scan_end_time DESC LIMIT 1`
	var nullableTime sql.NullTime
	err := d.db.QueryRow(query, "COMPLETED").Scan(&nullableTime)
	if err != nil {
		if err == sql.ErrNoRows {
			d.logger.Info().Msg("No completed scan found in history.")
			return nil, err // Return sql.ErrNoRows as is, so caller can distinguish
		}
		d.logger.Error().Err(err).Str("query", query).Msg("Failed to query last scan time")
		return nil, fmt.Errorf("failed to query last scan time: %w", err)
	}

	if nullableTime.Valid {
		d.logger.Debug().Time("last_scan_time", nullableTime.Time).Msg("Found last scan time.")
		return &nullableTime.Time, nil
	}
	d.logger.Info().Msg("Last scan time was NULL in DB (likely an incomplete scan was the latest). Treat as no scan found.")
	return nil, sql.ErrNoRows // Treat NULL as no row for scheduling purposes
}
