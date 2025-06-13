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
	{
		ID:          "Cloudinary",
		Description: "Cloudinary URL",
		Regex:       regexp.MustCompile(`cloudinary://.*`),
	},
	{
		ID:          "Firebase URL",
		Description: "Firebase URL",
		Regex:       regexp.MustCompile(`.*firebaseio\.com`),
	},
	{
		ID:          "Slack Token Extended",
		Description: "Slack Token (xox patterns)",
		Regex:       regexp.MustCompile(`(xox[p|b|o|a]-[0-9]{12}-[0-9]{12}-[0-9]{12}-[a-z0-9]{32})`),
	},
	{
		ID:          "RSA Private Key",
		Description: "RSA Private Key",
		Regex:       regexp.MustCompile(`-----BEGIN RSA PRIVATE KEY-----`),
	},
	{
		ID:          "SSH DSA Private Key",
		Description: "SSH DSA Private Key",
		Regex:       regexp.MustCompile(`-----BEGIN DSA PRIVATE KEY-----`),
	},
	{
		ID:          "SSH EC Private Key",
		Description: "SSH EC Private Key",
		Regex:       regexp.MustCompile(`-----BEGIN EC PRIVATE KEY-----`),
	},
	{
		ID:          "PGP Private Key Block",
		Description: "PGP Private Key Block",
		Regex:       regexp.MustCompile(`-----BEGIN PGP PRIVATE KEY BLOCK-----`),
	},
	{
		ID:          "Amazon MWS Auth Token",
		Description: "Amazon MWS Auth Token",
		Regex:       regexp.MustCompile(`amzn\.mws\.[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
	},
	{
		ID:          "Facebook Access Token",
		Description: "Facebook Access Token",
		Regex:       regexp.MustCompile(`EAACEdEose0cBA[0-9A-Za-z]+`),
	},
	{
		ID:          "Facebook OAuth",
		Description: "Facebook OAuth Token",
		Regex:       regexp.MustCompile(`(?i)(?:facebook[_-]?(?:app[_-]?)?(?:secret|token|key|id))\s*[:=]\s*['"][0-9a-f]{32}['"]`),
	},
	{
		ID:          "GitHub Token",
		Description: "GitHub Token Assignment",
		Regex:       regexp.MustCompile(`(?i)(?:github[_-]?(?:token|key|secret))\s*[:=]\s*['"][0-9a-zA-Z]{35,40}['"]`),
	},
	{
		ID:          "Generic API Key (Extended)",
		Description: "Generic API Key with assignment",
		Regex:       regexp.MustCompile(`(?i)(?:api[_-]?key|apikey)\s*[:=]\s*['"][0-9a-zA-Z]{32,64}['"]`),
	},

	{
		ID:          "Google API Key",
		Description: "Google API Key",
		Regex:       regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
	},
	{
		ID:          "Google Cloud Platform OAuth",
		Description: "Google Cloud Platform OAuth",
		Regex:       regexp.MustCompile(`[0-9]+-[0-9A-Za-z_]{32}\.apps\.googleusercontent\.com`),
	},
	{
		ID:          "Google Service Account",
		Description: "Google Service Account",
		Regex:       regexp.MustCompile(`"type": "service_account"`),
	},
	{
		ID:          "Google OAuth Access Token",
		Description: "Google OAuth Access Token",
		Regex:       regexp.MustCompile(`ya29\.[0-9A-Za-z_-]+`),
	},
	{
		ID:          "Heroku API Key",
		Description: "Heroku API Key",
		Regex:       regexp.MustCompile(`(?i)(?:heroku[_-]?(?:api[_-]?key|key|token))\s*[:=]\s*['"][0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}['"]`),
	},
	{
		ID:          "MailChimp API Key",
		Description: "MailChimp API Key",
		Regex:       regexp.MustCompile(`[0-9a-f]{32}-us[0-9]{1,2}`),
	},
	{
		ID:          "Mailgun API Key",
		Description: "Mailgun API Key",
		Regex:       regexp.MustCompile(`key-[0-9a-zA-Z]{32}`),
	},
	{
		ID:          "Password in URL",
		Description: "Password in URL",
		Regex:       regexp.MustCompile(`[a-zA-Z]{3,10}://[^/\\s:@]{3,20}:[^/\\s:@]{3,20}@.{1,100}["'\\s]`),
	},
	{
		ID:          "PayPal Braintree Access Token",
		Description: "PayPal Braintree Access Token",
		Regex:       regexp.MustCompile(`access_token\\$production\\$[0-9a-z]{16}\\$[0-9a-f]{32}`),
	},
	{
		ID:          "Picatic API Key",
		Description: "Picatic API Key",
		Regex:       regexp.MustCompile(`sk_live_[0-9a-z]{32}`),
	},
	{
		ID:          "Stripe API Key",
		Description: "Stripe API Key",
		Regex:       regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24}`),
	},
	{
		ID:          "Stripe Restricted API Key",
		Description: "Stripe Restricted API Key",
		Regex:       regexp.MustCompile(`rk_live_[0-9a-zA-Z]{24}`),
	},
	{
		ID:          "Square Access Token",
		Description: "Square Access Token",
		Regex:       regexp.MustCompile(`sq0atp-[0-9A-Za-z_-]{22}`),
	},
	{
		ID:          "Square OAuth Secret",
		Description: "Square OAuth Secret",
		Regex:       regexp.MustCompile(`sq0csp-[0-9A-Za-z_-]{43}`),
	},
	{
		ID:          "Twilio API Key",
		Description: "Twilio API Key",
		Regex:       regexp.MustCompile(`SK[0-9a-fA-F]{32}`),
	},
	{
		ID:          "Twitter Access Token",
		Description: "Twitter Access Token",
		Regex:       regexp.MustCompile(`(?i)(?:twitter[_-]?(?:access[_-]?token|token))\s*[:=]\s*['"][1-9][0-9]+-[0-9a-zA-Z]{40}['"]`),
	},
	{
		ID:          "Twitter OAuth",
		Description: "Twitter OAuth Secret",
		Regex:       regexp.MustCompile(`(?i)(?:twitter[_-]?(?:consumer[_-]?secret|secret|key))\s*[:=]\s*['"][0-9a-zA-Z]{35,44}['"]`),
	},
	{
		ID:          "GitHub Fine-Grained Token",
		Description: "GitHub Fine-Grained Personal Access Token",
		Regex:       regexp.MustCompile(`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`),
	},
	{
		ID:          "GitHub OAuth Token",
		Description: "GitHub OAuth Access Token",
		Regex:       regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),
	},
	{
		ID:          "GitHub User-to-Server Token",
		Description: "GitHub User-to-Server Access Token",
		Regex:       regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`),
	},
	{
		ID:          "GitHub Server-to-Server Token",
		Description: "GitHub Server-to-Server Access Token",
		Regex:       regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`),
	},
	{
		ID:          "GitHub Refresh Token",
		Description: "GitHub Refresh Token",
		Regex:       regexp.MustCompile(`ghr_[a-zA-Z0-9]{36}`),
	},
	{
		ID:          "Slack OAuth v2 Bot Token",
		Description: "Slack OAuth v2 Bot Access Token",
		Regex:       regexp.MustCompile(`xoxb-[0-9]{11}-[0-9]{11}-[0-9a-zA-Z]{24}`),
	},
	{
		ID:          "Slack OAuth v2 User Token",
		Description: "Slack OAuth v2 User Access Token",
		Regex:       regexp.MustCompile(`xoxp-[0-9]{11}-[0-9]{11}-[0-9a-zA-Z]{24}`),
	},
	{
		ID:          "Slack Configuration Token",
		Description: "Slack OAuth v2 Configuration Token",
		Regex:       regexp.MustCompile(`xoxe\.xoxp-1-[0-9a-zA-Z]{166}`),
	},
	{
		ID:          "Slack Refresh Token",
		Description: "Slack OAuth v2 Refresh Token",
		Regex:       regexp.MustCompile(`xoxe-1-[0-9a-zA-Z]{147}`),
	},
	{
		ID:          "Slack Webhook URL",
		Description: "Slack Webhook URL",
		Regex:       regexp.MustCompile(`T[a-zA-Z0-9_]{8}/B[a-zA-Z0-9_]{8}/[a-zA-Z0-9_]{24}`),
	},
	{
		ID:          "OpenAI User API Key",
		Description: "OpenAI User API Key",
		Regex:       regexp.MustCompile(`sk-[A-Za-z0-9]{20}T3BlbkFJ[A-Za-z0-9]{20}`),
	},
	{
		ID:          "OpenAI Project Key",
		Description: "OpenAI User Project Key",
		Regex:       regexp.MustCompile(`sk-proj-[A-Za-z0-9]{20}T3BlbkFJ[A-Za-z0-9]{20}`),
	},
	{
		ID:          "WakaTime API Key",
		Description: "WakaTime API Key",
		Regex:       regexp.MustCompile(`waka_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`),
	},
	{
		ID:          "Authorization Basic",
		Description: "Authorization Basic Header",
		Regex:       regexp.MustCompile(`(?i)(?:authorization:\s*|auth:\s*|^)basic\s+[a-zA-Z0-9+/]{16,}={0,2}`),
	},
	{
		ID:          "Authorization Bearer",
		Description: "Authorization Bearer Token",
		Regex:       regexp.MustCompile(`(?i)(?:authorization:\s*|auth:\s*|^)bearer\s+[a-zA-Z0-9_\\.=-]{16,}`),
	},

	{
		ID:          "Cloudinary Basic Auth",
		Description: "Cloudinary Basic Auth",
		Regex:       regexp.MustCompile(`cloudinary://[0-9]{15}:[0-9A-Za-z]+@[a-z]+`),
	},
	{
		ID:          "Artifactory API Token",
		Description: "Artifactory API Token",
		Regex:       regexp.MustCompile(`(?:\s|=|:|"|^)AKC[a-zA-Z0-9]{10,}`),
	},
	{
		ID:          "Artifactory Password",
		Description: "Artifactory Password",
		Regex:       regexp.MustCompile(`(?:\s|=|:|"|^)AP[\dABCDEF][a-zA-Z0-9]{8,}`),
	},
	{
		ID:          "Basic Auth Credentials",
		Description: "Basic Auth Credentials in URL",
		Regex:       regexp.MustCompile(`://[a-zA-Z0-9]+:[a-zA-Z0-9]+@[a-zA-Z0-9]+\.[a-zA-Z]+`),
	},
	{
		ID:          "Google OAuth 2.0 Refresh Token",
		Description: "Google OAuth 2.0 Refresh Token",
		Regex:       regexp.MustCompile(`\b1/[0-9A-Za-z-]{43}\b|\b1/[0-9A-Za-z-]{64}\b`),
	},
	{
		ID:          "Google OAuth 2.0 Auth Code",
		Description: "Google OAuth 2.0 Authorization Code",
		Regex:       regexp.MustCompile(`\b4/[0-9A-Za-z_-]{20,200}\b`),
	},
}
