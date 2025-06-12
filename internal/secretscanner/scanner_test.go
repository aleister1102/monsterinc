package secretscanner

import (
	"testing"
)

func TestRegexScanner_Scan(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name      string
		sourceURL string
		content   []byte
		wantCount int
		wantTypes []string
	}{
		{
			name:      "No secrets found",
			sourceURL: "https://example.com/clean.js",
			content:   []byte("var x = 'hello world'; console.log(x);"),
			wantCount: 0,
		},
		{
			name:      "AWS API key",
			sourceURL: "https://example.com/config.js",
			content:   []byte("const AWS_ACCESS_KEY_ID = 'AKIAIOSFODNN7EXAMPLE';"),
			wantCount: 1,
			wantTypes: []string{"AWS Access Key ID"},
		},
		{
			name:      "Multiple secrets",
			sourceURL: "https://example.com/app.js",
			content: []byte(`
				const config = {
					awsKey: 'AKIAIOSFODNN7EXAMPLE',
					githubToken: 'ghp_16C7e42F292c6912E7710c838347Ae178B4a',
					apiSecret: 'sk-1234567890abcdef1234567890abcdef'
				};
			`),
			wantCount: 3,
			wantTypes: []string{"AWS Access Key ID", "GitHub Personal Access Token", "Generic API Key"},
		},
		{
			name:      "JWT token",
			sourceURL: "https://example.com/auth.js",
			content:   []byte("const token = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c';"),
			wantCount: 1,
			wantTypes: []string{"JWT Token"},
		},
		{
			name:      "Slack token",
			sourceURL: "https://example.com/slack.js",
			content:   []byte("const slackBot = 'xoxb-1234567890-1234567890-abcdefghijklmnopqrstuvwx';"),
			wantCount: 1,
			wantTypes: []string{"Slack Bot Token"},
		},
		{
			name:      "False positive - short string",
			sourceURL: "https://example.com/short.js",
			content:   []byte("var a = 'sk-123';"), // Too short
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := scanner.Scan(tt.sourceURL, tt.content)

			if len(findings) != tt.wantCount {
				t.Errorf("Scan() returned %d findings, want %d", len(findings), tt.wantCount)
				for i, finding := range findings {
					t.Logf("Finding %d: Type=%s, Value=%s", i+1, finding.RuleID, finding.SecretText)
				}
				return
			}

			if tt.wantCount > 0 {
				// Verify secret types are as expected
				foundTypes := make(map[string]bool)
				for _, finding := range findings {
					foundTypes[finding.RuleID] = true

					// Verify required fields are set
					if finding.SourceURL != tt.sourceURL {
						t.Errorf("Finding has incorrect SourceURL: got %s, want %s", finding.SourceURL, tt.sourceURL)
					}
					if finding.SecretText == "" {
						t.Errorf("Finding has empty SecretText")
					}
					if finding.RuleID == "" {
						t.Errorf("Finding has empty RuleID")
					}
					if finding.LineNumber < 0 {
						t.Errorf("Finding has negative LineNumber: %d", finding.LineNumber)
					}
				}

				// Check that expected types were found
				for _, expectedType := range tt.wantTypes {
					if !foundTypes[expectedType] {
						t.Errorf("Expected secret type %s was not found", expectedType)
					}
				}
			}
		})
	}
}

func TestRegexScanner_Patterns(t *testing.T) {
	scanner := NewRegexScanner()

	// Test individual patterns
	patternTests := []struct {
		name    string
		content string
		want    bool
	}{
		{"AWS Secret Key", "aws_secret_access_key = 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'", true},
		{"GitHub Token", "token = 'ghp_16C7e42F292c6912E7710c838347Ae178B4a'", true},
		{"Private Key", "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7VJTUt9Us8cKB", true},
		{"API Key Pattern", "api_key = 'sk-1234567890abcdef1234567890abcdef'", true},
		{"Slack Webhook", "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX", true},
		{"Not a secret", "regular text with no secrets", false},
	}

	for _, tt := range patternTests {
		t.Run(tt.name, func(t *testing.T) {
			findings := scanner.Scan("https://test.com", []byte(tt.content))
			found := len(findings) > 0

			if found != tt.want {
				t.Errorf("Pattern test %s: got %v, want %v", tt.name, found, tt.want)
				if found {
					t.Logf("Found: %+v", findings[0])
				}
			}
		})
	}
}

func TestRegexScanner_EdgeCases(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name    string
		content []byte
		want    int
	}{
		{
			name:    "Empty content",
			content: []byte{},
			want:    0,
		},
		{
			name:    "Nil content",
			content: nil,
			want:    0,
		},
		{
			name:    "Very large content",
			content: []byte(generateLargeContent()),
			want:    0, // No secrets in generated content
		},
		{
			name:    "Binary content",
			content: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE},
			want:    0,
		},
		{
			name:    "Unicode content",
			content: []byte("const msg = '你好世界'; // No secrets here"),
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := scanner.Scan("https://test.com", tt.content)
			if len(findings) != tt.want {
				t.Errorf("EdgeCase %s: got %d findings, want %d", tt.name, len(findings), tt.want)
			}
		})
	}
}

// generateLargeContent creates a large string for testing performance
func generateLargeContent() string {
	const chunk = "console.log('This is just normal JavaScript code with no secrets');\n"
	result := ""
	for i := 0; i < 1000; i++ {
		result += chunk
	}
	return result
}

func BenchmarkRegexScanner_Scan(b *testing.B) {
	scanner := NewRegexScanner()
	content := []byte(`
		const config = {
			awsKey: 'AKIAIOSFODNN7EXAMPLE',
			githubToken: 'ghp_16C7e42F292c6912E7710c838347Ae178B4a',
			apiSecret: 'sk-1234567890abcdef1234567890abcdef',
			normalData: 'just some regular text that should not match'
		};
	`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.Scan("https://benchmark.com", content)
	}
}
