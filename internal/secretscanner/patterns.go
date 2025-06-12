package secretscanner

import "regexp"

// RegexRule defines a rule for detecting secrets.
type RegexRule struct {
	ID          string
	Description string
	Regex       *regexp.Regexp
}

// DefaultRules is a list of default regex patterns for secret detection.
var DefaultRules = []RegexRule{
	{
		ID:          "AWS Access Key ID",
		Description: "AWS Access Key ID",
		Regex:       regexp.MustCompile(`\b(AKIA[0-9A-Z]{16})\b`),
	},
	{
		ID:          "AWS Secret Access Key",
		Description: "AWS Secret Access Key",
		// This pattern looks for the key within a variable assignment context to reduce false positives.
		Regex: regexp.MustCompile(`(?i)(?:aws_secret_access_key|aws_secret_key)\s*[:=]\s*['"]([A-Za-z0-9/+=]{40})['"]`),
	},
	{
		ID:          "GitHub Personal Access Token",
		Description: "GitHub Personal Access Token",
		Regex:       regexp.MustCompile(`\b(ghp_[A-Za-z0-9]{36})\b`),
	},
	{
		ID:          "Generic API Key",
		Description: "Generic API Key",
		Regex:       regexp.MustCompile(`\b(sk-[a-zA-Z0-9]{32,50})\b`),
	},
	{
		ID:          "JWT Token",
		Description: "JWT Token",
		Regex:       regexp.MustCompile(`\b(eyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_/+=]*)\b`),
	},
	{
		ID:          "Slack Bot Token",
		Description: "Slack Bot Token",
		Regex:       regexp.MustCompile(`(xoxb-[0-9a-zA-Z]{10,48})`),
	},
	{
		ID:          "Slack Webhook",
		Description: "Slack Webhook",
		Regex:       regexp.MustCompile(`(https://hooks\.slack\.com/services/T[a-zA-Z0-9]{8}/B[a-zA-Z0-9]{8}/[a-zA-Z0-9]{24})`),
	},
	{
		ID:          "Private Key",
		Description: "Private Key",
		Regex:       regexp.MustCompile(`(-----BEGIN(?: [A-Z]+)? PRIVATE KEY-----)`),
	},
}
