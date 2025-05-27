package models

import (
	"errors"
	"time"
)

// ErrRecordNotFound is returned when a record is not found in the store.
var ErrRecordNotFound = errors.New("record not found") // Exported

// FileHistoryRecord defines the structure for storing file monitoring history.
// Note: The parquet tags define ZSTD compression for relevant fields.
// If ZSTD is not directly supported by a simple tag in your parquet-go version for all fields,
// compression might need to be set as a WriterOption or per-column in a custom schema.
// For now, we assume ",zstd" in tags works like ",snappy".
type FileHistoryRecord struct {
	URL            string  `parquet:"url,zstd"`
	Timestamp      int64   `parquet:"timestamp,zstd"`
	Hash           string  `parquet:"hash,zstd"`
	ContentType    string  `parquet:"content_type,zstd,optional"`
	Content        []byte  `parquet:"content,zstd,optional"`
	ETag           string  `parquet:"etag,zstd,optional"`
	LastModified   string  `parquet:"last_modified,zstd,optional"`
	DiffResultJSON *string `parquet:"diff_result_json,zstd,optional"`
	// ExtractedPathsJSON stores the JSON string representation of []models.ExtractedPath
	// This is for JS files primarily, to store paths found within them.
	ExtractedPathsJSON *string `parquet:"extracted_paths_json,zstd,optional"`
}

// FileHistoryStore defines the interface for storing and retrieving file history.
type FileHistoryStore interface {
	// GetLastKnownRecord retrieves the most recent FileHistoryRecord for a given URL.
	// Returns nil, nil if no record is found.
	GetLastKnownRecord(url string) (*FileHistoryRecord, error)

	// GetLastKnownHash retrieves the most recent hash for a given URL.
	// This can be a convenience method if only the hash is needed.
	GetLastKnownHash(url string) (string, error)

	// StoreFileRecord stores a new version of a monitored file.
	StoreFileRecord(record FileHistoryRecord) error

	// GetFileHistory retrieves all historical records for a given URL (optional, for more advanced diffing later).
	// GetFileHistory(url string) ([]FileHistoryRecord, error)

	GetLatestRecord(url string) (*FileHistoryRecord, error)
	GetRecordsForURL(url string, limit int) ([]*FileHistoryRecord, error)
	ArchiveHistory(url string) error                      // Archives old records for a URL
	GetAllRecordsWithDiff() ([]*FileHistoryRecord, error) // Added for aggregated diff reporting

	// GetHostnamesWithHistory retrieves a list of unique hostnames that have history records.
	GetHostnamesWithHistory() ([]string, error)

	// DeleteOldRecordsForHost deletes records older than a specified duration for a given hostname.
	DeleteOldRecordsForHost(hostname string, olderThan time.Duration) (int64, error)

	GetAllLatestDiffResultsForURLs(urls []string) (map[string]*ContentDiffResult, error)
	// GetAllDiffResults retrieves all stored diff results, primarily for aggregated reporting.
	// It's up to the implementation to decide how to best fetch these (e.g., from all files, or specific diff storage).
	GetAllDiffResults() ([]ContentDiffResult, error)
}

// DiffOperation defines the type of diff operation (insert, delete, equal).
// ... existing code ...
