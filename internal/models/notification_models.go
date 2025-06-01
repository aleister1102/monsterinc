package models

import "time"

// DiscordMessagePayload represents the JSON payload sent to a Discord webhook.
type DiscordMessagePayload struct {
	Content         string           `json:"content,omitempty"`          // Message content (text)
	Username        string           `json:"username,omitempty"`         // Override the default webhook username
	AvatarURL       string           `json:"avatar_url,omitempty"`       // Override the default webhook avatar
	TTS             bool             `json:"tts,omitempty"`              // Whether this is a text-to-speech message
	Embeds          []DiscordEmbed   `json:"embeds,omitempty"`           // Array of embed objects
	AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"` // Allowed mentions for the message
	// Files      []interface{}   `json:"files"` // For file uploads, handled by multipart/form-data, not directly in JSON
}

// AllowedMentions specifies how mentions should be handled in a message.
type AllowedMentions struct {
	Parse       []string `json:"parse,omitempty"`        // Types of mentions to parse (e.g., "roles", "users", "everyone")
	Roles       []string `json:"roles,omitempty"`        // Array of role_ids to mention (max 100)
	Users       []string `json:"users,omitempty"`        // Array of user_ids to mention (max 100)
	RepliedUser bool     `json:"replied_user,omitempty"` // For replies, whether to mention the author of the message being replied to
}

// DiscordEmbed represents a Discord embed object.
type DiscordEmbed struct {
	Title       string                 `json:"title,omitempty"`       // Title of embed
	Description string                 `json:"description,omitempty"` // Description of embed
	URL         string                 `json:"url,omitempty"`         // URL of embed
	Timestamp   string                 `json:"timestamp,omitempty"`   // ISO8601 timestamp
	Color       int                    `json:"color,omitempty"`       // Color code of the embed
	Footer      *DiscordEmbedFooter    `json:"footer,omitempty"`
	Image       *DiscordEmbedImage     `json:"image,omitempty"`
	Thumbnail   *DiscordEmbedThumbnail `json:"thumbnail,omitempty"`
	Author      *DiscordEmbedAuthor    `json:"author,omitempty"`
	Fields      []DiscordEmbedField    `json:"fields,omitempty"` // Array of embed field objects
}

// DiscordEmbedFooter represents the footer of an embed.
type DiscordEmbedFooter struct {
	Text    string `json:"text"`               // Footer text
	IconURL string `json:"icon_url,omitempty"` // URL of footer icon (only supports http(s) and attachments)
}

// DiscordEmbedImage represents the image of an embed.
type DiscordEmbedImage struct {
	URL string `json:"url"` // Source URL of image (only supports http(s) and attachments)
}

// DiscordEmbedThumbnail represents the thumbnail of an embed.
type DiscordEmbedThumbnail struct {
	URL string `json:"url"` // Source URL of thumbnail (only supports http(s) and attachments)
}

// DiscordEmbedAuthor represents the author of an embed.
type DiscordEmbedAuthor struct {
	Name    string `json:"name"`               // Name of author
	URL     string `json:"url,omitempty"`      // URL of author (only supports http(s))
	IconURL string `json:"icon_url,omitempty"` // URL of author icon (only supports http(s) and attachments)
}

// DiscordEmbedField represents a field in an embed.
type DiscordEmbedField struct {
	Name   string `json:"name"`             // Name of the field
	Value  string `json:"value"`            // Value of the field
	Inline bool   `json:"inline,omitempty"` // Whether or not this field should display inline
}

// ScanSummaryData holds all relevant information about a scan to be used in notifications.
type ScanSummaryData struct {
	ScanSessionID    string        // Unique identifier for the scan session (e.g., YYYYMMDD-HHMMSS timestamp)
	TargetSource     string        // The source of the targets (e.g., file path, "config_input_urls")
	ScanMode         string        // Mode of the scan (e.g., "onetime", "automated")
	Targets          []string      // List of original target URLs/identifiers
	TotalTargets     int           // Total number of targets processed or attempted
	ProbeStats       ProbeStats    // Statistics from the probing phase
	DiffStats        DiffStats     // Statistics from the diffing phase (New, Old, Existing)
	ScanDuration     time.Duration // Total duration of the scan
	ReportPath       string        // Filesystem path to the generated report (used by notifier to attach)
	Status           string        // Overall status: "COMPLETED", "FAILED", "STARTED", "INTERRUPTED", "PARTIAL_COMPLETE"
	ErrorMessages    []string      // Any critical errors encountered during the scan
	Component        string        // Component where an error might have occurred (for critical errors)
	RetriesAttempted int           // Number of retries, if applicable
}

// ProbeStats holds statistics related to the probing phase of a scan.
type ProbeStats struct {
	TotalProbed       int // Total URLs sent to the prober
	SuccessfulProbes  int // Number of probes that returned a successful response (e.g., 2xx)
	FailedProbes      int // Number of probes that failed or returned error codes
	DiscoverableItems int // e.g. number of items from httpx
}

// DiffStats holds statistics related to the diffing phase of a scan.
type DiffStats struct {
	New      int
	Old      int
	Existing int
	Changed  int // (If StatusChanged is implemented)
}

// FileChangeInfo holds information about a single file change for aggregation.
type FileChangeInfo struct {
	URL            string
	OldHash        string
	NewHash        string
	ContentType    string
	ChangeTime     time.Time       // Time the change was detected
	DiffReportPath *string         // Path to the generated HTML diff report for this specific change
	ExtractedPaths []ExtractedPath // Paths extracted from the content (for JS files)
}

// MonitorFetchErrorInfo holds information about an error encountered during file fetching or processing.
type MonitorFetchErrorInfo struct {
	URL        string    `json:"url"`
	Error      string    `json:"error"`  // Error message
	Source     string    `json:"source"` // e.g., "fetch", "process", "store_history"
	OccurredAt time.Time `json:"occurred_at"`
}

// ScanStatus defines the possible states of a scan.
type ScanStatus string

const (
	ScanStatusStarted             ScanStatus = "STARTED"
	ScanStatusCompleted           ScanStatus = "COMPLETED"
	ScanStatusFailed              ScanStatus = "FAILED"
	ScanStatusCriticalError       ScanStatus = "CRITICAL_ERROR"
	ScanStatusPartialComplete     ScanStatus = "PARTIAL_COMPLETE"
	ScanStatusInterrupted         ScanStatus = "INTERRUPTED"
	ScanStatusUnknown             ScanStatus = "UNKNOWN"
	ScanStatusNoTargets           ScanStatus = "NO_TARGETS"
	ScanStatusCompletedWithIssues ScanStatus = "COMPLETED_WITH_ISSUES"
)

// GetDefaultScanSummaryData initializes a ScanSummaryData with default/empty values.
func GetDefaultScanSummaryData() ScanSummaryData {
	return ScanSummaryData{
		ScanSessionID: "",
		TargetSource:  "Unknown",
		ScanMode:      "Unknown",
		Targets:       []string{},
		TotalTargets:  0,
		ProbeStats:    ProbeStats{},
		DiffStats:     DiffStats{},
		Status:        string(ScanStatusUnknown), // Default to unknown status
	}
}

// MonitorAggregatedStats holds aggregated statistics for monitor service notifications.
type MonitorAggregatedStats struct {
	TotalChanges      int // Total number of file changes
	TotalPaths        int // Total number of extracted paths
	TotalSecrets      int // Total number of secret findings
	HighSeverityCount int // Number of high/critical severity secrets
}

// MonitorCycleCompleteData holds data for the monitor service's end-of-cycle notification.
type MonitorCycleCompleteData struct {
	ChangedURLs    []string         // List of URLs that had changes detected during this cycle.
	FileChanges    []FileChangeInfo // Detailed information about file changes
	ReportPath     string           // Path to the aggregated HTML diff report for all monitored URLs.
	TotalMonitored int              // Total number of URLs being monitored in this cycle.
	Timestamp      time.Time        // Timestamp when the cycle completed.
	// Add any other summary fields you might want, e.g., total errors in cycle.
}
