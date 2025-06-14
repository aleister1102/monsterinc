package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProbeResult_HasTechnologies(t *testing.T) {
	tests := []struct {
		name         string
		probeResult  ProbeResult
		expectResult bool
	}{
		{
			name: "has technologies",
			probeResult: ProbeResult{
				Technologies: []Technology{
					{Name: "React", Version: "18.0.0"},
					{Name: "Next.js", Version: "12.0.0"},
				},
			},
			expectResult: true,
		},
		{
			name: "no technologies",
			probeResult: ProbeResult{
				Technologies: []Technology{},
			},
			expectResult: false,
		},
		{
			name: "nil technologies",
			probeResult: ProbeResult{
				Technologies: nil,
			},
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.probeResult.HasTechnologies()
			assert.Equal(t, tt.expectResult, result)
		})
	}
}

func TestTechnology_Complete(t *testing.T) {
	tests := []struct {
		name       string
		technology Technology
	}{
		{
			name: "complete technology",
			technology: Technology{
				Name:     "React",
				Version:  "18.0.0",
				Category: "JavaScript Framework",
			},
		},
		{
			name: "technology without version",
			technology: Technology{
				Name:     "Apache",
				Category: "Web Server",
			},
		},
		{
			name: "technology without category",
			technology: Technology{
				Name:    "jQuery",
				Version: "3.6.0",
			},
		},
		{
			name: "minimal technology",
			technology: Technology{
				Name: "Express.js",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that technology fields are properly set
			assert.NotEmpty(t, tt.technology.Name)

			// Test that technology is valid (has at least a name)
			assert.True(t, len(tt.technology.Name) > 0)
		})
	}
}

func TestProbeResult_Complete(t *testing.T) {
	now := time.Now()

	probeResult := ProbeResult{
		Body:          "<html><body>Test</body></html>",
		CNAMEs:        []string{"example.com", "www.example.com"},
		ContentLength: 1024,
		ContentType:   "text/html",
		Duration:      1.5,
		Error:         "",
		FinalURL:      "https://example.com/final",
		Headers: map[string]string{
			"Content-Type":   "text/html; charset=utf-8",
			"Server":         "nginx/1.20.0",
			"Content-Length": "1024",
		},
		InputURL:            "https://example.com",
		IPs:                 []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"},
		Method:              "GET",
		OldestScanTimestamp: now.Add(-24 * time.Hour),
		RootTargetURL:       "https://example.com",
		StatusCode:          200,
		Technologies: []Technology{
			{Name: "nginx", Version: "1.20.0", Category: "Web Server"},
			{Name: "HTML", Category: "Markup Language"},
		},
		Timestamp: now,
		Title:     "Example Domain",
		URLStatus: "existing",
		WebServer: "nginx/1.20.0",
		ASN:       15133,
		ASNOrg:    "Example Organization",
	}

	// Test individual fields
	assert.Equal(t, "<html><body>Test</body></html>", probeResult.Body)
	assert.Equal(t, []string{"example.com", "www.example.com"}, probeResult.CNAMEs)
	assert.Equal(t, int64(1024), probeResult.ContentLength)
	assert.Equal(t, "text/html", probeResult.ContentType)
	assert.Equal(t, 1.5, probeResult.Duration)
	assert.Empty(t, probeResult.Error)
	assert.Equal(t, "https://example.com/final", probeResult.FinalURL)
	assert.Equal(t, "https://example.com", probeResult.InputURL)
	assert.Equal(t, []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}, probeResult.IPs)
	assert.Equal(t, "GET", probeResult.Method)
	assert.Equal(t, "https://example.com", probeResult.RootTargetURL)
	assert.Equal(t, 200, probeResult.StatusCode)
	assert.Equal(t, "Example Domain", probeResult.Title)
	assert.Equal(t, "existing", probeResult.URLStatus)
	assert.Equal(t, "nginx/1.20.0", probeResult.WebServer)
	assert.Equal(t, 15133, probeResult.ASN)
	assert.Equal(t, "Example Organization", probeResult.ASNOrg)

	// Test that technologies are properly set
	assert.True(t, probeResult.HasTechnologies())
	assert.Equal(t, 2, len(probeResult.Technologies))

	// Test headers
	assert.Equal(t, 3, len(probeResult.Headers))
	assert.Equal(t, "text/html; charset=utf-8", probeResult.Headers["Content-Type"])

	// Test timestamps
	assert.True(t, probeResult.Timestamp.After(probeResult.OldestScanTimestamp))
}

func TestProbeResult_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		probeResult ProbeResult
		hasError    bool
	}{
		{
			name: "no error",
			probeResult: ProbeResult{
				InputURL:   "https://example.com",
				StatusCode: 200,
				Error:      "",
			},
			hasError: false,
		},
		{
			name: "connection error",
			probeResult: ProbeResult{
				InputURL: "https://invalid-domain.example",
				Error:    "connection timeout",
			},
			hasError: true,
		},
		{
			name: "DNS error",
			probeResult: ProbeResult{
				InputURL: "https://nonexistent.domain",
				Error:    "no such host",
			},
			hasError: true,
		},
		{
			name: "HTTP error",
			probeResult: ProbeResult{
				InputURL:   "https://example.com/notfound",
				StatusCode: 404,
				Error:      "HTTP 404 Not Found",
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := len(tt.probeResult.Error) > 0
			assert.Equal(t, tt.hasError, hasError)

			if tt.hasError {
				assert.NotEmpty(t, tt.probeResult.Error)
			} else {
				assert.Empty(t, tt.probeResult.Error)
			}
		})
	}
}

func TestProbeResult_URLStatus(t *testing.T) {
	tests := []struct {
		name      string
		urlStatus string
		valid     bool
	}{
		{
			name:      "new URL",
			urlStatus: "new",
			valid:     true,
		},
		{
			name:      "existing URL",
			urlStatus: "existing",
			valid:     true,
		},
		{
			name:      "old URL",
			urlStatus: "old",
			valid:     true,
		},
		{
			name:      "empty status",
			urlStatus: "",
			valid:     false,
		},
		{
			name:      "invalid status",
			urlStatus: "invalid",
			valid:     false,
		},
	}

	validStatuses := map[string]bool{
		"new":      true,
		"existing": true,
		"old":      true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probeResult := ProbeResult{
				InputURL:  "https://example.com",
				URLStatus: tt.urlStatus,
			}

			isValid := validStatuses[probeResult.URLStatus]
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestProbeResult_NetworkInfo(t *testing.T) {
	probeResult := ProbeResult{
		InputURL: "https://example.com",
		IPs:      []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"},
		CNAMEs:   []string{"example.com"},
		ASN:      15133,
		ASNOrg:   "Example Organization",
	}

	// Test IP addresses
	assert.Equal(t, 2, len(probeResult.IPs))
	assert.Contains(t, probeResult.IPs, "93.184.216.34")
	assert.Contains(t, probeResult.IPs, "2606:2800:220:1:248:1893:25c8:1946")

	// Test CNAMEs
	assert.Equal(t, 1, len(probeResult.CNAMEs))
	assert.Contains(t, probeResult.CNAMEs, "example.com")

	// Test ASN info
	assert.Equal(t, 15133, probeResult.ASN)
	assert.Equal(t, "Example Organization", probeResult.ASNOrg)
}

func TestProbeResult_HTTPInfo(t *testing.T) {
	headers := map[string]string{
		"Content-Type":     "application/json",
		"Content-Length":   "512",
		"Server":           "Apache/2.4.41",
		"X-Powered-By":     "PHP/7.4.0",
		"Cache-Control":    "no-cache",
		"Set-Cookie":       "session=abc123; HttpOnly",
		"X-Frame-Options":  "DENY",
		"X-XSS-Protection": "1; mode=block",
	}

	probeResult := ProbeResult{
		InputURL:      "https://api.example.com/data",
		FinalURL:      "https://api.example.com/data",
		Method:        "GET",
		StatusCode:    200,
		ContentType:   "application/json",
		ContentLength: 512,
		Headers:       headers,
		WebServer:     "Apache/2.4.41",
		Duration:      0.85,
	}

	// Test HTTP response info
	assert.Equal(t, "GET", probeResult.Method)
	assert.Equal(t, 200, probeResult.StatusCode)
	assert.Equal(t, "application/json", probeResult.ContentType)
	assert.Equal(t, int64(512), probeResult.ContentLength)
	assert.Equal(t, "Apache/2.4.41", probeResult.WebServer)
	assert.Equal(t, 0.85, probeResult.Duration)

	// Test headers
	assert.Equal(t, len(headers), len(probeResult.Headers))
	for k, v := range headers {
		assert.Equal(t, v, probeResult.Headers[k])
	}
}

func TestProbeResult_ContentAnalysis(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
    <meta charset="utf-8">
</head>
<body>
    <h1>Welcome to Test Page</h1>
    <p>This is a test page for content analysis.</p>
</body>
</html>`

	probeResult := ProbeResult{
		InputURL:    "https://example.com/test",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		Title:       "Test Page",
		Body:        htmlContent,
		Technologies: []Technology{
			{Name: "HTML", Category: "Markup Language"},
		},
	}

	// Test content properties
	assert.Equal(t, "Test Page", probeResult.Title)
	assert.Equal(t, "text/html; charset=utf-8", probeResult.ContentType)
	assert.Contains(t, probeResult.Body, "Welcome to Test Page")
	assert.Contains(t, probeResult.Body, "<!DOCTYPE html>")

	// Test technologies
	assert.True(t, probeResult.HasTechnologies())
	assert.Equal(t, 1, len(probeResult.Technologies))
	assert.Equal(t, "HTML", probeResult.Technologies[0].Name)
}
