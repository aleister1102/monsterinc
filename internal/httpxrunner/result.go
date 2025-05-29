package httpxrunner

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/models" // Import models package
)

// ProbeResult is now defined in internal/models/probe_result.go
// type ProbeResult struct { ... }

// Technology is now defined in internal/models/probe_result.go
// type Technology struct { ... }

// SetProbeError sets the error message on a ProbeResult and clears/resets potentially inconsistent fields.
// This is useful when a probe fundamentally fails and other data is not reliable.
func SetProbeError(r *models.ProbeResult, errMsg string) {
	if r == nil {
		return
	}
	r.Error = errMsg
	r.StatusCode = 0
	r.ContentLength = 0
	r.ContentType = ""
	r.Headers = nil
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

// IsProbeSuccess returns true if the probe was successful (no error reported by httpx).
func IsProbeSuccess(r *models.ProbeResult) bool {
	if r == nil {
		return false
	}
	return r.Error == ""
}

// ProbeHasTechnologies returns true if any technologies were detected in the probe result.
func ProbeHasTechnologies(r *models.ProbeResult) bool {
	if r == nil {
		return false
	}
	return len(r.Technologies) > 0
}

// ProbeHasTLS returns true if TLS information (version) is available in the probe result.
func ProbeHasTLS(r *models.ProbeResult) bool {
	if r == nil {
		return false
	}
	return r.TLSVersion != ""
}
