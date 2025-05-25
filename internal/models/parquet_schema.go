package models

import (
	"time"
)

// Required for TLSCertExpiry potentially if it becomes time.Time

// ParquetProbeResult defines the schema for storing probe results in Parquet format.
// Fields are made optional if they might not be present for every record.
// Timestamps are generally stored as int64 (UnixMilli or UnixNano as per parquet-go conventions).
// Optional fields use pointers and ',optional' tag if needed, though often type inference is sufficient.
// Slices are used for REPEATED/LIST types.
// Maps are handled by marshalling to JSON string and storing as a string.
type ParquetProbeResult struct {
	OriginalURL   string   `parquet:"original_url"` // REQUIRED by default
	FinalURL      *string  `parquet:"final_url,optional"`
	StatusCode    *int32   `parquet:"status_code,optional"`
	ContentLength *int64   `parquet:"content_length,optional"`
	ContentType   *string  `parquet:"content_type,optional"`
	Title         *string  `parquet:"title,optional"`
	WebServer     *string  `parquet:"web_server,optional"`
	Technologies  []string `parquet:"technologies,list"`
	IPAddress     []string `parquet:"ip_address,list"`
	RootTargetURL *string  `parquet:"root_target_url,optional"`
	ProbeError    *string  `parquet:"probe_error,optional"`
	Method        *string  `parquet:"method,optional"`
	HeadersJSON   *string  `parquet:"headers_json,optional"` // Storing map as JSON string

	// New fields for diffing and timestamping
	DiffStatus         *string `parquet:"diff_status,optional"`          // "new", "old", "existing"
	ScanTimestamp      int64   `parquet:"scan_timestamp"`                // Timestamp of the current scan session for this record
	FirstSeenTimestamp *int64  `parquet:"first_seen_timestamp,optional"` // Timestamp when this URL was first ever seen
	LastSeenTimestamp  *int64  `parquet:"last_seen_timestamp,optional"`  // Timestamp when this URL was last seen (could be same as ScanTimestamp for new/existing)
}

// TimePtrToUnixMilliOptional converts time.Time to a pointer to int64 (Unix milliseconds).
// Returns nil if the time is zero.
func TimePtrToUnixMilliOptional(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	millis := t.UnixMilli()
	return &millis
}
