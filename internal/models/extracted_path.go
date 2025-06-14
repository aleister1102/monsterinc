package models

import "time"

// ExtractedPath holds information about a URL or path found within content.
type ExtractedPath struct {
	SourceURL            string    `parquet:"source_url,plain_dictionary,utf8" json:"source_url"`
	ExtractedRawPath     string    `parquet:"extracted_raw_path,plain_dictionary,utf8" json:"extracted_raw_path"`
	ExtractedAbsoluteURL string    `parquet:"extracted_absolute_url,plain_dictionary,utf8" json:"extracted_absolute_url"`
	Context              string    `parquet:"context,plain_dictionary,utf8" json:"context"` // e.g., "<a>[href]", "script[src]", "JS:string_literal"
	Type                 string    `parquet:"type,plain_dictionary,utf8" json:"type"`       // e.g., "html_attr_link", "html_attr_script", "js_string", "js_fetch_param"
	DiscoveryTimestamp   time.Time `parquet:"discovery_timestamp,timestamp" json:"discovery_timestamp"`
}
