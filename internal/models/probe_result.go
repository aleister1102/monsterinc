package models

import "time"

// ProbeResult represents the result of a single httpx probe, serving as a centralized model.
type ProbeResult struct {
	// Basic information
	InputURL      string    `json:"input_url"` // Renamed from OriginalURL for consistency with httpxrunner
	Method        string    `json:"method"`
	Timestamp     time.Time `json:"timestamp"`
	Duration      float64   `json:"duration,omitempty"` // in seconds
	Error         string    `json:"error,omitempty"`
	RootTargetURL string    `json:"root_target_url,omitempty"` // Added for multi-target report navigation

	// HTTP response information
	StatusCode    int               `json:"status_code,omitempty"`
	ContentLength int64             `json:"content_length,omitempty"`
	ContentType   string            `json:"content_type,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"` // Response headers
	Body          string            `json:"body,omitempty"`    // Consider if full body is needed in this central model or just for specific processing
	Title         string            `json:"title,omitempty"`
	WebServer     string            `json:"webserver,omitempty"`

	// Redirect information
	FinalURL string `json:"final_url,omitempty"`

	// DNS information
	IPs    []string `json:"ips,omitempty"`
	CNAMEs []string `json:"cnames,omitempty"`
	ASN    int      `json:"asn,omitempty"`
	ASNOrg string   `json:"asn_org,omitempty"`

	// Technology detection
	Technologies []Technology `json:"technologies,omitempty"`

	// TLS information
	TLSVersion    string    `json:"tls_version,omitempty"`
	TLSCipher     string    `json:"tls_cipher,omitempty"`
	TLSCertIssuer string    `json:"tls_cert_issuer,omitempty"`
	TLSCertExpiry time.Time `json:"tls_cert_expiry,omitempty"`
	// Placeholder for more detailed TLSData if needed, like the one previously in models.TLSData
	// For now, keeping fields flat as in httpxrunner.ProbeResult for simplicity

	// Diffing information (to be populated before writing to Parquet)
	URLStatus           string    `json:"url_status,omitempty"`            // "new", "old", "existing"
	OldestScanTimestamp time.Time `json:"oldest_scan_timestamp,omitempty"` // Timestamp of the very first scan, or historical record
}

// HasTechnologies returns true if any technologies were detected in the probe result.
func (pr *ProbeResult) HasTechnologies() bool {
	if pr == nil {
		return false
	}
	return len(pr.Technologies) > 0
}

// Technology represents a detected technology
// This is moved here from httpxrunner to centralize models.
type Technology struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Category string `json:"category,omitempty"`
}

// TLSData holds basic TLS/SSL certificate information.
type TLSData struct {
	SubjectCN   string    `json:"subject_cn"`
	IssuerCN    string    `json:"issuer_cn"`
	SANs        []string  `json:"sans"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	Certificate string    `json:"certificate_pem"` // Full PEM encoded certificate
}

// Placeholder for actual ProbeResult structure from httpxrunner if different.
// Ensure fields align with what httpxrunner provides and what the report needs.
// For example, Technologies might be a slice of strings.
// IPAddress might also be a slice or more complex if CNAMEs/multiple IPs are involved.
