package httpxrunner

import (
	"time"
)

// ProbeResult represents the result of a single httpx probe
type ProbeResult struct {
	// Basic information
	InputURL  string    `json:"input_url"`
	Method    string    `json:"method"`
	Timestamp time.Time `json:"timestamp"`
	Duration  float64   `json:"duration,omitempty"` // in seconds
	Error     string    `json:"error,omitempty"`

	// HTTP response information
	StatusCode    int               `json:"status_code,omitempty"`
	ContentLength int64             `json:"content_length,omitempty"`
	ContentType   string            `json:"content_type,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	Title         string            `json:"title,omitempty"`     // Task 3.4.6
	WebServer     string            `json:"webserver,omitempty"` // Task 3.4.7

	// Redirect information
	FinalURL string `json:"final_url,omitempty"`
	// RedirectURLs []string `json:"redirect_urls,omitempty"` // runner.Result has Chain, but might be too much detail, FinalURL is usually enough

	// DNS information - Task 3.6
	IPs    []string `json:"ips,omitempty"`     // Mapped from runner.Result.A
	CNAMEs []string `json:"cnames,omitempty"`  // Mapped from runner.Result.CNAMEs
	ASN    int      `json:"asn,omitempty"`     // Mapped from runner.Result.ASN.ASN (numeric part)
	ASNOrg string   `json:"asn_org,omitempty"` // Mapped from runner.Result.ASN.Organization
	// Country string `json:"country,omitempty"` // Not directly available in core runner.Result, might need GeoIP lookup post-processing
	// City    string `json:"city,omitempty"`    // Same as Country

	// Technology detection - Task 3.5.2
	Technologies []Technology `json:"technologies,omitempty"`

	// TLS information - Task 3.4.12
	TLSVersion    string    `json:"tls_version,omitempty"`     // Mapped from runner.Result.TLSData.TlsVersion
	TLSCipher     string    `json:"tls_cipher,omitempty"`      // Mapped from runner.Result.TLSData.GetCipherSuite()
	TLSCertIssuer string    `json:"tls_cert_issuer,omitempty"` // Mapped from runner.Result.TLSData.CertificateResponse.Chain[0].Issuer
	TLSCertExpiry time.Time `json:"tls_cert_expiry,omitempty"` // Mapped from runner.Result.TLSData.CertificateResponse.Chain[0].NotAfter

	// RawJSONOutput string `json:"raw_json_output,omitempty"` // Task 3.4.13 - Decided to omit as per previous discussions
}

// Technology represents a detected technology
type Technology struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`  // Populated from runner.Result.TechnologyDetails
	Category string `json:"category,omitempty"` // Populated from runner.Result.TechnologyDetails
}

// NewProbeResult creates a new ProbeResult with default values
// This might not be strictly necessary if direct struct literal initialization is always used.
func NewProbeResult() *ProbeResult {
	return &ProbeResult{
		Timestamp:    time.Now(), // Default to now, but httpx result's timestamp will usually overwrite
		Headers:      make(map[string]string),
		IPs:          make([]string, 0),
		CNAMEs:       make([]string, 0),
		Technologies: make([]Technology, 0),
	}
}

// SetError sets the error message and clears/resets potentially inconsistent fields.
// This is useful when a probe fundamentally fails and other data is not reliable.
func (r *ProbeResult) SetError(errMsg string) {
	r.Error = errMsg
	r.StatusCode = 0
	r.ContentLength = 0
	r.ContentType = ""
	r.Headers = nil // Or make(map[string]string) if prefer empty map over nil
	r.Body = ""
	r.Title = ""
	r.WebServer = ""
	r.FinalURL = ""
	r.IPs = nil
	r.CNAMEs = nil
	r.ASN = 0
	r.ASNOrg = ""
	r.Technologies = nil
	r.TLSVersion = ""
	r.TLSCipher = ""
	r.TLSCertIssuer = ""
	r.TLSCertExpiry = time.Time{}
	r.Duration = 0
}

// IsSuccess returns true if the probe was successful (no error reported by httpx)
func (r *ProbeResult) IsSuccess() bool {
	return r.Error == ""
}

// HasTechnologies returns true if any technologies were detected
func (r *ProbeResult) HasTechnologies() bool {
	return len(r.Technologies) > 0
}

// HasTLS returns true if TLS information (version) is available
func (r *ProbeResult) HasTLS() bool {
	return r.TLSVersion != ""
}
