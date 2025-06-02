package notifier

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
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
	fieldNameVerification  = ":white_check_mark: Verification"
	fieldNameErrorDetails  = ":warning: Error Details"

	// Standardized Footer Information
	footerTextScanning     = "MonsterInc Scanning Platform"
	footerTextMonitoring   = "MonsterInc File Monitor"
	footerTextSystemAlerts = "MonsterInc System Alert"

	maxEmbedFields                       = 25
	maxTargetsToShowInDiscordSummary     = 5
	maxFileChangesToShowInDiscordSummary = 3
	maxItemsPerSingleField               = 10
	maxTotalURLLengthInField             = 900
	defaultMaxErrorsToShow               = 5
	defaultMaxTargetsToShow              = 10
	maxFilesToShowInReportField          = 5

	DefaultEmbedColor = 0x5865F2 // Discord Blurple
	DiscordUsername   = "MonsterInc Watcher"
	DiscordAvatarURL  = "https://i.imgur.com/kIz6rU5.png"
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

	if summary.ScanMode != "" && summary.ScanMode != "Unknown" {
		description += fmt.Sprintf("\n**Mode**: `%s`", summary.ScanMode)
	}

	if len(summary.Targets) > 0 {
		description += "\n**Sample Targets**:\n"
		shownTargets := 0
		for _, t := range summary.Targets {
			if t == "" { // Skip empty target lines
				continue
			}
			if shownTargets < 5 {
				description += fmt.Sprintf("- %s\n", truncateString(t, 100))
				shownTargets++
			} else {
				description += fmt.Sprintf("(And %d more targets...)", len(summary.Targets)-shownTargets)
				break
			}
		}
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

	if summary.ScanMode != "" && summary.ScanMode != "Unknown" {
		description += fmt.Sprintf("\n**Mode**: `%s`", summary.ScanMode)
	}

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
		shownTargets := 0
		validTargetsCount := 0
		for _, target := range summary.Targets {
			if target == "" { // Skip empty target lines
				continue
			}
			validTargetsCount++
			if shownTargets >= maxTargetsToShow {
				targetURLsString.WriteString(fmt.Sprintf("\n... and %d more.", validTargetsCount-shownTargets))
				break
			}
			targetURLsString.WriteString(fmt.Sprintf("- %s\n", truncateString(target, 100)))
			shownTargets++
		}

		if targetURLsString.Len() > 0 {
			// Trim trailing newline if any, to prevent extra blank lines
			finalTargetString := strings.TrimRight(targetURLsString.String(), "\n")
			embed.Fields = append(embed.Fields, models.DiscordEmbedField{
				Name:   fieldNameTargets,
				Value:  finalTargetString,
				Inline: false,
			})
		}
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

// FormatInterruptNotificationMessage creates a unified Discord message payload for service interruptions.
func FormatInterruptNotificationMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)
	messageContent := ""
	if mentions != "" {
		messageContent = mentions + "\n"
	}

	// Determine service type from component or scan mode
	serviceType := "Service"
	if summary.Component != "" {
		if strings.Contains(strings.ToLower(summary.Component), "monitor") {
			serviceType = "Monitor Service"
		} else if strings.Contains(strings.ToLower(summary.Component), "scan") || strings.Contains(strings.ToLower(summary.Component), "scheduler") {
			serviceType = "Scan Service"
		} else {
			serviceType = summary.Component
		}
	} else if summary.ScanMode != "" {
		if summary.ScanMode == "automated" {
			serviceType = "Automated Scan Service"
		} else {
			serviceType = "Scan Service"
		}
	}

	title := fmt.Sprintf(":octagonal_sign: %s Interrupted", serviceType)

	description := fmt.Sprintf("The %s was interrupted and may not have completed successfully.", serviceType)
	if summary.ScanSessionID != "" {
		description += fmt.Sprintf("\n**Session ID**: `%s`", summary.ScanSessionID)
	}
	if summary.TargetSource != "" {
		description += fmt.Sprintf("\n**Target Source**: `%s`", summary.TargetSource)
	}
	if summary.ScanMode != "" {
		description += fmt.Sprintf("\n**Mode**: `%s`", summary.ScanMode)
	}

	var fields []models.DiscordEmbedField

	// Add status field
	fields = append(fields, createStatusField("INTERRUPTED", ":octagonal_sign:", true))

	// Add timing information if available
	if summary.ScanDuration > 0 {
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":stopwatch: Duration Before Interrupt",
			Value:  formatDuration(summary.ScanDuration),
			Inline: true,
		})
	}

	// Add target information
	if summary.TotalTargets > 0 {
		fields = append(fields, createCountField("Total Targets", summary.TotalTargets, "", true))
	}

	// Add error details if available
	if len(summary.ErrorMessages) > 0 {
		errorMsg := ""
		maxErrors := 3 // Limit to first 3 errors for readability
		for i, e := range summary.ErrorMessages {
			if i >= maxErrors {
				errorMsg += fmt.Sprintf("\n... and %d more errors", len(summary.ErrorMessages)-maxErrors)
				break
			}
			if i > 0 {
				errorMsg += "\n"
			}
			errorMsg += truncateString(e, 200)
		}
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":warning: Error Details",
			Value:  truncateString(errorMsg, 1000),
			Inline: false,
		})
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       colorWarning, // Orange for interruptions (less severe than critical errors)
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
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// FormatCriticalErrorMessage creates a Discord message payload for critical application errors.
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
		Value:  strings.TrimRight(targetURLsString.String(), "\n"),
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
		return models.DiscordMessagePayload{
			Content: "No file changes detected in this aggregation period.",
		}
	}

	title := fmt.Sprintf("📑 Monitored File Changes (%d)", len(changes))
	if len(changes) == 1 {
		title = "📑 Monitored File Change"
	}

	var descriptionBuilder strings.Builder
	descriptionBuilder.WriteString(fmt.Sprintf("**%d** file(s) changed. Details for up to %d changes shown below:\n\n", len(changes), maxFileChangesToShowInDiscordSummary))

	uniqueHosts := make(map[string]struct{})
	var aggregatedStats models.MonitorAggregatedStats

	for i, change := range changes {
		if i < maxFileChangesToShowInDiscordSummary {
			changeURL, err := url.Parse(change.URL)
			if err == nil {
				uniqueHosts[changeURL.Hostname()] = struct{}{}
			}

			descriptionBuilder.WriteString(fmt.Sprintf("**File:** `%s`\n", truncateString(change.URL, maxFieldValueLength/2))) // Shorter truncate for URL in list
			descriptionBuilder.WriteString(fmt.Sprintf("  - **Type:** `%s`\n", change.ContentType))
			descriptionBuilder.WriteString(fmt.Sprintf("  - **Time:** %s\n", change.ChangeTime.Format("2006-01-02 15:04:05 MST")))
			if change.DiffReportPath != nil && *change.DiffReportPath != "" {
				descriptionBuilder.WriteString(fmt.Sprintf("  - **Diff Report:** Attached (if this is the first part of a multi-part message)\n"))
			}
			if len(change.ExtractedPaths) > 0 {
				descriptionBuilder.WriteString(fmt.Sprintf("  - **Extracted Paths:** %d\n", len(change.ExtractedPaths)))
			}

			descriptionBuilder.WriteString("\n")
		}
		aggregatedStats.TotalChanges++
		aggregatedStats.TotalPaths += len(change.ExtractedPaths)

	}

	if len(changes) > maxFileChangesToShowInDiscordSummary {
		descriptionBuilder.WriteString(fmt.Sprintf("... and %d more file changes.\n", len(changes)-maxFileChangesToShowInDiscordSummary))
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: descriptionBuilder.String(),
		Color:       0xFFA500, // Orange
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields:      []models.DiscordEmbedField{},
	}

	// Add summary fields
	embed.Fields = append(embed.Fields, createCountField("Total Changes", aggregatedStats.TotalChanges, "files", true))
	embed.Fields = append(embed.Fields, createCountField("Unique Hosts Affected", len(uniqueHosts), "hosts", true))
	if aggregatedStats.TotalPaths > 0 {
		embed.Fields = append(embed.Fields, createCountField("Total Extracted Paths", aggregatedStats.TotalPaths, "paths", true))
	}
	embed.Footer = createStandardFooter("File Change Aggregation")
	return createWebhookPayloadWithMentions(embed, cfg)
}

// FormatAggregatedMonitorErrorsMessage creates a Discord message payload for aggregated monitor errors.
func FormatAggregatedMonitorErrorsMessage(errors []models.MonitorFetchErrorInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	allowedMentions := models.AllowedMentions{
		Parse: []string{"roles"}, // Mention roles if specified in config
		Roles: cfg.MentionRoleIDs,
	}

	title := fmt.Sprintf(":x: %d Monitor Error(s) Detected (Monitor)", len(errors)) // Added (Monitor)
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
	return models.DiscordMessagePayload{
		Username:  monsterIncUsername,
		AvatarURL: monsterIncIconURL,
		Content:   buildMentions(cfg.MentionRoleIDs),
		Embeds:    []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// calculateMonitorAggregatedStats calculates aggregated statistics from file changes
func calculateMonitorAggregatedStats(changes []models.FileChangeInfo) models.MonitorAggregatedStats {
	stats := models.MonitorAggregatedStats{
		TotalChanges: len(changes),
	}

	// Calculate total paths from the changes
	for _, change := range changes {
		stats.TotalPaths += len(change.ExtractedPaths)
	}

	return stats
}

// calculateContentTypeBreakdown creates a breakdown of changes by content type
func calculateContentTypeBreakdown(changes []models.FileChangeInfo) map[string]int {
	breakdown := make(map[string]int)

	for _, change := range changes {
		contentType := change.ContentType
		if contentType == "" {
			contentType = "unknown"
		}
		breakdown[contentType]++
	}

	return breakdown
}

// FormatMonitorCycleCompleteMessage creates a Discord message payload for monitor cycle completion.
func FormatMonitorCycleCompleteMessage(data models.MonitorCycleCompleteData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	title := fmt.Sprintf("🔄 Monitor Cycle Complete (%d URLs)", data.TotalMonitored)
	if data.TotalMonitored == 0 {
		title = "🔄 Monitor Cycle Complete (No URLs Monitored)"
	}

	description := fmt.Sprintf("Monitoring cycle finished at %s.", data.Timestamp.Format("2006-01-02 15:04:05 MST"))
	if len(data.ChangedURLs) > 0 {
		description += fmt.Sprintf("\n**%d** URL(s) had content changes detected in this cycle.", len(data.ChangedURLs))
	} else {
		description += "\nNo content changes detected in any monitored URLs this cycle."
	}
	if data.ReportPath != "" {
		description += "\nAn aggregated diff report for all changes in this cycle is attached."
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       0x2ECC71, // Green for successful cycle
		Timestamp:   data.Timestamp.UTC().Format(time.RFC3339),
		Fields:      []models.DiscordEmbedField{},
	}

	embed.Fields = append(embed.Fields, createCountField("Total Monitored URLs", data.TotalMonitored, "URLs", true))
	embed.Fields = append(embed.Fields, createCountField("URLs with Changes", len(data.ChangedURLs), "URLs", true))

	if len(data.ChangedURLs) > 0 {
		// Use the new createSummaryListField for changed URLs
		changedURLSummary := createSummaryListField(data.ChangedURLs, "changed URLs", maxTargetsToShowInDiscordSummary, "- ")
		embed.Fields = append(embed.Fields, models.DiscordEmbedField{
			Name:   fmt.Sprintf("Changed URLs (Up to %d shown)", maxTargetsToShowInDiscordSummary),
			Value:  changedURLSummary,
			Inline: false,
		})
	}

	// If there were file changes, let's try to add a summary of those too.
	if len(data.FileChanges) > 0 {
		aggregatedStats := calculateMonitorAggregatedStats(data.FileChanges) // Use existing helper
		contentTypeBreakdown := calculateContentTypeBreakdown(data.FileChanges)

		embed.Fields = append(embed.Fields, models.DiscordEmbedField{Name: "📊 Change Details Summary", Value: "---", Inline: false})
		embed.Fields = append(embed.Fields, createCountField("Total Content Diffs", aggregatedStats.TotalChanges, "diffs", true))
		if aggregatedStats.TotalPaths > 0 {
			embed.Fields = append(embed.Fields, createCountField("Total Extracted Paths", aggregatedStats.TotalPaths, "paths", true))
		}

		if len(contentTypeBreakdown) > 0 {
			var breakdownStr strings.Builder
			for ct, count := range contentTypeBreakdown {
				breakdownStr.WriteString(fmt.Sprintf("- `%s`: %d\n", ct, count))
			}
			embed.Fields = append(embed.Fields, models.DiscordEmbedField{
				Name:   "Content Type Breakdown (Changes)",
				Value:  strings.TrimSuffix(breakdownStr.String(), "\n"),
				Inline: false,
			})
		}
	}

	embed.Footer = createStandardFooter("Monitor Cycle Complete")
	return createStandardWebhookPayload(embed, cfg, "") // No specific content needed for this one
}

// createSummaryListField creates a string for a field value, summarizing a list of items.
// It shows up to 'maxToShow' items and adds an ellipsis if there are more.
func createSummaryListField(items []string, itemNamePlural string, maxToShow int, itemPrefix string) string {
	if len(items) == 0 {
		return fmt.Sprintf("No %s.", itemNamePlural)
	}

	var builder strings.Builder
	count := 0
	for i, item := range items {
		if i >= maxToShow {
			break
		}
		line := fmt.Sprintf("%s%s\n", itemPrefix, item)
		// Check if adding the next line would exceed Discord's field value limit.
		// This is a rough check; actual limit can be complex with markdown.
		if builder.Len()+len(line) > maxFieldValueLength-100 { // -100 for "and X more..."
			break
		}
		builder.WriteString(line)
		count++
	}

	remaining := len(items) - count
	if remaining > 0 {
		builder.WriteString(fmt.Sprintf("... and %d more %s.\n", remaining, itemNamePlural))
	}
	return strings.TrimSuffix(builder.String(), "\n")
}

// createBulletedListField formats a list of strings into one or more embed fields.
// It tries to fit as many items as possible into a field without exceeding Discord's limits.
// If the list is too long for one field, it creates subsequent fields labeled "(cont.)".
func createBulletedListFields(title string, items []string, maxItemsPerField int, maxTotalLengthPerField int, inline bool) []models.DiscordEmbedField {
	if len(items) == 0 {
		return []models.DiscordEmbedField{}
	}

	var fields []models.DiscordEmbedField
	var currentFieldBuilder strings.Builder
	var itemsInCurrentField int

	baseTitle := title

	for i, item := range items {
		line := fmt.Sprintf("- %s\n", truncateString(item, maxTotalLengthPerField-5)) // -5 for "- "

		if currentFieldBuilder.Len()+len(line) > maxTotalLengthPerField || itemsInCurrentField >= maxItemsPerField {
			// Finalize current field
			fieldValue := strings.TrimSuffix(currentFieldBuilder.String(), "\n")
			if fieldValue == "" { // Avoid empty field if first item itself is too long
				fieldValue = "- Item(s) too long to display individually."
			}
			fields = append(fields, models.DiscordEmbedField{
				Name:   baseTitle,
				Value:  fieldValue,
				Inline: inline,
			})

			// Start new field
			currentFieldBuilder.Reset()
			itemsInCurrentField = 0
			baseTitle = fmt.Sprintf("%s (cont.)", title) // Subsequent fields get a "(cont.)"
		}

		currentFieldBuilder.WriteString(line)
		itemsInCurrentField++

		// If it's the last item and it hasn't triggered a new field creation, but current field has content
		if i == len(items)-1 && currentFieldBuilder.Len() > 0 {
			break // The loop will end, and the last field will be added after the loop
		}
	}

	// Add the last or only field
	if currentFieldBuilder.Len() > 0 {
		fields = append(fields, models.DiscordEmbedField{
			Name:   baseTitle,
			Value:  strings.TrimSuffix(currentFieldBuilder.String(), "\n"),
			Inline: inline,
		})
	}
	return fields
}

// FormatSecondaryReportPartMessage creates a minimal Discord message payload for a secondary part of a multi-part report.
func FormatSecondaryReportPartMessage(scanSessionID string, partNumber int, totalParts int, cfg *config.NotificationConfig) models.DiscordMessagePayload {
	description := fmt.Sprintf("Attached is part %d of %d of the scan report (Session ID: %s).", partNumber, totalParts, scanSessionID)
	if scanSessionID == "" {
		description = fmt.Sprintf("Attached is part %d of %d of the report.", partNumber, totalParts)
	}

	var content string
	if len(cfg.MentionRoleIDs) > 0 {
		var mentions []string
		for _, roleID := range cfg.MentionRoleIDs {
			mentions = append(mentions, fmt.Sprintf("<@&%s>", roleID))
		}
		content = strings.Join(mentions, " ") + "\n"
	}

	payload := models.DiscordMessagePayload{
		Username:  DiscordUsername,
		AvatarURL: DiscordAvatarURL,
		Content:   content,
		Embeds: []models.DiscordEmbed{
			{
				Title:       fmt.Sprintf("📄 Report Part %d/%d", partNumber, totalParts),
				Description: description,
				Color:       DefaultEmbedColor,
				Timestamp:   time.Now().Format(time.RFC3339),
			},
		},
	}
	return payload
}
