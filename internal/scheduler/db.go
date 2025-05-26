package scheduler

import (
	"database/sql"
	// "log"
	"time"

	"github.com/rs/zerolog" // Added
	_ "modernc.org/sqlite" // SQLite driver, CGO-free
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

// NewDB initializes a new DB wrapper with the given data source name (SQLite file path).
// It also pings the database to ensure connectivity.
func NewDB(dataSourceName string, logger zerolog.Logger) (*DB, error) { // Changed logger type
	d, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, err
	}
	if err = d.Ping(); err != nil {
		return nil, err
	}
	logger.Info().Str("database_path", dataSourceName).Msg("DB: Successfully connected to SQLite database") // Changed logger call
	return &DB{db: d, logger: logger}, nil
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
func (d *DB) RecordScanStart(scanSessionID string, targetSource string, numTargets int, startTime time.Time) (int64, error) { // Updated signature
	query := `INSERT INTO scan_history (scan_session_id, scan_start_time, status, target_source, num_targets) VALUES (?, ?, ?, ?, ?)`
	res, err := d.db.Exec(query, scanSessionID, startTime, "STARTED", targetSource, numTargets)
	if err != nil {
		d.logger.Error().Err(err).Str("scan_session_id", scanSessionID).Msg("DB: Failed to record scan start") // Changed logger call
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		d.logger.Error().Err(err).Str("scan_session_id", scanSessionID).Msg("DB: Failed to get last insert ID for scan start") // Changed logger call
		return 0, err
	}
	d.logger.Info().Int64("db_id", id).Str("scan_session_id", scanSessionID).Str("target_source", targetSource).Msg("DB: Recorded scan start") // Changed logger call
	return id, nil
}

// UpdateScanCompletion updates an existing scan_history record with completion details.
func (d *DB) UpdateScanCompletion(id int64, endTime time.Time, status string, logSummary string, newURLs int, oldURLs int, existingURLs int, reportPath string) error { // Updated signature
	query := `UPDATE scan_history SET scan_end_time = ?, status = ?, report_file_path = ?, log_summary = ?, new_urls = ?, old_urls = ?, existing_urls = ? WHERE id = ?`

	nullReportPath := sql.NullString{String: reportPath, Valid: reportPath != ""}
	nullLogSummary := sql.NullString{String: logSummary, Valid: logSummary != ""}
	nullEndTime := sql.NullTime{Time: endTime, Valid: !endTime.IsZero()}

	_, err := d.db.Exec(query, nullEndTime, status, nullReportPath, nullLogSummary, newURLs, oldURLs, existingURLs, id)
	if err != nil {
		d.logger.Error().Err(err).Int64("db_id", id).Msg("DB: Failed to update scan completion") // Changed logger call
		return err
	}
	d.logger.Info().Int64("db_id", id).Str("status", status).Msg("DB: Updated scan completion") // Changed logger call
	return nil
}

// GetLastScanTime retrieves the scan_end_time of the most recent scan attempt
// (either successfully completed or the last retry that failed).
// Returns nil if no scan history is found.
func (d *DB) GetLastScanTime() (*time.Time, error) {
	query := `SELECT scan_end_time FROM scan_history ORDER BY scan_start_time DESC LIMIT 1`
	var lastScanEndTime sql.NullTime
	err := d.db.QueryRow(query).Scan(&lastScanEndTime)

	if err != nil {
		if err == sql.ErrNoRows {
			d.logger.Info().Msg("DB: No previous scan history found.") // Changed logger call
			return nil, nil // No history, not an error for scheduler logic
		}
		d.logger.Error().Err(err).Msg("DB: Failed to get last scan time") // Changed logger call
		return nil, err
	}

	if lastScanEndTime.Valid {
		d.logger.Info().Time("last_scan_end_time", lastScanEndTime.Time).Msg("DB: Retrieved last scan end time") // Changed logger call
		return &lastScanEndTime.Time, nil
	}
	d.logger.Info().Msg("DB: Last scan end time was NULL (scan might be in progress or crashed). Consider scan_start_time for next logic if needed.") // Changed logger call
	// If scan_end_time is NULL, it means the last scan didn't complete properly.
	// The scheduler might want to use scan_start_time of that record for its next calculation,
	// or treat it as if no valid last completion time exists. For now, returning nil.
	return nil, nil
}
