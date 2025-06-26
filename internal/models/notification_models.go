package models

import (
	"fmt"
	"time"
)

// Discord color constants for different types of notifications
const (
	DiscordColorSuccess   = 0x00ff00 // Green
	DiscordColorError     = 0xff0000 // Red
	DiscordColorWarning   = 0xffa500 // Orange
	DiscordColorInfo      = 0x0099ff // Blue
	DiscordColorDefault   = 0x36393f // Discord default gray
	DiscordColorCritical  = 0x8b0000 // Dark red
	DiscordColorCompleted = 0x228b22 // Forest green
)

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

// AllowedMentionsBuilder builds AllowedMentions objects
type AllowedMentionsBuilder struct {
	mentions AllowedMentions
}

// NewAllowedMentionsBuilder creates a new allowed mentions builder
func NewAllowedMentionsBuilder() *AllowedMentionsBuilder {
	return &AllowedMentionsBuilder{
		mentions: AllowedMentions{},
	}
}

// WithParse sets parse types
func (amb *AllowedMentionsBuilder) WithParse(parse []string) *AllowedMentionsBuilder {
	amb.mentions.Parse = make([]string, len(parse))
	copy(amb.mentions.Parse, parse)
	return amb
}

// WithRoles sets roles to mention
func (amb *AllowedMentionsBuilder) WithRoles(roles []string) *AllowedMentionsBuilder {
	amb.mentions.Roles = make([]string, len(roles))
	copy(amb.mentions.Roles, roles)
	return amb
}

// WithUsers sets users to mention
func (amb *AllowedMentionsBuilder) WithUsers(users []string) *AllowedMentionsBuilder {
	amb.mentions.Users = make([]string, len(users))
	copy(amb.mentions.Users, users)
	return amb
}

// WithRepliedUser sets replied user mention
func (amb *AllowedMentionsBuilder) WithRepliedUser(replied bool) *AllowedMentionsBuilder {
	amb.mentions.RepliedUser = replied
	return amb
}

// Build returns the constructed AllowedMentions
func (amb *AllowedMentionsBuilder) Build() AllowedMentions {
	return amb.mentions
}

// DiscordEmbedField represents a field in an embed.
type DiscordEmbedField struct {
	Name   string `json:"name"`             // Name of the field
	Value  string `json:"value"`            // Value of the field
	Inline bool   `json:"inline,omitempty"` // Whether or not this field should display inline
}

// NewDiscordEmbedField creates a new Discord embed field
func NewDiscordEmbedField(name, value string, inline bool) DiscordEmbedField {
	return DiscordEmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
}

// DiscordEmbedFooter represents the footer of an embed.
type DiscordEmbedFooter struct {
	Text    string `json:"text"`               // Footer text
	IconURL string `json:"icon_url,omitempty"` // URL of footer icon (only supports http(s) and attachments)
}

// NewDiscordEmbedFooter creates a new Discord embed footer
func NewDiscordEmbedFooter(text, iconURL string) *DiscordEmbedFooter {
	return &DiscordEmbedFooter{
		Text:    text,
		IconURL: iconURL,
	}
}

// DiscordEmbedImage represents the image of an embed.
type DiscordEmbedImage struct {
	URL string `json:"url"` // Source URL of image (only supports http(s) and attachments)
}

// NewDiscordEmbedImage creates a new Discord embed image
func NewDiscordEmbedImage(url string) *DiscordEmbedImage {
	return &DiscordEmbedImage{URL: url}
}

// DiscordEmbedThumbnail represents the thumbnail of an embed.
type DiscordEmbedThumbnail struct {
	URL string `json:"url"` // Source URL of thumbnail (only supports http(s) and attachments)
}

// NewDiscordEmbedThumbnail creates a new Discord embed thumbnail
func NewDiscordEmbedThumbnail(url string) *DiscordEmbedThumbnail {
	return &DiscordEmbedThumbnail{URL: url}
}

// DiscordEmbedAuthor represents the author of an embed.
type DiscordEmbedAuthor struct {
	Name    string `json:"name"`               // Name of author
	URL     string `json:"url,omitempty"`      // URL of author (only supports http(s))
	IconURL string `json:"icon_url,omitempty"` // URL of author icon (only supports http(s) and attachments)
}

// NewDiscordEmbedAuthor creates a new Discord embed author
func NewDiscordEmbedAuthor(name, url, iconURL string) *DiscordEmbedAuthor {
	return &DiscordEmbedAuthor{
		Name:    name,
		URL:     url,
		IconURL: iconURL,
	}
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

// DiscordEmbedValidator validates Discord embed objects
type DiscordEmbedValidator struct{}

// NewDiscordEmbedValidator creates a new embed validator
func NewDiscordEmbedValidator() *DiscordEmbedValidator {
	return &DiscordEmbedValidator{}
}

// ValidateEmbed validates a Discord embed
func (dev *DiscordEmbedValidator) ValidateEmbed(embed DiscordEmbed) error {
	if len(embed.Title) > 256 {
		return NewValidationError("title", embed.Title, "title cannot exceed 256 characters")
	}

	if len(embed.Description) > 4096 {
		return NewValidationError("description", embed.Description, "description cannot exceed 4096 characters")
	}

	if len(embed.Fields) > 25 {
		return NewValidationError("fields", embed.Fields, "cannot have more than 25 fields")
	}

	// Validate fields
	for i, field := range embed.Fields {
		if len(field.Name) > 256 {
			return NewValidationError("field_name", field.Name, fmt.Sprintf("field %d name cannot exceed 256 characters", i))
		}
		if len(field.Value) > 1024 {
			return NewValidationError("field_value", field.Value, fmt.Sprintf("field %d value cannot exceed 1024 characters", i))
		}
		if field.Name == "" {
			return NewValidationError("field_name", field.Name, fmt.Sprintf("field %d name cannot be empty", i))
		}
		if field.Value == "" {
			return NewValidationError("field_value", field.Value, fmt.Sprintf("field %d value cannot be empty", i))
		}
	}

	if embed.Footer != nil && len(embed.Footer.Text) > 2048 {
		return NewValidationError("footer_text", embed.Footer.Text, "footer text cannot exceed 2048 characters")
	}

	if embed.Author != nil && len(embed.Author.Name) > 256 {
		return NewValidationError("author_name", embed.Author.Name, "author name cannot exceed 256 characters")
	}

	return nil
}

// DiscordEmbedBuilder builds Discord embed objects
type DiscordEmbedBuilder struct {
	embed     DiscordEmbed
	validator *DiscordEmbedValidator
}

// NewDiscordEmbedBuilder creates a new Discord embed builder
func NewDiscordEmbedBuilder() *DiscordEmbedBuilder {
	return &DiscordEmbedBuilder{
		embed:     DiscordEmbed{},
		validator: NewDiscordEmbedValidator(),
	}
}

// WithTitle sets the embed title
func (deb *DiscordEmbedBuilder) WithTitle(title string) *DiscordEmbedBuilder {
	deb.embed.Title = title
	return deb
}

// WithDescription sets the embed description
func (deb *DiscordEmbedBuilder) WithDescription(description string) *DiscordEmbedBuilder {
	deb.embed.Description = description
	return deb
}

// WithURL sets the embed URL
func (deb *DiscordEmbedBuilder) WithURL(url string) *DiscordEmbedBuilder {
	deb.embed.URL = url
	return deb
}

// WithTimestamp sets the embed timestamp
func (deb *DiscordEmbedBuilder) WithTimestamp(timestamp time.Time) *DiscordEmbedBuilder {
	deb.embed.Timestamp = timestamp.Format(time.RFC3339)
	return deb
}

// WithCurrentTimestamp sets the embed timestamp to current time
func (deb *DiscordEmbedBuilder) WithCurrentTimestamp() *DiscordEmbedBuilder {
	return deb.WithTimestamp(time.Now())
}

// WithColor sets the embed color
func (deb *DiscordEmbedBuilder) WithColor(color int) *DiscordEmbedBuilder {
	deb.embed.Color = color
	return deb
}

// WithSuccessColor sets success color
func (deb *DiscordEmbedBuilder) WithSuccessColor() *DiscordEmbedBuilder {
	return deb.WithColor(DiscordColorSuccess)
}

// WithErrorColor sets error color
func (deb *DiscordEmbedBuilder) WithErrorColor() *DiscordEmbedBuilder {
	return deb.WithColor(DiscordColorError)
}

// WithWarningColor sets warning color
func (deb *DiscordEmbedBuilder) WithWarningColor() *DiscordEmbedBuilder {
	return deb.WithColor(DiscordColorWarning)
}

// WithInfoColor sets info color
func (deb *DiscordEmbedBuilder) WithInfoColor() *DiscordEmbedBuilder {
	return deb.WithColor(DiscordColorInfo)
}

// WithFooter sets the embed footer
func (deb *DiscordEmbedBuilder) WithFooter(text, iconURL string) *DiscordEmbedBuilder {
	deb.embed.Footer = NewDiscordEmbedFooter(text, iconURL)
	return deb
}

// WithImage sets the embed image
func (deb *DiscordEmbedBuilder) WithImage(url string) *DiscordEmbedBuilder {
	deb.embed.Image = NewDiscordEmbedImage(url)
	return deb
}

// WithThumbnail sets the embed thumbnail
func (deb *DiscordEmbedBuilder) WithThumbnail(url string) *DiscordEmbedBuilder {
	deb.embed.Thumbnail = NewDiscordEmbedThumbnail(url)
	return deb
}

// WithAuthor sets the embed author
func (deb *DiscordEmbedBuilder) WithAuthor(name, url, iconURL string) *DiscordEmbedBuilder {
	deb.embed.Author = NewDiscordEmbedAuthor(name, url, iconURL)
	return deb
}

// AddField adds a field to the embed
func (deb *DiscordEmbedBuilder) AddField(name, value string, inline bool) *DiscordEmbedBuilder {
	field := NewDiscordEmbedField(name, value, inline)
	deb.embed.Fields = append(deb.embed.Fields, field)
	return deb
}

// AddFields adds multiple fields to the embed
func (deb *DiscordEmbedBuilder) AddFields(fields []DiscordEmbedField) *DiscordEmbedBuilder {
	deb.embed.Fields = append(deb.embed.Fields, fields...)
	return deb
}

// ClearFields clears all fields
func (deb *DiscordEmbedBuilder) ClearFields() *DiscordEmbedBuilder {
	deb.embed.Fields = []DiscordEmbedField{}
	return deb
}

// Validate validates the current embed
func (deb *DiscordEmbedBuilder) Validate() error {
	return deb.validator.ValidateEmbed(deb.embed)
}

// Build builds the Discord embed with validation
func (deb *DiscordEmbedBuilder) Build() (DiscordEmbed, error) {
	if err := deb.Validate(); err != nil {
		return DiscordEmbed{}, WrapError(err, "embed validation failed")
	}
	return deb.embed, nil
}

// BuildUnsafe builds the Discord embed without validation
func (deb *DiscordEmbedBuilder) BuildUnsafe() DiscordEmbed {
	return deb.embed
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
	CycleMinutes     int           // Cycle interval in minutes (only for automated mode)
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
	CycleID        string          // Unique identifier for the monitoring cycle
	BatchInfo      *BatchInfo      `json:"batch_info,omitempty"` // Information about the batch this change belongs to
}

// MonitorFetchErrorInfo holds information about an error encountered during file fetching or processing.
type MonitorFetchErrorInfo struct {
	URL        string     `json:"url"`
	Error      string     `json:"error"`  // Error message
	Source     string     `json:"source"` // e.g., "fetch", "process", "store_history"
	OccurredAt time.Time  `json:"occurred_at"`
	CycleID    string     `json:"cycle_id"`             // Unique identifier for the monitoring cycle
	BatchInfo  *BatchInfo `json:"batch_info,omitempty"` // Information about the batch this error occurred in
}

// BatchInfo holds information about a monitoring batch
type BatchInfo struct {
	BatchNumber      int `json:"batch_number"`       // Current batch number (1-based)
	TotalBatches     int `json:"total_batches"`      // Total number of batches in the cycle
	BatchSize        int `json:"batch_size"`         // Number of URLs in this batch
	ProcessedInBatch int `json:"processed_in_batch"` // Number of URLs processed in this batch so far
}

// NewBatchInfo creates a new BatchInfo instance
func NewBatchInfo(batchNumber, totalBatches, batchSize, processedInBatch int) *BatchInfo {
	return &BatchInfo{
		BatchNumber:      batchNumber,
		TotalBatches:     totalBatches,
		BatchSize:        batchSize,
		ProcessedInBatch: processedInBatch,
	}
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

// IsSuccess checks if scan status indicates success
func (ss ScanStatus) IsSuccess() bool {
	return ss == ScanStatusCompleted
}

// IsFailure checks if scan status indicates failure
func (ss ScanStatus) IsFailure() bool {
	return ss == ScanStatusFailed || ss == ScanStatusCriticalError
}

// IsInProgress checks if scan status indicates in progress
func (ss ScanStatus) IsInProgress() bool {
	return ss == ScanStatusStarted
}

// GetColor returns appropriate Discord color for the status
func (ss ScanStatus) GetColor() int {
	switch ss {
	case ScanStatusCompleted:
		return DiscordColorSuccess
	case ScanStatusFailed, ScanStatusCriticalError:
		return DiscordColorError
	case ScanStatusPartialComplete, ScanStatusCompletedWithIssues:
		return DiscordColorWarning
	case ScanStatusStarted:
		return DiscordColorInfo
	default:
		return DiscordColorDefault
	}
}

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
	TotalChanges int // Total number of file changes
	TotalPaths   int // Total number of extracted paths
}

// MonitorCycleCompleteData holds data for the monitor service's end-of-cycle notification.
type MonitorCycleCompleteData struct {
	CycleID        string           // Unique identifier for the monitoring cycle
	ChangedURLs    []string         // List of URLs that had changes detected during this cycle.
	FileChanges    []FileChangeInfo // Detailed information about file changes
	ReportPaths    []string         // Paths to the aggregated HTML diff reports for all monitored URLs.
	TotalMonitored int              // Total number of URLs being monitored in this cycle.
	Timestamp      time.Time        // Timestamp when the cycle completed.
	BatchStats     *BatchStats      `json:"batch_stats,omitempty"` // Statistics about batch processing for this cycle
	// Add any other summary fields you might want, e.g., total errors in cycle.
}

// BatchStats holds statistics about batch processing for a monitoring cycle
type BatchStats struct {
	UsedBatching       bool `json:"used_batching"`        // Whether batching was used for this cycle
	TotalBatches       int  `json:"total_batches"`        // Total number of batches processed
	CompletedBatches   int  `json:"completed_batches"`    // Number of batches that completed successfully
	AvgBatchSize       int  `json:"avg_batch_size"`       // Average number of URLs per batch
	MaxBatchSize       int  `json:"max_batch_size"`       // Maximum batch size configured
	TotalURLsProcessed int  `json:"total_urls_processed"` // Total number of URLs processed across all batches
}

// NewBatchStats creates a new BatchStats instance
func NewBatchStats(usedBatching bool, totalBatches, completedBatches, avgBatchSize, maxBatchSize, totalURLsProcessed int) *BatchStats {
	return &BatchStats{
		UsedBatching:       usedBatching,
		TotalBatches:       totalBatches,
		CompletedBatches:   completedBatches,
		AvgBatchSize:       avgBatchSize,
		MaxBatchSize:       maxBatchSize,
		TotalURLsProcessed: totalURLsProcessed,
	}
}

// MonitorStartData holds data for the monitor service's start notification.
type MonitorStartData struct {
	CycleID       string    // Unique identifier for the monitoring cycle
	TotalTargets  int       // Total number of URLs to be monitored
	TargetSource  string    // Source of the targets (e.g., file path)
	Timestamp     time.Time // Timestamp when monitoring started
	Mode          string    // Mode of the monitoring (e.g., "automated")
	CycleInterval int       // Interval between cycles in minutes
}

// MonitorInterruptData holds data for the monitor service's interrupt notification.
type MonitorInterruptData struct {
	CycleID          string    // Unique identifier for the monitoring cycle
	TotalTargets     int       // Total number of URLs being monitored
	ProcessedTargets int       // Number of URLs processed before interruption
	Timestamp        time.Time // Timestamp when monitoring was interrupted
	Reason           string    // Reason for interruption (e.g., "user_signal", "context_canceled")
	LastActivity     string    // Description of last activity before interruption
}

// MonitorErrorData holds data for the monitor service's error notification.
type MonitorErrorData struct {
	CycleID      string    // Unique identifier for the monitoring cycle
	TotalTargets int       // Total number of URLs being monitored
	Timestamp    time.Time // Timestamp when error occurred
	ErrorType    string    // Type of error (e.g., "batch_processing", "initialization", "runtime")
	ErrorMessage string    // Detailed error message
	Component    string    // Component where error occurred
	Recoverable  bool      // Whether the error is recoverable
}

// MonitorStatus defines the possible states of a monitor service.
type MonitorStatus string

const (
	MonitorStatusStarted     MonitorStatus = "STARTED"
	MonitorStatusRunning     MonitorStatus = "RUNNING"
	MonitorStatusCompleted   MonitorStatus = "COMPLETED"
	MonitorStatusInterrupted MonitorStatus = "INTERRUPTED"
	MonitorStatusError       MonitorStatus = "ERROR"
	MonitorStatusStopped     MonitorStatus = "STOPPED"
)

// GetColor returns appropriate Discord color for the monitor status
func (ms MonitorStatus) GetColor() int {
	switch ms {
	case MonitorStatusStarted, MonitorStatusRunning:
		return DiscordColorInfo
	case MonitorStatusCompleted:
		return DiscordColorSuccess
	case MonitorStatusInterrupted, MonitorStatusStopped:
		return DiscordColorWarning
	case MonitorStatusError:
		return DiscordColorError
	default:
		return DiscordColorDefault
	}
}
