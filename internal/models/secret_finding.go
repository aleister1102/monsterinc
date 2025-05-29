package models

import "time"

// SecretFinding represents a secret found by a detection tool.
// Tags are for Parquet storage.
type SecretFinding struct {
	SourceURL         string    `parquet:"name=source_url, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"source_url"`
	FilePathInArchive string    `parquet:"name=file_path_in_archive, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"file_path_in_archive"`
	RuleID            string    `parquet:"name=rule_id, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=REQUIRED" json:"rule_id"`
	Description       string    `parquet:"name=description, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"description"`
	Severity          string    `parquet:"name=severity, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"severity"`
	SecretText        string    `parquet:"name=secret_text, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=REQUIRED" json:"secret_text"`
	LineNumber        int       `parquet:"name=line_number, type=INT32, repetitiontype=OPTIONAL" json:"line_number"`
	Timestamp         time.Time `parquet:"name=timestamp, type=INT64, convertedtype=TIMESTAMP_MILLIS, repetitiontype=REQUIRED" json:"timestamp"`
	ToolName          string    `parquet:"name=tool_name, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"tool_name"`
	VerificationState string    `parquet:"name=verification_state, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"verification_state"` // e.g., "Verified", "Unverified", "Attempted"
	ExtraDataJSON     string    `parquet:"name=extra_data_json, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"extra_data_json"`       // JSON string instead of map
}
