package models

import "time"

// ExtractedPath holds information about a URL or path found within content.
type ExtractedPath struct {
	SourceURL            string    `parquet:"name=source_url, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"source_url"`
	ExtractedRawPath     string    `parquet:"name=extracted_raw_path, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"extracted_raw_path"`
	ExtractedAbsoluteURL string    `parquet:"name=extracted_absolute_url, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=REQUIRED" json:"extracted_absolute_url"`
	Context              string    `parquet:"name=context, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"context"` // e.g., "<a>[href]", "script[src]", "JS:string_literal"
	Type                 string    `parquet:"name=type, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL" json:"type"`       // e.g., "html_attr_link", "html_attr_script", "js_string", "js_fetch_param"
	DiscoveryTimestamp   time.Time `parquet:"name=discovery_timestamp, type=INT64, convertedtype=TIMESTAMP_MILLIS, repetitiontype=REQUIRED" json:"discovery_timestamp"`
}
