package httpxrunner

import "time"

// ProbeResult represents the result of a single httpx probe, serving as a centralized model.
// Refactored ✅
type ProbeResult struct {
	Body                string            `json:"body,omitempty"`
	CNAMEs              []string          `json:"cnames,omitempty"`
	ContentLength       int64             `json:"content_length,omitempty"`
	ContentType         string            `json:"content_type,omitempty"`
	Duration            float64           `json:"duration,omitempty"` // in seconds
	Error               string            `json:"error,omitempty"`
	FinalURL            string            `json:"final_url,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"` // Response headers
	InputURL            string            `json:"input_url"`
	IPs                 []string          `json:"ips,omitempty"`
	Method              string            `json:"method"`
	OldestScanTimestamp time.Time         `json:"oldest_scan_timestamp,omitempty"` // Timestamp of the very first scan, or historical record
	RootTargetURL       string            `json:"root_target_url,omitempty"`
	StatusCode          int               `json:"status_code,omitempty"`
	Technologies        []Technology      `json:"technologies,omitempty"`
	Timestamp           time.Time         `json:"timestamp"`
	Title               string            `json:"title,omitempty"`
	URLStatus           string            `json:"url_status,omitempty"` // "new", "old", "existing"
	WebServer           string            `json:"webserver,omitempty"`
	ASN                 int               `json:"asn,omitempty"`
	ASNOrg              string            `json:"asn_org,omitempty"`
}

// HasTechnologies returns true if any technologies were detected in the probe result.
// No need to refactor ✅
func (pr *ProbeResult) HasTechnologies() bool {
	if pr == nil {
		return false
	}
	return len(pr.Technologies) > 0
}

// Technology represents a detected technology
// This is moved here from httpxrunner to centralize models.
// No need to refactor ✅
type Technology struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Category string `json:"category,omitempty"`
}
