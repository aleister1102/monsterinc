package models

import "time"

// SecretFinding represents a secret found by a detection tool.
// Tags are for Parquet storage.
type SecretFinding struct {
	SourceURL         string    `parquet:"name=source_url, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	FilePathInArchive string    `parquet:"name=file_path_in_archive, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	RuleID            string    `parquet:"name=rule_id, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=REQUIRED"`
	Description       string    `parquet:"name=description, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	Severity          string    `parquet:"name=severity, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	SecretText        string    `parquet:"name=secret_text, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=REQUIRED"`
	LineNumber        int       `parquet:"name=line_number, type=INT32, repetitiontype=OPTIONAL"`
	Timestamp         time.Time `parquet:"name=timestamp, type=INT64, convertedtype=TIMESTAMP_MILLIS, repetitiontype=REQUIRED"`
	ToolName          string    `parquet:"name=tool_name, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	VerificationState string    `parquet:"name=verification_state, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"` // e.g., "Verified", "Unverified", "Attempted"
	ExtraDataJSON     string    `parquet:"name=extra_data_json, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`   // JSON string instead of map
}
 
