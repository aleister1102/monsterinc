package datastore

import (
	"encoding/json"
	stdtime "time"

	time "github.com/aleister1102/monsterinc/internal/common/timeutils"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
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
	ScanSessionID      *string `parquet:"scan_session_id,optional"`      // Unique identifier for the scan session
	ScanTimestamp      int64   `parquet:"scan_timestamp"`                // Timestamp of the current scan session for this record
	FirstSeenTimestamp *int64  `parquet:"first_seen_timestamp,optional"` // Timestamp when this URL was first ever seen
	LastSeenTimestamp  *int64  `parquet:"last_seen_timestamp,optional"`  // Timestamp when this URL was last seen (could be same as ScanTimestamp for new/existing)
}

// TimePtrToUnixMilliOptional converts time.Time to a pointer to int64 (Unix milliseconds).
// Returns nil if the time is zero.
func TimePtrToUnixMilliOptional(t stdtime.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	millis := t.UnixMilli()
	return &millis
}

// ToProbeResult converts a ParquetProbeResult back to a models.ProbeResult.
func (ppr *ParquetProbeResult) ToProbeResult() httpxrunner.ProbeResult {
	var headers map[string]string
	if ppr.HeadersJSON != nil && *ppr.HeadersJSON != "" {
		if err := json.Unmarshal([]byte(*ppr.HeadersJSON), &headers); err != nil {
			// Log or handle error appropriately, for now, headers will be nil
			headers = nil
			return httpxrunner.ProbeResult{}
		}
	}

	var technologies []httpxrunner.Technology
	for _, name := range ppr.Technologies { // Assuming ppr.Technologies is []string
		technologies = append(technologies, httpxrunner.Technology{Name: name})
	}

	return httpxrunner.ProbeResult{
		InputURL:            ppr.OriginalURL,
		FinalURL:            StringFromPtr(ppr.FinalURL),
		Method:              StringFromPtr(ppr.Method),
		Timestamp:           time.UnixMilliToTimeOptional(ppr.LastSeenTimestamp), // Corrected: Call directly from models package
		Error:               StringFromPtr(ppr.ProbeError),
		RootTargetURL:       StringFromPtr(ppr.RootTargetURL),
		StatusCode:          int(Int32FromPtr(ppr.StatusCode)),
		ContentLength:       Int64FromPtr(ppr.ContentLength),
		ContentType:         StringFromPtr(ppr.ContentType),
		Headers:             headers,
		Title:               StringFromPtr(ppr.Title),
		WebServer:           StringFromPtr(ppr.WebServer),
		IPs:                 ppr.IPAddress,
		Technologies:        technologies,
		URLStatus:           StringFromPtr(ppr.DiffStatus),
		OldestScanTimestamp: time.UnixMilliToTimeOptional(ppr.FirstSeenTimestamp), // Corrected: Call directly from models package
	}
}

// Helper functions to convert from pointer to value (or zero value if nil)
// These should be in a shared utility or models package if used elsewhere.
func StringFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func Int32FromPtr(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

func Int64FromPtr(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}
