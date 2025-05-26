package models

import "time"

// MonitoredFile represents the data structure for a file being monitored.
// This might be used by the Processor and stored by the FileHistoryStore.
type MonitoredFile struct {
	URL         string    `json:"url"`
	LastHash    string    `json:"last_hash,omitempty"`
	LastChecked time.Time `json:"last_checked,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	// Other relevant fields can be added here, e.g.:
	// ETag string `json:"etag,omitempty"`
	// LastModified string `json:"last_modified,omitempty"`
	// IsNew bool `json:"is_new,omitempty"`
}

// MonitoredFileUpdate might be returned by the Processor after fetching and processing a file.
// It contains the essential information for the MonitoringService to decide on storage and notification.
type MonitoredFileUpdate struct {
	URL         string
	NewHash     string
	ContentType string
	FetchedAt   time.Time
	Content     []byte // Optional: Only if full content is needed by the service directly
}
