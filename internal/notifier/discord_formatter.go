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
	maxTargetsInMessage       = 10
	maxDescriptionLength      = 4096 // Discord embed description limit
	maxFieldValueLength       = 1024 // Discord embed field value limit
	maxFields                 = 25   // Discord embed field limit
	colorGreen                = 0x2ECC71
	colorRed                  = 0xE74C3C
	colorBlue                 = 0x3498DB
	colorOrange               = 0xE67E22
	monsterIncIconURL         = "" // Placeholder icon - User should provide a publicly accessible URL to their favicon.ico or other desired avatar.
	monsterIncUsername        = "MonsterInc"
	defaultScanCompleteTitle  = ":white_check_mark: Scan Complete"
	defaultScanStartTitle     = ":rocket: Scan Started"
	defaultCriticalErrorTitle = ":x: Critical Error"
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
		Color:       0x3498DB, // Blue
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc Scanning Platform",
		},
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
		color = 0x2ECC71 // Green
		statusEmoji = ":white_check_mark:"
	case models.ScanStatusFailed:
		title = fmt.Sprintf(":x: Scan Failed: %s", summary.TargetSource)
		color = 0xE74C3C // Red
		statusEmoji = ":x:"
	case models.ScanStatusPartialComplete:
		title = fmt.Sprintf(":warning: Scan Partially Completed: %s", summary.TargetSource)
		color = 0xF39C12 // Orange
		statusEmoji = ":warning:"
	case models.ScanStatusInterrupted:
		title = fmt.Sprintf(":octagonal_sign: Scan Interrupted: %s", summary.TargetSource)
		color = 0x95A5A6 // Grey
		statusEmoji = ":octagonal_sign:"
	default:
		title = fmt.Sprintf(":question: Scan Status Unknown: %s", summary.TargetSource)
		color = 0x7F8C8D // Grey
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
			Name:   ":mag: Probe Stats",
			Value:  fmt.Sprintf("Discoverable: %d | Successful: %d | Failed: %d", summary.ProbeStats.DiscoverableItems, summary.ProbeStats.SuccessfulProbes, summary.ProbeStats.FailedProbes),
			Inline: false,
		})
	}

	if summary.DiffStats.New > 0 || summary.DiffStats.Old > 0 || summary.DiffStats.Existing > 0 {
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":arrows_counterclockwise: Diff Stats",
			Value:  fmt.Sprintf("New: %d | Old: %d | Existing: %d", summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing),
			Inline: false,
		})
	}

	if summary.RetriesAttempted > 0 && (models.ScanStatus(summary.Status) == models.ScanStatusFailed || models.ScanStatus(summary.Status) == models.ScanStatusPartialComplete) {
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":hourglass_flowing_sand: Retries",
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
			Name:   ":exclamation: Errors",
			Value:  truncateString(errorMsg, 1000), // Truncate overall error message block
			Inline: false,
		})
	}

	if summary.ReportPath != "" {
		// The actual file attachment is handled by DiscordNotifier.SendNotification
		// This field just informs the user that a report is available.
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":page_facing_up: Report",
			Value:  "Report attached.", // Or provide a link if it's uploaded elsewhere
			Inline: false,
		})
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      fields,
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc Scan",
		},
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
			Name:   "Scanned Target URLs",
			Value:  targetURLsString.String(),
			Inline: false,
		})
	} else if summary.TargetSource != "" {
		// Fallback to TargetSource if Targets list is empty but source is known
		embed.Fields = append(embed.Fields, models.DiscordEmbedField{
			Name:   "Target Source",
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
			Name:   "Error Details",
			Value:  truncateString(errorMsg, 1000), // Truncate overall error message block
			Inline: false,
		})
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       0xDD2E44, // Darker Red
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      fields,
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc System Alert",
		},
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

func FormatInitialMonitoredURLsMessage(monitoredURLs []string, cfg config.NotificationConfig) models.DiscordMessagePayload {
	allowedMentions := models.AllowedMentions{
		Parse: []string{"roles"}, // Mention roles if specified in config
		Roles: cfg.MentionRoleIDs,
	}

	description := "Monitoring the following URLs for changes:\n\n"
	for i, url := range monitoredURLs {
		description += fmt.Sprintf("%d. %s\n", i+1, url)
		if len(description) > 3800 { // Keep under Discord's limit
			description += fmt.Sprintf("\n...and %d more URLs.", len(monitoredURLs)-(i+1))
			break
		}
	}

	embed := models.DiscordEmbed{
		Title:       ":pencil: Initial File Monitoring Targets",
		Description: description,
		Color:       colorBlue, // Green
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc File Monitor",
		},
	}

	return models.DiscordMessagePayload{
		Username:        monsterIncUsername,
		AvatarURL:       monsterIncIconURL,
		Embeds:          []models.DiscordEmbed{embed},
		AllowedMentions: &allowedMentions,
	}
}

func FormatAggregatedFileChangesMessage(changes []models.FileChangeInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	if len(changes) == 0 {
		return models.DiscordMessagePayload{}
	}

	allowedMentions := models.AllowedMentions{
		Parse: []string{"roles"},
		Roles: cfg.MentionRoleIDs,
	}

	description := "Multiple file changes detected:\n\n"
	maxLength := 3800 // Discord embed description limit is 4096, leave some room

	for i, change := range changes {
		changeEntry := fmt.Sprintf("**%d. URL:** %s\n   - **Content Type:** `%s`\n   - **Time:** %s\n   - **New Hash:** `%s`\n   - **Old Hash:** `%s`\n",
			i+1,
			change.URL,
			change.ContentType,
			change.ChangeTime.Format(time.RFC1123),
			change.NewHash,
			change.OldHash,
		)
		if change.DiffReportPath != nil && *change.DiffReportPath != "" {
			// Note: Discord messages don't directly support local file links.
			// This path is for user reference if they have access to the filesystem where reports are stored.
			// Alternatively, if reports are uploaded to a web-accessible location, that URL could be put here.
			baseName := filepath.Base(*change.DiffReportPath)
			changeEntry += fmt.Sprintf("   - **Diff Report:** `%s` (details available in generated report)\n", baseName)
		}
		changeEntry += "\n"

		if len(description)+len(changeEntry) > maxLength {
			description += fmt.Sprintf("...and %d more changes.", len(changes)-i)
			break
		}
		description += changeEntry
	}

	embeds := []models.DiscordEmbed{{
		Title:       ":white_check_mark: Aggregated File Changes Detected",
		Description: description,
		Color:       colorGreen, // Green
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc File Monitor",
		},
	}}

	return models.DiscordMessagePayload{
		Username:        monsterIncUsername,
		AvatarURL:       monsterIncIconURL,
		Embeds:          embeds,
		AllowedMentions: &allowedMentions,
	}
}

// func FormatMonitorFetchErrorMessage(url string, fetchError error, cfg config.NotificationConfig) models.DiscordMessagePayload {
// 	allowedMentions := models.AllowedMentions{
// 		Parse: []string{"roles"}, // Mention roles if specified in config
// 		Roles: cfg.MentionRoleIDs,
// 	}

// 	title := ":x: Monitor: File Fetch Error"
// 	description := fmt.Sprintf("An error occurred while trying to fetch the monitored file: **%s**", url)

// 	embed := models.DiscordEmbed{
// 		Title:       title,
// 		Description: description,
// 		Color:       0xff0000, // Red for error
// 		Timestamp:   time.Now().Format(time.RFC3339),
// 		Fields: []models.DiscordEmbedField{
// 			{
// 				Name:   "Error Details",
// 				Value:  truncateString(fetchError.Error(), 1000),
// 				Inline: false,
// 			},
// 		},
// 		Footer: &models.DiscordEmbedFooter{
// 			Text: "MonsterInc File Monitor",
// 		},
// 	}

// 	return models.DiscordMessagePayload{
// 		Embeds:          []models.DiscordEmbed{embed},
// 		AllowedMentions: &allowedMentions,
// 	}
// }

func FormatAggregatedMonitorErrorsMessage(errors []models.MonitorFetchErrorInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	allowedMentions := models.AllowedMentions{
		Parse: []string{"roles"}, // Mention roles if specified in config
		Roles: cfg.MentionRoleIDs,
	}

	title := fmt.Sprintf(":x: Monitor: %d Fetch/Process Error(s) Detected", len(errors))
	var descriptionBuilder strings.Builder
	descriptionBuilder.WriteString(fmt.Sprintf("Found **%d** error(s) during file monitoring operations:\n", len(errors)))

	for i, errInfo := range errors {
		if i > 0 {
			descriptionBuilder.WriteString("\n")
		}
		descriptionBuilder.WriteString(fmt.Sprintf("• **URL**: %s\n  **Source**: `%s`\n  **Error**: `%s`\n  **Time**: %s\n",
			errInfo.URL,
			errInfo.Source,
			truncateString(errInfo.Error, 200), // Truncate individual error message if too long
			errInfo.OccurredAt.Format(time.RFC1123)))
		// Limit the number of detailed errors in the message to avoid exceeding Discord limits
		if i >= 4 && len(errors) > 5 { // Show first 4, then a summary if more than 5 total
			descriptionBuilder.WriteString(fmt.Sprintf("\n...and %d more errors.", len(errors)-(i+1)))
			break
		}
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: truncateString(descriptionBuilder.String(), 4000), // Max description length
		Color:       0xffa500,                                          // Orange for warning/multiple errors
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc File Monitor",
		},
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
		Name:   ":globe_with_meridians: Source URL",
		Value:  truncateString(finding.SourceURL, 1000),
		Inline: false,
	})

	// Rule and Description
	fields = append(fields, models.DiscordEmbedField{
		Name:   ":mag: Detection Rule",
		Value:  fmt.Sprintf("**Rule ID**: `%s`\n**Description**: %s", finding.RuleID, truncateString(finding.Description, 800)),
		Inline: false,
	})

	// Severity and Tool
	toolInfo := "Unknown"
	if finding.ToolName != "" {
		toolInfo = finding.ToolName
	}
	fields = append(fields, models.DiscordEmbedField{
		Name:   ":shield: Detection Details",
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
		Name:   ":lock: Secret Preview (Masked)",
		Value:  fmt.Sprintf("`%s`", maskedSecret),
		Inline: true,
	})

	// Verification state if available
	if finding.VerificationState != "" {
		fields = append(fields, models.DiscordEmbedField{
			Name:   ":white_check_mark: Verification",
			Value:  fmt.Sprintf("`%s`", finding.VerificationState),
			Inline: true,
		})
	}

	// Color based on severity
	color := colorOrange // Default for high
	if finding.Severity == "CRITICAL" {
		color = colorRed
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Timestamp:   finding.Timestamp.Format(time.RFC3339),
		Fields:      fields,
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc Secret Detection",
		},
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
