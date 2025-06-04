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

	embed := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(description).
		WithColor(colorInfo).
		WithTimestamp(time.Now()).
		WithFooter(footerTextScanning, monsterIncIconURL).
		Build()

	return NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		WithContent(messageContent).
		AddEmbed(embed).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		}).
		Build()
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

	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(description).
		WithColor(color).
		WithTimestamp(time.Now()).
		WithFooter(footerTextScanning, monsterIncIconURL)

	if summary.ProbeStats.DiscoverableItems > 0 || summary.ProbeStats.SuccessfulProbes > 0 || summary.ProbeStats.FailedProbes > 0 {
		embedBuilder.AddField(fieldNameProbeStats, fmt.Sprintf("Discoverable: %d | Successful: %d | Failed: %d", summary.ProbeStats.DiscoverableItems, summary.ProbeStats.SuccessfulProbes, summary.ProbeStats.FailedProbes), false)
	}

	if summary.DiffStats.New > 0 || summary.DiffStats.Old > 0 || summary.DiffStats.Existing > 0 {
		embedBuilder.AddField(fieldNameDiffStats, fmt.Sprintf("New: %d | Old: %d | Existing: %d", summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing), false)
	}

	if summary.RetriesAttempted > 0 && (models.ScanStatus(summary.Status) == models.ScanStatusFailed || models.ScanStatus(summary.Status) == models.ScanStatusPartialComplete) {
		embedBuilder.AddField(fieldNameRetries, fmt.Sprintf("This scan failed after %d retry attempt(s).", summary.RetriesAttempted), false)
	}

	if len(summary.ErrorMessages) > 0 {
		errorMsg := ""
		for i, e := range summary.ErrorMessages {
			errorMsg += truncateString(e, 200) // Truncate individual errors
			if i < len(summary.ErrorMessages)-1 {
				errorMsg += "\n"
			}
		}
		embedBuilder.AddField(fieldNameErrors, truncateString(errorMsg, 1000), false)
	}

	if summary.ReportPath != "" {
		embedBuilder.AddField(fieldNameReport, "Report attached.", false)
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
			finalTargetString := strings.TrimRight(targetURLsString.String(), "\n")
			embedBuilder.AddField(fieldNameTargets, finalTargetString, false)
		}
	} else if summary.TargetSource != "" {
		embedBuilder.AddField(fieldNameTargets, fmt.Sprintf("`%s` (No individual URLs listed in summary)", summary.TargetSource), false)
	}

	payloadBuilder := NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		WithContent(messageContent).
		AddEmbed(embedBuilder.Build()).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		})

	return payloadBuilder.Build()
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

	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(description).
		WithColor(colorWarning). // Orange for interruptions
		WithTimestamp(time.Now()).
		WithFooter(footerTextSystemAlerts, monsterIncIconURL)

	// Add status field
	embedBuilder.AddField(fieldNameStatus, ":octagonal_sign: INTERRUPTED", true)

	// Add timing information if available
	if summary.ScanDuration > 0 {
		embedBuilder.AddField(":stopwatch: Duration Before Interrupt", formatDuration(summary.ScanDuration), true)
	}

	// Add target information
	if summary.TotalTargets > 0 {
		embedBuilder.AddField("Total Targets", fmt.Sprintf("%d", summary.TotalTargets), true) // Using createCountField logic directly
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
		embedBuilder.AddField(":warning: Error Details", truncateString(errorMsg, 1000), false)
	}

	return NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		WithContent(messageContent).
		AddEmbed(embedBuilder.Build()).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		}).
		Build()
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

	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(description).
		WithColor(colorCritical). // Purple for critical system alerts
		WithTimestamp(time.Now()).
		WithFooter(footerTextSystemAlerts, monsterIncIconURL)

	if len(summary.ErrorMessages) > 0 {
		errorMsg := ""
		for i, e := range summary.ErrorMessages {
			errorMsg += truncateString(e, 200) // Truncate individual errors
			if i < len(summary.ErrorMessages)-1 {
				errorMsg += "\n"
			}
		}
		embedBuilder.AddField(fieldNameErrorDetails, truncateString(errorMsg, 1000), false)
	}

	return NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		WithContent(messageContent).
		AddEmbed(embedBuilder.Build()).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles", "users", "everyone"},
			Roles: cfg.MentionRoleIDs,
		}).
		Build()
}

// FormatInitialMonitoredURLsMessage creates a Discord message payload for initial monitored URLs.
func FormatInitialMonitoredURLsMessage(monitoredURLs []string, cycleID string, cfg config.NotificationConfig) models.DiscordMessagePayload {
	title := ":pencil: File Monitoring Started"
	description := fmt.Sprintf("**Total URLs**: %d now being monitored for changes", len(monitoredURLs))
	if cycleID != "" {
		description += fmt.Sprintf("\n**Cycle ID**: `%s`", cycleID)
	}

	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(description).
		WithColor(colorInfo). // Blue
		WithTimestamp(time.Now()).
		WithFooter(footerTextMonitoring, monsterIncIconURL)

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
	embedBuilder.AddField(fieldNameTargets, strings.TrimRight(targetURLsString.String(), "\n"), false)

	// Add status field
	embedBuilder.AddField(fieldNameStatus, ":white_check_mark: Active Monitoring", true)

	return NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		AddEmbed(embedBuilder.Build()).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		}).
		Build()
}

// FormatAggregatedFileChangesMessage creates a Discord message payload for aggregated file changes.
func FormatAggregatedFileChangesMessage(changes []models.FileChangeInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	if len(changes) == 0 {
		return NewDiscordMessagePayloadBuilder().WithContent("No file changes detected in this aggregation period.").Build()
	}

	title := fmt.Sprintf("📑 Monitored File Changes (%d)", len(changes))
	if len(changes) == 1 {
		title = "📑 Monitored File Change"
	}

	var descriptionBuilder strings.Builder
	descriptionBuilder.WriteString(fmt.Sprintf("**%d** file(s) changed. Details for up to %d changes shown below:\n", len(changes), maxFileChangesToShowInDiscordSummary))

	// Add cycle ID if available from the first change
	if len(changes) > 0 && changes[0].CycleID != "" {
		descriptionBuilder.WriteString(fmt.Sprintf("**Cycle ID**: `%s`\n", changes[0].CycleID))
	}
	descriptionBuilder.WriteString("\n")

	uniqueHosts := make(map[string]struct{})
	var aggregatedStats models.MonitorAggregatedStats

	for i, change := range changes {
		if i < maxFileChangesToShowInDiscordSummary {
			changeURL, err := url.Parse(change.URL)
			if err == nil {
				uniqueHosts[changeURL.Hostname()] = struct{}{}
			}

			descriptionBuilder.WriteString(fmt.Sprintf("**File:** `%s`\n", truncateString(change.URL, maxFieldValueLength/2)))
			descriptionBuilder.WriteString(fmt.Sprintf("  - **Type:** `%s`\n", change.ContentType))
			descriptionBuilder.WriteString(fmt.Sprintf("  - **Time:** %s\n", change.ChangeTime.Format("2006-01-02 15:04:05 MST")))
			if change.DiffReportPath != nil && *change.DiffReportPath != "" {
				descriptionBuilder.WriteString("  - **Diff Report:** Attached (if this is the first part of a multi-part message)\n")
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

	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(descriptionBuilder.String()).
		WithColor(0xFFA500).             // Orange
		WithTimestamp(time.Now().UTC()). // Ensure UTC for consistency
		WithFooter("File Change Aggregation", monsterIncIconURL)

	// Add summary fields
	embedBuilder.AddField(createCountField("Total Changes", aggregatedStats.TotalChanges, "files", true).Name, createCountField("Total Changes", aggregatedStats.TotalChanges, "files", true).Value, true)
	embedBuilder.AddField(createCountField("Unique Hosts Affected", len(uniqueHosts), "hosts", true).Name, createCountField("Unique Hosts Affected", len(uniqueHosts), "hosts", true).Value, true)
	if aggregatedStats.TotalPaths > 0 {
		embedBuilder.AddField(createCountField("Total Extracted Paths", aggregatedStats.TotalPaths, "paths", true).Name, createCountField("Total Extracted Paths", aggregatedStats.TotalPaths, "paths", true).Value, true)
	}

	return createWebhookPayloadWithMentions(embedBuilder.Build(), cfg) // Assuming createWebhookPayloadWithMentions uses the builder or is adapted
}

// FormatAggregatedMonitorErrorsMessage creates a Discord message payload for aggregated monitor errors.
func FormatAggregatedMonitorErrorsMessage(errors []models.MonitorFetchErrorInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	title := fmt.Sprintf(":x: %d Monitor Error(s) Detected (Monitor)", len(errors))
	description := fmt.Sprintf("**Total Errors**: %d during file monitoring operations", len(errors))

	// Add cycle ID if available from the first error
	if len(errors) > 0 && errors[0].CycleID != "" {
		description += fmt.Sprintf("\n**Cycle ID**: `%s`", errors[0].CycleID)
	}

	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(description).
		WithColor(colorWarning). // Orange for warning/multiple errors
		WithTimestamp(time.Now()).
		WithFooter(footerTextMonitoring, monsterIncIconURL)

	// Add summary field for multiple errors
	if len(errors) > 1 {
		embedBuilder.AddField(":warning: Error Summary", fmt.Sprintf("Found **%d** errors across monitored operations", len(errors)), false)
	}

	// Add individual error fields (limit to prevent Discord embed limits)
	maxErrorsToShow := 5
	for i, errInfo := range errors {
		if i >= maxErrorsToShow {
			embedBuilder.AddField(":page_facing_up: Additional Errors", fmt.Sprintf("... and **%d** more errors not shown here", len(errors)-maxErrorsToShow), false)
			break
		}

		errorTitle := fmt.Sprintf(":exclamation: Error #%d", i+1)
		errorValue := fmt.Sprintf("**URL**: %s\n**Source**: `%s`\n**Time**: %s\n**Error**: %s",
			truncateString(errInfo.URL, 150),
			errInfo.Source,
			errInfo.OccurredAt.Format(timestampFormatReadable),
			truncateString(errInfo.Error, 300))

		embedBuilder.AddField(errorTitle, errorValue, false)
	}

	return NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		WithContent(buildMentions(cfg.MentionRoleIDs)).
		AddEmbed(embedBuilder.Build()).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		}).
		Build()
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

// createStandardWebhookPayload creates a standardized Discord webhook payload using the builder
func createStandardWebhookPayload(embed models.DiscordEmbed, cfg config.NotificationConfig, content string) models.DiscordMessagePayload {
	return NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		WithContent(content).
		AddEmbed(embed).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		}).
		Build()
}

// createWebhookPayloadWithMentions creates a webhook payload with mentions built from config using the builder
func createWebhookPayloadWithMentions(embed models.DiscordEmbed, cfg config.NotificationConfig) models.DiscordMessagePayload {
	return NewDiscordMessagePayloadBuilder().
		WithUsername(monsterIncUsername).
		WithAvatarURL(monsterIncIconURL).
		WithContent(buildMentions(cfg.MentionRoleIDs)).
		AddEmbed(embed).
		WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		}).
		Build()
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
	if data.CycleID != "" {
		description += fmt.Sprintf("\n**Cycle ID**: `%s`", data.CycleID)
	}
	if len(data.ChangedURLs) > 0 {
		description += fmt.Sprintf("\n**%d** URL(s) had content changes detected in this cycle.", len(data.ChangedURLs))
	} else {
		description += "\nNo content changes detected in any monitored URLs this cycle."
	}
	if data.ReportPath != "" {
		description += "\nAn aggregated diff report for all changes in this cycle is attached."
	}

	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(title).
		WithDescription(description).
		WithColor(0x2ECC71).                 // Green for successful cycle
		WithTimestamp(data.Timestamp.UTC()). // Ensure UTC
		WithFooter("Monitor Cycle Complete", monsterIncIconURL)

	embedBuilder.AddField(createCountField("Total Monitored URLs", data.TotalMonitored, "URLs", true).Name, createCountField("Total Monitored URLs", data.TotalMonitored, "URLs", true).Value, true)
	embedBuilder.AddField(createCountField("URLs with Changes", len(data.ChangedURLs), "URLs", true).Name, createCountField("URLs with Changes", len(data.ChangedURLs), "URLs", true).Value, true)

	if len(data.ChangedURLs) > 0 {
		changedURLSummary := createSummaryListField(data.ChangedURLs, "changed URLs", maxTargetsToShowInDiscordSummary, "- ")
		embedBuilder.AddField(fmt.Sprintf("Changed URLs (Up to %d shown)", maxTargetsToShowInDiscordSummary), changedURLSummary, false)
	}

	if len(data.FileChanges) > 0 {
		aggregatedStats := calculateMonitorAggregatedStats(data.FileChanges)
		contentTypeBreakdown := calculateContentTypeBreakdown(data.FileChanges)

		embedBuilder.AddField("📊 Change Details Summary", "---", false)
		embedBuilder.AddField(createCountField("Total Content Diffs", aggregatedStats.TotalChanges, "diffs", true).Name, createCountField("Total Content Diffs", aggregatedStats.TotalChanges, "diffs", true).Value, true)
		if aggregatedStats.TotalPaths > 0 {
			embedBuilder.AddField(createCountField("Total Extracted Paths", aggregatedStats.TotalPaths, "paths", true).Name, createCountField("Total Extracted Paths", aggregatedStats.TotalPaths, "paths", true).Value, true)
		}

		if len(contentTypeBreakdown) > 0 {
			var breakdownStr strings.Builder
			for ct, count := range contentTypeBreakdown {
				breakdownStr.WriteString(fmt.Sprintf("- `%s`: %d\n", ct, count))
			}
			embedBuilder.AddField("Content Type Breakdown (Changes)", strings.TrimSuffix(breakdownStr.String(), "\n"), false)
		}
	}

	return createStandardWebhookPayload(embedBuilder.Build(), cfg, "")
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

	embed := NewDiscordEmbedBuilder().
		WithTitle(fmt.Sprintf("📄 Report Part %d/%d", partNumber, totalParts)).
		WithDescription(description).
		WithColor(DefaultEmbedColor).
		WithTimestamp(time.Now()).
		Build()

	return NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		WithContent(content).
		AddEmbed(embed).
		// No specific AllowedMentions here as content already includes it, or it's a minimal message.
		Build()
}
