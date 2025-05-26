package scheduler

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite" // SQLite driver, CGO-free
)

// DB wraps the SQL database connection and provides methods for interacting with scan history.
type DB struct {
	db     *sql.DB
	logger *log.Logger
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
func NewDB(dataSourceName string, logger *log.Logger) (*DB, error) {
	d, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, err
	}
	if err = d.Ping(); err != nil {
		return nil, err
	}
	logger.Printf("[INFO] DB: Successfully connected to SQLite database: %s", dataSourceName)
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
		scan_start_time DATETIME NOT NULL,
		scan_end_time DATETIME,
		status TEXT NOT NULL,
		target_source TEXT NOT NULL,
		report_file_path TEXT,
		log_summary TEXT
	);
	`
	_, err := d.db.Exec(query)
	if err != nil {
		d.logger.Printf("[ERROR] DB: Failed to initialize schema: %v", err)
		return err
	}
	d.logger.Println("[INFO] DB: Schema initialized successfully (scan_history table ensured).")
	return nil
}

// RecordScanStart inserts a new record into scan_history with status "STARTED"
// and returns the ID of the newly inserted row.
func (d *DB) RecordScanStart(startTime time.Time, targetSource string) (int64, error) {
	query := `INSERT INTO scan_history (scan_start_time, status, target_source) VALUES (?, ?, ?)`
	res, err := d.db.Exec(query, startTime, "STARTED", targetSource)
	if err != nil {
		d.logger.Printf("[ERROR] DB: Failed to record scan start: %v", err)
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		d.logger.Printf("[ERROR] DB: Failed to get last insert ID for scan start: %v", err)
		return 0, err
	}
	d.logger.Printf("[INFO] DB: Recorded scan start with ID: %d, Target: %s", id, targetSource)
	return id, nil
}

// UpdateScanCompletion updates an existing scan_history record with completion details.
func (d *DB) UpdateScanCompletion(id int64, endTime time.Time, status string, reportPath string, logSummary string) error {
	query := `UPDATE scan_history SET scan_end_time = ?, status = ?, report_file_path = ?, log_summary = ? WHERE id = ?`

	nullReportPath := sql.NullString{String: reportPath, Valid: reportPath != ""}
	nullLogSummary := sql.NullString{String: logSummary, Valid: logSummary != ""}
	nullEndTime := sql.NullTime{Time: endTime, Valid: !endTime.IsZero()}

	_, err := d.db.Exec(query, nullEndTime, status, nullReportPath, nullLogSummary, id)
	if err != nil {
		d.logger.Printf("[ERROR] DB: Failed to update scan completion for ID %d: %v", id, err)
		return err
	}
	d.logger.Printf("[INFO] DB: Updated scan completion for ID: %d, Status: %s", id, status)
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
			d.logger.Println("[INFO] DB: No previous scan history found.")
			return nil, nil // No history, not an error for scheduler logic
		}
		d.logger.Printf("[ERROR] DB: Failed to get last scan time: %v", err)
		return nil, err
	}

	if lastScanEndTime.Valid {
		d.logger.Printf("[INFO] DB: Retrieved last scan end time: %v", lastScanEndTime.Time)
		return &lastScanEndTime.Time, nil
	}
	d.logger.Println("[INFO] DB: Last scan end time was NULL (scan might be in progress or crashed). Consider scan_start_time for next logic if needed.")
	// If scan_end_time is NULL, it means the last scan didn't complete properly.
	// The scheduler might want to use scan_start_time of that record for its next calculation,
	// or treat it as if no valid last completion time exists. For now, returning nil.
	return nil, nil
}
