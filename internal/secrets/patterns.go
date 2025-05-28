package secrets

import "regexp"

// RegexPattern defines the structure for a custom secret detection pattern.
type RegexPattern struct {
	RuleID      string         `json:"rule_id" yaml:"rule_id"`
	Description string         `json:"description" yaml:"description"`
	Pattern     string         `json:"pattern" yaml:"pattern"`
	Severity    string         `json:"severity" yaml:"severity"` // e.g., CRITICAL, HIGH, MEDIUM, LOW, INFO
	Compiled    *regexp.Regexp `json:"-" yaml:"-"`               // Compiled version of the regex pattern, ignored by marshallers

	// Optional fields for more context or refinement
	Keywords   []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`       // Keywords that might appear near the secret
	Entropy    float64  `json:"entropy,omitempty" yaml:"entropy,omitempty"`         // Minimum Shannon entropy for a match (if applicable)
	MaxFinds   int      `json:"max_finds,omitempty" yaml:"max_finds,omitempty"`     // Max number of findings for this rule in a single scan
	LineLength int      `json:"line_length,omitempty" yaml:"line_length,omitempty"` // Max line length to consider for this pattern to reduce FP on long encoded strings
}

// DefaultRegexPatterns returns a list of predefined regex patterns.
// These are examples and should be expanded and refined based on common secret formats.
func DefaultRegexPatterns() ([]RegexPattern, error) {
	patterns := []RegexPattern{
		{
			RuleID:      "GENERIC-API-KEY",
			Description: "Generic API Key (High Entropy String)",
			Pattern:     `(?i)(apikey|api_key|api-key|authorization_token|auth_token|access_token|secret_key|client_secret|bearer_token)[\s:=]+\"([a-zA-Z0-9_\-/\.+]{32,128})\"`,
			Severity:    "HIGH",
			Keywords:    []string{"api_key", "apikey", "secret_key", "client_secret", "access_token", "auth_token", "bearer_token", "authorization_token"},
		},
		{
			RuleID:      "AWS-ACCESS-KEY-ID",
			Description: "AWS Access Key ID",
			Pattern:     `(A3T[A-Z0-9]|AKIA|AGPA|AROA|ASCA|ASIA)[A-Z0-9]{16}`,
			Severity:    "CRITICAL",
			Keywords:    []string{"aws_access_key_id", "aws_secret_access_key"},
		},
		{
			RuleID:      "RSA-PRIVATE-KEY",
			Description: "RSA Private Key",
			Pattern:     `-----BEGIN RSA PRIVATE KEY-----`, // Simple check for header, full key is multiline
			Severity:    "CRITICAL",
		},
		{
			RuleID:      "SSH-PRIVATE-KEY-OPENSSH",
			Description: "OpenSSH Private Key",
			Pattern:     `-----BEGIN OPENSSH PRIVATE KEY-----`,
			Severity:    "CRITICAL",
		},
		{
			RuleID:      "SSH-PRIVATE-KEY-DSA-EC",
			Description: "DSA or EC Private Key",
			Pattern:     `-----BEGIN (DSA|EC) PRIVATE KEY-----`,
			Severity:    "CRITICAL",
		},
		{
			RuleID:      "SLACK-TOKEN",
			Description: "Slack Token (xoxp, xoxb, xapp)",
			Pattern:     `(xox[pbaroa])-[0-9]{10,13}-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{32,40}`,
			Severity:    "HIGH",
			Keywords:    []string{"slack_token", "slack_api_token"},
		},
		{
			RuleID:      "GITHUB-TOKEN",
			Description: "GitHub Personal Access Token (classic or fine-grained)",
			Pattern:     `(ghp|gho|ghu|ghs|ghr)_[a-zA-Z0-9]{36,255}`,
			Severity:    "CRITICAL",
			Keywords:    []string{"github_token", "GITHUB_TOKEN"},
		},
		{
			RuleID:      "STRIPE-API-KEY",
			Description: "Stripe API Key (Live or Test)",
			Pattern:     `(sk|pk)_(live|test)_[0-9a-zA-Z]{24,99}`,
			Severity:    "HIGH",
			Keywords:    []string{"stripe_api_key", "stripe_secret_key", "STRIPE_API_KEY"},
		},
		{
			RuleID:      "GENERIC-PASSWORD-IN-URL",
			Description: "Password in URL (e.g., user:password@host)",
			Pattern:     `[a-zA-Z0-9]+://[^:]+:[^@\s]+@[^\s]+`,
			Severity:    "MEDIUM",
		},
		{
			RuleID:      "PLACEHOLDER-PII",
			Description: "Placeholder for Personally Identifiable Information (PII) like SSN or Phone (Example)",
			Pattern:     `\b(?:\d{3}[-\s]?\d{2}[-\s]?\d{4}|\(\d{3}\)[\s.-]?\d{3}[\s.-]?\d{4})\b`, // Example for SSN/Phone
			Severity:    "INFO",                                                                  // Severity might be higher depending on context
		},
	}

	// Compile all regex patterns
	for i := range patterns {
		compiledRegex, err := regexp.Compile(patterns[i].Pattern)
		if err != nil {
			// This should ideally not happen for default patterns, indicates a bad predefined regex
			return nil, err // Or log and skip the bad pattern
		}
		patterns[i].Compiled = compiledRegex
	}
	return patterns, nil
}
