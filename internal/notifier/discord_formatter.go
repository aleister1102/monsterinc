package notifier

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"monsterinc/internal/config"
	"monsterinc/internal/models"
)

const (
	maxTargetsInMessage  = 10
	maxDescriptionLength = 4096 // Discord embed description limit
	maxFieldValueLength  = 1024 // Discord embed field value limit
	maxFields            = 25   // Discord embed field limit

	// Standardized Color Scheme
	colorSuccess       = 0x2ECC71 // Green - for successful operations
	colorError         = 0xE74C3C // Red - for errors and failures
	colorInfo          = 0x3498DB // Blue - for informational messages
	colorWarning       = 0xF39C12 // Orange - for warnings and partial failures
	colorCritical      = 0x8E44AD // Purple - for critical alerts
	colorNeutral       = 0x95A5A6 // Grey - for neutral/unknown status
	colorSecurityAlert = 0xE91E63 // Pink - for security-related alerts

	// Legacy color constants (deprecated - use standardized colors above)
	colorGreen  = colorSuccess
	colorRed    = colorError
	colorBlue   = colorInfo
	colorOrange = colorWarning

	// Standardized Timestamp Formats
	timestampFormatDiscord  = time.RFC3339          // ISO 8601 format for Discord embed timestamps
	timestampFormatReadable = time.RFC1123          // Human-readable format for field values
	timestampFormatShort    = "2006-01-02 15:04:05" // Short format for compact display

	monsterIncIconURL         = "" // Placeholder icon - User should provide a publicly accessible URL to their favicon.ico or other desired avatar.
	monsterIncUsername        = "MonsterInc"
	defaultScanCompleteTitle  = ":white_check_mark: Scan Complete"
	defaultScanStartTitle     = ":rocket: Scan Started"
	defaultCriticalErrorTitle = ":x: Critical Error"

	// Standard field names for consistency across all Discord messages
	fieldNameSessionID     = ":id: Session ID"
	fieldNameStatus        = ":traffic_light: Status"
	fieldNameDuration      = ":stopwatch: Duration"
	fieldNameTargets       = ":dart: Targets"
	fieldNameProbeStats    = ":mag: Probe Statistics"
	fieldNameDiffStats     = ":arrows_counterclockwise: Diff Statistics"
	fieldNameRetries       = ":hourglass_flowing_sand: Retries"
	fieldNameErrors        = ":exclamation: Errors"
	fieldNameReport        = ":page_facing_up: Report"
	fieldNameSourceURL     = ":globe_with_meridians: Source URL"
	fieldNameDetectionRule = ":mag: Detection Rule"
	fieldNameSecurityInfo  = ":shield: Security Details"
	fieldNameSecretPreview = ":lock: Secret Preview (Masked)"
	fieldNameVerification  = ":white_check_mark: Verification"
	fieldNameErrorDetails  = ":warning: Error Details"

	// Standardized Footer Information
	footerTextScanning     = "MonsterInc Scanning Platform"
	footerTextMonitoring   = "MonsterInc File Monitor"
	footerTextSecrets      = "MonsterInc Secret Detection"
	footerTextSystemAlerts = "MonsterInc System Alert"
)

func buildMentions(roleIDs []string) string {
	if len(roleIDs) == 0 {
		return ""
	}
	var mentions []string
	for _, id := range roleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", id))
	}
	return strings.Join(mentions, " ") + "\n"
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}

func formatDuration(d time.Duration) string {
	return d.Round(time.Second).String()
}

// FormatScanStartMessage creates a Discord message payload for scan start events.
func FormatScanStartMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)
	messageContent := ""
	if mentions != "" {
		messageContent = mentions + "\n"
	}

	title := fmt.Sprintf(":rocket: Scan Started: %s", summary.TargetSource)
	description := fmt.Sprintf("**Session ID**: `%s`\n**Total Targets**: %d",
		summary.ScanSessionID,
		summary.TotalTargets)

	if len(summary.Targets) > 0 && len(summary.Targets) <= 5 { // Show a few targets if the list is small
		description += "\n**Sample Targets**:\n"
		for i, t := range summary.Targets {
			if i < 5 {
				description += fmt.Sprintf("- %s\n", truncateString(t, 100))
			}
		}
	} else if len(summary.Targets) > 5 {
		description += fmt.Sprintf("\n(And %d more targets...)", len(summary.Targets)-5)
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       colorInfo, // Blue for informational scan start messages
		Timestamp:   time.Now().Format(timestampFormatDiscord),
		Footer:      createStandardFooter(footerTextScanning),
	}

	return models.DiscordMessagePayload{
		Username:  monsterIncUsername,
		AvatarURL: monsterIncIconURL,
		Content:   messageContent,
		Embeds:    []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// FormatScanCompleteMessage creates a Discord message payload for scan completion events.
func FormatScanCompleteMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)
	messageContent := ""
	if mentions != "" {
		messageContent = mentions + "\n"
	}

	var title string
	var color int
	statusEmoji := ""

	switch models.ScanStatus(summary.Status) {
	case models.ScanStatusCompleted:
		title = fmt.Sprintf(":white_check_mark: Scan Completed: %s", summary.TargetSource)
		color = colorSuccess // Green for successful completion
		statusEmoji = ":white_check_mark:"
	case models.ScanStatusFailed:
		title = fmt.Sprintf(":x: Scan Failed: %s", summary.TargetSource)
		color = colorError // Red for failures
		statusEmoji = ":x:"
	case models.ScanStatusPartialComplete:
		title = fmt.Sprintf(":warning: Scan Partially Completed: %s", summary.TargetSource)
		color = colorWarning // Orange for partial completion
		statusEmoji = ":warning:"
	case models.ScanStatusInterrupted:
		title = fmt.Sprintf(":octagonal_sign: Scan Interrupted: %s", summary.TargetSource)
		color = colorNeutral // Grey for interruptions
		statusEmoji = ":octagonal_sign:"
	default:
		title = fmt.Sprintf(":question: Scan Status Unknown: %s", summary.TargetSource)
		color = colorNeutral // Grey for unknown status
		statusEmoji = ":question:"
	}

	description := fmt.Sprintf("**Session ID**: `%s`\n**Status**: %s %s\n**Duration**: %s\n**Total Targets**: %d",
		summary.ScanSessionID,
		statusEmoji,
		summary.Status,
		formatDuration(summary.ScanDuration),
		summary.TotalTargets)

	var fields []models.DiscordEmbedField

	if summary.ProbeStats.DiscoverableItems > 0 || summary.ProbeStats.SuccessfulProbes > 0 || summary.ProbeStats.FailedProbes > 0 {
		fields = append(fields, models.DiscordEmbedField{
			Name:   fieldNameProbeStats,
			Value:  fmt.Sprintf("Discoverable: %d | Successful: %d | Failed: %d", summary.ProbeStats.DiscoverableItems, summary.ProbeStats.SuccessfulProbes, summary.ProbeStats.FailedProbes),
			Inline: false,
		})
	}

	if summary.DiffStats.New > 0 || summary.DiffStats.Old > 0 || summary.DiffStats.Existing > 0 {
		fields = append(fields, models.DiscordEmbedField{
			Name:   fieldNameDiffStats,
			Value:  fmt.Sprintf("New: %d | Old: %d | Existing: %d", summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing),
			Inline: false,
		})
	}

	if summary.RetriesAttempted > 0 && (models.ScanStatus(summary.Status) == models.ScanStatusFailed || models.ScanStatus(summary.Status) == models.ScanStatusPartialComplete) {
		fields = append(fields, models.DiscordEmbedField{
			Name:   fieldNameRetries,
			Value:  fmt.Sprintf("This scan failed after %d retry attempt(s).", summary.RetriesAttempted),
			Inline: false,
		})
	}

	if len(summary.ErrorMessages) > 0 {
		errorMsg := ""
		for i, e := range summary.ErrorMessages {
			errorMsg += truncateString(e, 200) // Truncate individual errors
			if i < len(summary.ErrorMessages)-1 {
				errorMsg += "\n"
			}
		}
		fields = append(fields, models.DiscordEmbedField{
			Name:   fieldNameErrors,
			Value:  truncateString(errorMsg, 1000), // Truncate overall error message block
			Inline: false,
		})
	}

	if summary.ReportPath != "" {
		// The actual file attachment is handled by DiscordNotifier.SendNotification
		// This field just informs the user that a report is available.
		fields = append(fields, models.DiscordEmbedField{
			Name:   fieldNameReport,
			Value:  "Report attached.", // Or provide a link if it's uploaded elsewhere
			Inline: false,
		})
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Timestamp:   time.Now().Format(timestampFormatDiscord),
		Fields:      fields,
		Footer:      createStandardFooter(footerTextScanning),
	}

	// Add Target URLs field if available
	if len(summary.Targets) > 0 {
		var targetURLsString strings.Builder
		maxTargetsToShow := 10 // Show up to 10 URLs directly in the message
		for i, target := range summary.Targets {
			if i >= maxTargetsToShow {
				targetURLsString.WriteString(fmt.Sprintf("\n... and %d more.", len(summary.Targets)-maxTargetsToShow))
				break
			}
			targetURLsString.WriteString(fmt.Sprintf("- %s\n", truncateString(target, 100)))
		}

		embed.Fields = append(embed.Fields, models.DiscordEmbedField{
			Name:   fieldNameTargets,
			Value:  targetURLsString.String(),
			Inline: false,
		})
	} else if summary.TargetSource != "" {
		// Fallback to TargetSource if Targets list is empty but source is known
		embed.Fields = append(embed.Fields, models.DiscordEmbedField{
			Name:   fieldNameTargets,
			Value:  fmt.Sprintf("`%s` (No individual URLs listed in summary)", summary.TargetSource),
			Inline: false,
		})
	}

	payload := models.DiscordMessagePayload{
		Username:  monsterIncUsername,
		AvatarURL: monsterIncIconURL,
		Content:   messageContent,
		Embeds:    []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}

	return payload
}

// FormatCriticalErrorMessage creates a Discord message payload for critical errors.
func FormatCriticalErrorMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)
	messageContent := ""
	if mentions != "" {
		messageContent = mentions + "\n"
	}

	title := ":bangbang: Critical Application Error"
	if summary.Component != "" {
		title = fmt.Sprintf(":bangbang: Critical Error in %s", summary.Component)
	}

	description := "A critical error occurred that may have prevented the application from functioning correctly."
	if summary.ScanSessionID != "" {
		description += fmt.Sprintf("\n**Context/Session ID**: `%s`", summary.ScanSessionID)
	}
	if summary.TargetSource != "" {
		description += fmt.Sprintf("\n**Target Source Context**: `%s`", summary.TargetSource)
	}

	var fields []models.DiscordEmbedField
	if len(summary.ErrorMessages) > 0 {
		errorMsg := ""
		for i, e := range summary.ErrorMessages {
			errorMsg += truncateString(e, 200) // Truncate individual errors
			if i < len(summary.ErrorMessages)-1 {
				errorMsg += "\n"
			}
		}
		fields = append(fields, models.DiscordEmbedField{
			Name:   fieldNameErrorDetails,
			Value:  truncateString(errorMsg, 1000), // Truncate overall error message block
			Inline: false,
		})
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       colorCritical, // Purple for critical system alerts
		Timestamp:   time.Now().Format(timestampFormatDiscord),
		Fields:      fields,
		Footer:      createStandardFooter(footerTextSystemAlerts),
	}

	return models.DiscordMessagePayload{
		Username:  monsterIncUsername,
		AvatarURL: monsterIncIconURL,
		Content:   messageContent,
		Embeds:    []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles", "users", "everyone"}, // Be more liberal with critical errors
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// FormatInitialMonitoredURLsMessage creates a Discord message payload for initial monitored URLs.
func FormatInitialMonitoredURLsMessage(monitoredURLs []string, cfg config.NotificationConfig) models.DiscordMessagePayload {
	allowedMentions := models.AllowedMentions{
		Parse: []string{"roles"}, // Mention roles if specified in config
		Roles: cfg.MentionRoleIDs,
	}

	title := ":pencil: Initial File Monitoring Started"
	description := fmt.Sprintf("**Total URLs**: %d now being monitored for changes", len(monitoredURLs))

	var fields []models.DiscordEmbedField

	// Add targets field using standardized field name
	var targetURLsString strings.Builder
	maxURLsToShow := 10 // Show up to 10 URLs directly in the message
	for i, url := range monitoredURLs {
		if i >= maxURLsToShow {
			targetURLsString.WriteString(fmt.Sprintf("\n... and %d more URLs.", len(monitoredURLs)-maxURLsToShow))
			break
		}
		targetURLsString.WriteString(fmt.Sprintf("• %s\n", truncateString(url, 150)))
	}

	fields = append(fields, models.DiscordEmbedField{
		Name:   fieldNameTargets,
		Value:  targetURLsString.String(),
		Inline: false,
	})

	// Add status field
	fields = append(fields, models.DiscordEmbedField{
		Name:   fieldNameStatus,
		Value:  ":white_check_mark: Active Monitoring",
		Inline: true,
	})

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       colorInfo, // Blue
		Timestamp:   time.Now().Format(timestampFormatDiscord),
		Fields:      fields,
		Footer:      createStandardFooter(footerTextMonitoring),
	}

	return models.DiscordMessagePayload{
		Username:        monsterIncUsername,
		AvatarURL:       monsterIncIconURL,
		Embeds:          []models.DiscordEmbed{embed},
		AllowedMentions: &allowedMentions,
	}
}

// FormatAggregatedFileChangesMessage creates a Discord message payload for aggregated file changes.
func FormatAggregatedFileChangesMessage(changes []models.FileChangeInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	if len(changes) == 0 {
		return models.DiscordMessagePayload{}
	}

	allowedMentions := models.AllowedMentions{
		Parse: []string{"roles"},
		Roles: cfg.MentionRoleIDs,
	}

	title := fmt.Sprintf(":white_check_mark: %d File Change(s) Detected", len(changes))
	description := fmt.Sprintf("**Total Changes**: %d", len(changes))

	var fields []models.DiscordEmbedField

	// Add summary field if multiple changes
	if len(changes) > 1 {
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":chart_with_upwards_trend: Summary",
			Value:  fmt.Sprintf("Found **%d** file changes across monitored URLs", len(changes)),
			Inline: false,
		})
	}

	// Add individual change fields (limit to prevent Discord embed limits)
	maxChangesToShow := 5
	for i, change := range changes {
		if i >= maxChangesToShow {
			fields = append(fields, models.DiscordEmbedField{
				Name:   ":page_facing_up: Additional Changes",
				Value:  fmt.Sprintf("... and **%d** more changes not shown here", len(changes)-maxChangesToShow),
				Inline: false,
			})
			break
		}

		changeTitle := fmt.Sprintf(":file_folder: Change #%d", i+1)
		changeValue := fmt.Sprintf("**URL**: %s\n**Content Type**: `%s`\n**Time**: %s",
			truncateString(change.URL, 150),
			change.ContentType,
			change.ChangeTime.Format(timestampFormatReadable))

		if change.NewHash != "" && change.OldHash != "" {
			changeValue += fmt.Sprintf("\n**Hash Change**: `%s` → `%s`",
				truncateString(change.OldHash, 8),
				truncateString(change.NewHash, 8))
		}

		if change.DiffReportPath != nil && *change.DiffReportPath != "" {
			baseName := filepath.Base(*change.DiffReportPath)
			changeValue += fmt.Sprintf("\n**Report**: `%s`", baseName)
		}

		fields = append(fields, models.DiscordEmbedField{
			Name:   changeTitle,
			Value:  changeValue,
			Inline: false,
		})
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       colorSuccess, // Green
		Timestamp:   time.Now().Format(timestampFormatDiscord),
		Fields:      fields,
		Footer:      createStandardFooter(footerTextMonitoring),
	}

	return models.DiscordMessagePayload{
		Username:        monsterIncUsername,
		AvatarURL:       monsterIncIconURL,
		Embeds:          []models.DiscordEmbed{embed},
		AllowedMentions: &allowedMentions,
	}
}

// FormatAggregatedMonitorErrorsMessage creates a Discord message payload for aggregated monitor errors.
func FormatAggregatedMonitorErrorsMessage(errors []models.MonitorFetchErrorInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	allowedMentions := models.AllowedMentions{
		Parse: []string{"roles"}, // Mention roles if specified in config
		Roles: cfg.MentionRoleIDs,
	}

	title := fmt.Sprintf(":x: %d Monitor Error(s) Detected", len(errors))
	description := fmt.Sprintf("**Total Errors**: %d during file monitoring operations", len(errors))

	var fields []models.DiscordEmbedField

	// Add summary field for multiple errors
	if len(errors) > 1 {
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":warning: Error Summary",
			Value:  fmt.Sprintf("Found **%d** errors across monitored operations", len(errors)),
			Inline: false,
		})
	}

	// Add individual error fields (limit to prevent Discord embed limits)
	maxErrorsToShow := 5
	for i, errInfo := range errors {
		if i >= maxErrorsToShow {
			fields = append(fields, models.DiscordEmbedField{
				Name:   ":page_facing_up: Additional Errors",
				Value:  fmt.Sprintf("... and **%d** more errors not shown here", len(errors)-maxErrorsToShow),
				Inline: false,
			})
			break
		}

		errorTitle := fmt.Sprintf(":exclamation: Error #%d", i+1)
		errorValue := fmt.Sprintf("**URL**: %s\n**Source**: `%s`\n**Time**: %s\n**Error**: %s",
			truncateString(errInfo.URL, 150),
			errInfo.Source,
			errInfo.OccurredAt.Format(timestampFormatReadable),
			truncateString(errInfo.Error, 300))

		fields = append(fields, models.DiscordEmbedField{
			Name:   errorTitle,
			Value:  errorValue,
			Inline: false,
		})
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       colorWarning, // Orange for warning/multiple errors
		Timestamp:   time.Now().Format(timestampFormatDiscord),
		Fields:      fields,
		Footer:      createStandardFooter(footerTextMonitoring),
	}

	return models.DiscordMessagePayload{
		Username:        monsterIncUsername,
		AvatarURL:       monsterIncIconURL,
		Content:         buildMentions(cfg.MentionRoleIDs),
		Embeds:          []models.DiscordEmbed{embed},
		AllowedMentions: &allowedMentions,
	}
}

// FormatHighSeveritySecretNotification creates a Discord message payload for high-severity secret findings.
func FormatHighSeveritySecretNotification(finding models.SecretFinding, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)
	messageContent := ""
	if mentions != "" {
		messageContent = mentions + "\n"
	}

	title := ":warning: High-Severity Secret Detected"
	if finding.Severity == "CRITICAL" {
		title = ":bangbang: Critical Secret Detected"
	}

	description := fmt.Sprintf("A %s severity secret has been detected during content scanning.", strings.ToLower(finding.Severity))

	var fields []models.DiscordEmbedField

	// Source URL field
	fields = append(fields, models.DiscordEmbedField{
		Name:   fieldNameSourceURL,
		Value:  truncateString(finding.SourceURL, 1000),
		Inline: false,
	})

	// Rule and Description
	fields = append(fields, models.DiscordEmbedField{
		Name:   fieldNameDetectionRule,
		Value:  fmt.Sprintf("**Rule ID**: `%s`\n**Description**: %s", finding.RuleID, truncateString(finding.Description, 800)),
		Inline: false,
	})

	// Severity and Tool
	toolInfo := "Unknown"
	if finding.ToolName != "" {
		toolInfo = finding.ToolName
	}
	fields = append(fields, models.DiscordEmbedField{
		Name:   fieldNameSecurityInfo,
		Value:  fmt.Sprintf("**Severity**: `%s`\n**Tool**: `%s`\n**Line**: %d", finding.Severity, toolInfo, finding.LineNumber),
		Inline: true,
	})

	// Masked secret preview
	maskedSecret := "***"
	if finding.SecretText != "" {
		if len(finding.SecretText) > 10 {
			maskedSecret = finding.SecretText[:3] + "***" + finding.SecretText[len(finding.SecretText)-3:]
		} else if len(finding.SecretText) > 2 {
			maskedSecret = "***" + finding.SecretText[len(finding.SecretText)-2:]
		}
	}
	fields = append(fields, models.DiscordEmbedField{
		Name:   fieldNameSecretPreview,
		Value:  fmt.Sprintf("`%s`", maskedSecret),
		Inline: true,
	})

	// Verification state if available
	if finding.VerificationState != "" {
		fields = append(fields, models.DiscordEmbedField{
			Name:   fieldNameVerification,
			Value:  fmt.Sprintf("`%s`", finding.VerificationState),
			Inline: true,
		})
	}

	// Color based on severity - use standardized colors
	embedColor := colorSecurityAlert // Pink for security alerts
	if finding.Severity == "CRITICAL" {
		embedColor = colorCritical // Purple for critical alerts
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       embedColor,
		Timestamp:   finding.Timestamp.Format(timestampFormatDiscord),
		Fields:      fields,
		Footer:      createStandardFooter(footerTextSecrets),
	}

	return models.DiscordMessagePayload{
		Username:  monsterIncUsername,
		AvatarURL: monsterIncIconURL,
		Content:   messageContent,
		Embeds:    []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// createStandardFooter creates a standardized footer with optional version info
func createStandardFooter(baseText string) *models.DiscordEmbedFooter {
	return &models.DiscordEmbedFooter{
		Text: baseText,
		// IconURL: monsterIncIconURL, // Uncomment if you want footer icon
	}
}

// createStatusField creates a standardized status field with emoji and text
func createStatusField(status string, emoji string, inline bool) models.DiscordEmbedField {
	return models.DiscordEmbedField{
		Name:   fieldNameStatus,
		Value:  fmt.Sprintf("%s %s", emoji, status),
		Inline: inline,
	}
}

// createURLField creates a standardized URL field with truncation
func createURLField(fieldName, url string, maxLength int, inline bool) models.DiscordEmbedField {
	return models.DiscordEmbedField{
		Name:   fieldName,
		Value:  truncateString(url, maxLength),
		Inline: inline,
	}
}

// createErrorField creates a standardized error field with proper formatting
func createErrorField(errors []string, maxErrorsToShow int) models.DiscordEmbedField {
	if len(errors) == 0 {
		return models.DiscordEmbedField{}
	}

	var errorMsg strings.Builder
	for i, e := range errors {
		if i >= maxErrorsToShow {
			errorMsg.WriteString(fmt.Sprintf("\n... and %d more errors", len(errors)-maxErrorsToShow))
			break
		}
		if i > 0 {
			errorMsg.WriteString("\n")
		}
		errorMsg.WriteString(fmt.Sprintf("• %s", truncateString(e, 200)))
	}

	return models.DiscordEmbedField{
		Name:   fieldNameErrors,
		Value:  truncateString(errorMsg.String(), maxFieldValueLength),
		Inline: false,
	}
}

// createTimestampField creates a standardized timestamp field
func createTimestampField(fieldName string, timestamp time.Time, inline bool) models.DiscordEmbedField {
	return models.DiscordEmbedField{
		Name:   fieldName,
		Value:  timestamp.Format(timestampFormatReadable),
		Inline: inline,
	}
}

// createCountField creates a standardized count field with formatting
func createCountField(fieldName string, count int, unit string, inline bool) models.DiscordEmbedField {
	value := fmt.Sprintf("**%d** %s", count, unit)
	return models.DiscordEmbedField{
		Name:   fieldName,
		Value:  value,
		Inline: inline,
	}
}

// createStandardWebhookPayload creates a standardized Discord webhook payload with consistent username and avatar
func createStandardWebhookPayload(embed models.DiscordEmbed, cfg config.NotificationConfig, content string) models.DiscordMessagePayload {
	return models.DiscordMessagePayload{
		Username:  monsterIncUsername,
		AvatarURL: monsterIncIconURL,
		Content:   content,
		Embeds:    []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// createWebhookPayloadWithMentions creates a webhook payload with mentions built from config
func createWebhookPayloadWithMentions(embed models.DiscordEmbed, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)
	content := ""
	if mentions != "" {
		content = mentions + "\n"
	}
	return createStandardWebhookPayload(embed, cfg, content)
}
