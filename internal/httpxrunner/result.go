package httpxrunner

import (
	"time"
)

// ProbeResult represents the result of a single httpx probe
type ProbeResult struct {
	// Basic information
	URL       string    `json:"url"`
	Method    string    `json:"method"`
	Timestamp time.Time `json:"timestamp"`
	Duration  float64   `json:"duration"` // in seconds
	Error     string    `json:"error,omitempty"`

	// HTTP response information
	StatusCode    int               `json:"status_code,omitempty"`
	ContentLength int64             `json:"content_length,omitempty"`
	ContentType   string            `json:"content_type,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`

	// Redirect information
	FinalURL     string   `json:"final_url,omitempty"`
	RedirectURLs []string `json:"redirect_urls,omitempty"`

	// DNS information
	IP      string `json:"ip,omitempty"`
	CNAME   string `json:"cname,omitempty"`
	ASN     int    `json:"asn,omitempty"`
	ASNOrg  string `json:"asn_org,omitempty"`
	Country string `json:"country,omitempty"`
	City    string `json:"city,omitempty"`

	// Technology detection
	Technologies []Technology `json:"technologies,omitempty"`

	// TLS information
	TLSVersion    string    `json:"tls_version,omitempty"`
	TLSCipher     string    `json:"tls_cipher,omitempty"`
	TLSCertIssuer string    `json:"tls_cert_issuer,omitempty"`
	TLSCertExpiry time.Time `json:"tls_cert_expiry,omitempty"`
}

// Technology represents a detected technology
type Technology struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Category string `json:"category,omitempty"`
}

// NewProbeResult creates a new ProbeResult with default values
func NewProbeResult() *ProbeResult {
	return &ProbeResult{
		Timestamp: time.Now(),
		Headers:   make(map[string]string),
	}
}

// SetError sets the error message and clears other fields
func (r *ProbeResult) SetError(err string) {
	r.Error = err
	r.StatusCode = 0
	r.ContentLength = 0
	r.ContentType = ""
	r.Headers = nil
	r.Body = ""
	r.FinalURL = ""
	r.RedirectURLs = nil
	r.IP = ""
	r.CNAME = ""
	r.ASN = 0
	r.ASNOrg = ""
	r.Country = ""
	r.City = ""
	r.Technologies = nil
	r.TLSVersion = ""
	r.TLSCipher = ""
	r.TLSCertIssuer = ""
	r.TLSCertExpiry = time.Time{}
}

// IsSuccess returns true if the probe was successful
func (r *ProbeResult) IsSuccess() bool {
	return r.Error == ""
}

// HasRedirects returns true if the probe resulted in redirects
func (r *ProbeResult) HasRedirects() bool {
	return len(r.RedirectURLs) > 0
}

// HasTechnologies returns true if any technologies were detected
func (r *ProbeResult) HasTechnologies() bool {
	return len(r.Technologies) > 0
}

// HasTLS returns true if TLS information is available
func (r *ProbeResult) HasTLS() bool {
	return r.TLSVersion != ""
}
