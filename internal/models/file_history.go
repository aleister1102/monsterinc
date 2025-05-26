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
	URL          string    `parquet:"url,zstd"`
	Timestamp    time.Time `parquet:"timestamp,zstd"`
	Hash         string    `parquet:"hash,zstd"`
	ContentType  string    `parquet:"content_type,zstd,optional"`
	Content      []byte    `parquet:"content,zstd,optional"`
	ETag         string    `parquet:"etag,zstd,optional"`
	LastModified string    `parquet:"last_modified,zstd,optional"`
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
}
