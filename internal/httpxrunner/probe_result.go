package httpxrunner

import "time"

// ProbeResult holds the structured result of a single HTTP probe.
type ProbeResult struct {
	InputURL      string            `json:"input_url"`
	FinalURL      string            `json:"final_url"`
	RootTargetURL string            `json:"root_target_url"`
	StatusCode    int               `json:"status_code"`
	ContentLength int64             `json:"content_length"`
	Title         string            `json:"title"`
	WebServer     string            `json:"web_server"`
	ContentType   string            `json:"content_type"`
	Body          []byte            `json:"body,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	IPs           []string          `json:"ips,omitempty"`
	Technologies  []Technology      `json:"technologies,omitempty"`
	ASN           int               `json:"asn,omitempty"`
	ASNOrg        string            `json:"asn_org,omitempty"`
	Duration      float64           `json:"duration"`
	Timestamp     time.Time         `json:"timestamp"`
	Error         error             `json:"error,omitempty"`
}

// Technology holds information about a detected technology.
type Technology struct {
	Name       string   `json:"name"`
	Version    string   `json:"version,omitempty"`
	Categories []string `json:"categories,omitempty"`
}
