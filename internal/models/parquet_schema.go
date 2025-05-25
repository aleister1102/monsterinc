package models

// Required for TLSCertExpiry potentially if it becomes time.Time

// ParquetProbeResult defines the schema for storing probe results using parquet-go/parquet-go.
// Tags are based on parquet-go/parquet-go conventions.
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
	Technologies  []string `parquet:"technologies,list"` // Explicitly mark as list if needed
	IPAddress     []string `parquet:"ip_address,list"`
	ScanTimestamp int64    `parquet:"scan_timestamp"` // REQUIRED (e.g., TIMESTAMP_MILLIS)
	RootTargetURL *string  `parquet:"root_target_url,optional"`
	ProbeError    *string  `parquet:"probe_error,optional"`
	Method        *string  `parquet:"method,optional"`
	Duration      *float64 `parquet:"duration_seconds,optional"`
	HeadersJSON   *string  `parquet:"headers_json,optional"` // Storing map as JSON string
	CNAMEs        []string `parquet:"cnames,list"`
	ASN           *int32   `parquet:"asn,optional"`
	ASNOrg        *string  `parquet:"asn_org,optional"`
	TLSVersion    *string  `parquet:"tls_version,optional"`
	TLSCipher     *string  `parquet:"tls_cipher,optional"`
	TLSCertIssuer *string  `parquet:"tls_cert_issuer,optional"`
	TLSCertExpiry *int64   `parquet:"tls_cert_expiry,optional"` // e.g., TIMESTAMP_MILLIS. If actual time.Time is used, parquet-go handles it.
}
