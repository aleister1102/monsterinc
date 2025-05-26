package notifier

import (
	"fmt"
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
	monsterIncIconURL         = "https://i.imgur.com/kRdXp5X.png" // Placeholder icon
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
				description += fmt.Sprintf("- `%s`\n", truncateString(t, 100))
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
		Content: messageContent,
		Embeds:  []models.DiscordEmbed{embed},
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
			Text: "MonsterInc Scanning Platform",
		},
	}

	return models.DiscordMessagePayload{
		Content: messageContent,
		Embeds:  []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
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
		description += fmt.Sprintf("\n**Related Scan Session ID**: `%s`", summary.ScanSessionID)
	}
	if summary.TargetSource != "" {
		description += fmt.Sprintf("\n**Related Target Source**: `%s`", summary.TargetSource)
	}

	var fields []models.DiscordEmbedField
	if len(summary.ErrorMessages) > 0 {
		errorMsg := ""
		for i, e := range summary.ErrorMessages {
			errorMsg += truncateString(e, 200)
			if i < len(summary.ErrorMessages)-1 {
				errorMsg += "\n"
			}
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
		Color:       0x992D22, // Dark Red
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      fields,
		Footer: &models.DiscordEmbedFooter{
			Text: "MonsterInc Monitoring",
		},
	}

	return models.DiscordMessagePayload{
		Content: messageContent,
		Embeds:  []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// FormatFileChangeNotification creates a Discord message payload for a file change event.
func FormatFileChangeNotification(url, oldHash, newHash, contentType string, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentionText := buildMentions(cfg.MentionRoleIDs)
	title := ":warning: File Change Detected"
	description := fmt.Sprintf("A change was detected for monitored file: **%s**", url)

	color := 0xFFCC00 // Yellow/Orange for warning

	embed := models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      []models.DiscordEmbedField{},
	}

	embed.Fields = append(embed.Fields, models.DiscordEmbedField{
		Name:   "URL",
		Value:  url,
		Inline: false,
	})
	embed.Fields = append(embed.Fields, models.DiscordEmbedField{
		Name:   "Content Type",
		Value:  fmt.Sprintf("`%s`", contentType),
		Inline: true,
	})

	if oldHash != "" {
		embed.Fields = append(embed.Fields, models.DiscordEmbedField{
			Name:   "Previous Hash (SHA256)",
			Value:  fmt.Sprintf("`%s`", truncateString(oldHash, 32)), // Show a truncated hash
			Inline: true,
		})
		embed.Fields = append(embed.Fields, models.DiscordEmbedField{
			Name:   "New Hash (SHA256)",
			Value:  fmt.Sprintf("`%s`", truncateString(newHash, 32)),
			Inline: true,
		})
	} else {
		embed.Fields = append(embed.Fields, models.DiscordEmbedField{
			Name:   "New Hash (SHA256) (New File)",
			Value:  fmt.Sprintf("`%s`", truncateString(newHash, 32)),
			Inline: true,
		})
	}

	// Add a deep link if possible (e.g., to a dashboard or the file itself)
	// embed.Fields = append(embed.Fields, models.DiscordEmbedField{
	// 	Name:   "Link to File",
	// 	Value:  fmt.Sprintf("[View File](%s)", url),
	// 	Inline: false,
	// })

	payload := models.DiscordMessagePayload{
		Content: mentionText,
		Embeds:  []models.DiscordEmbed{embed},
	}
	if len(cfg.MentionRoleIDs) > 0 {
		payload.AllowedMentions = &models.AllowedMentions{Parse: []string{"roles"}}
	}

	return payload
}
