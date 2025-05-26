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
	if len(s) > maxLength {
		return s[:maxLength-3] + "..."
	}
	return s
}

func formatDuration(d time.Duration) string {
	return d.Round(time.Second).String()
}

// FormatScanStartMessage creates a Discord message payload for a scan start event.
func FormatScanStartMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)

	description := fmt.Sprintf("**Input File/Source**: `%s`\n**Total Targets**: %d",
		truncateString(summary.ScanID, 200), // ScanID here might be input file path or similar identifier
		summary.TotalTargets)

	var targetFields []models.DiscordEmbedField
	if len(summary.Targets) > 0 {
		targetsDisplay := ""
		for i, t := range summary.Targets {
			if i >= maxTargetsInMessage {
				targetsDisplay += fmt.Sprintf("\n...and %d more.", len(summary.Targets)-maxTargetsInMessage)
				break
			}
			targetsDisplay += fmt.Sprintf("- `%s`\n", truncateString(t, maxFieldValueLength-10))
		}
		targetFields = append(targetFields, models.DiscordEmbedField{
			Name:  "Targets Preview",
			Value: targetsDisplay,
		})
	}

	embed := models.DiscordEmbed{
		Title:       defaultScanStartTitle,
		Description: truncateString(description, maxDescriptionLength),
		Color:       colorBlue,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields:      targetFields,
		Footer: &models.DiscordEmbedFooter{
			Text:    "MonsterInc Scanner",
			IconURL: monsterIncIconURL,
		},
	}

	return models.DiscordMessagePayload{
		Content: mentions,
		Embeds:  []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// FormatScanCompleteMessage creates a Discord message payload for a scan complete event.
func FormatScanCompleteMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)

	title := defaultScanCompleteTitle
	color := colorGreen
	if summary.Status == string(models.ScanStatusFailed) || summary.Status == string(models.ScanStatusPartialComplete) {
		title = ":warning: Scan Finished with Issues"
		color = colorOrange
	}
	if len(summary.ErrorMessages) > 0 {
		title = ":warning: Scan Finished with Errors"
		color = colorOrange
	}

	description := fmt.Sprintf("Scan for `%s` finished.\n**Duration**: %s",
		truncateString(summary.ScanID, 200),
		formatDuration(summary.ScanDuration),
	)

	if summary.ReportPath != "" {
		description += fmt.Sprintf("\n\n:page_facing_up: The full HTML report (`%s`) is attached.", filepath.Base(summary.ReportPath))
	} else {
		description += "\n\nReport was not generated or could not be attached."
	}

	fields := []models.DiscordEmbedField{
		{
			Name:   ":mag: Probe Stats",
			Value:  fmt.Sprintf("**Total Discovered**: %d\n**Successful**: %d\n**Failed**: %d", summary.ProbeStats.DiscoverableItems, summary.ProbeStats.SuccessfulProbes, summary.ProbeStats.FailedProbes),
			Inline: true,
		},
		{
			Name:   ":arrows_counterclockwise: URL Diff Stats",
			Value:  fmt.Sprintf("**New**: %d\n**Existing**: %d\n**Old**: %d", summary.DiffStats.New, summary.DiffStats.Existing, summary.DiffStats.Old),
			Inline: true,
		},
	}

	if len(summary.Targets) > 0 {
		targetsValue := ""
		for i, t := range summary.Targets {
			if i >= maxTargetsInMessage {
				targetsValue += fmt.Sprintf("\n...and %d more.", len(summary.Targets)-maxTargetsInMessage)
				break
			}
			targetsValue += fmt.Sprintf("- `%s`\n", truncateString(t, maxFieldValueLength-10))
		}
		if len(fields)+1 <= maxFields {
			fields = append(fields, models.DiscordEmbedField{
				Name:  ":dart: Targets Processed",
				Value: truncateString(targetsValue, maxFieldValueLength),
			})
		} else {
			description += "\n\n(Target list truncated due to field limit)"
		}
	}

	if len(summary.ErrorMessages) > 0 {
		errorsValue := ""
		for i, errMsg := range summary.ErrorMessages {
			if i >= 3 { // Limit number of error messages displayed
				errorsValue += fmt.Sprintf("\n...and %d more errors.", len(summary.ErrorMessages)-3)
				break
			}
			errorsValue += fmt.Sprintf("- %s\n", truncateString(errMsg, maxFieldValueLength-5))
		}
		if len(fields)+1 <= maxFields {
			fields = append(fields, models.DiscordEmbedField{
				Name:  ":exclamation: Errors Encountered",
				Value: truncateString(errorsValue, maxFieldValueLength),
			})
		} else {
			description += "\n\n(Error message list truncated due to field limit)"
		}
	}

	embed := models.DiscordEmbed{
		Title:       title,
		Description: truncateString(description, maxDescriptionLength),
		Color:       color,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields:      fields,
		Footer: &models.DiscordEmbedFooter{
			Text:    "MonsterInc Scanner",
			IconURL: monsterIncIconURL,
		},
	}

	return models.DiscordMessagePayload{
		Content: mentions,
		Embeds:  []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}

// FormatCriticalErrorMessage creates a Discord message payload for a critical error event.
func FormatCriticalErrorMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	mentions := buildMentions(cfg.MentionRoleIDs)

	description := "A critical error occurred in the application."
	if summary.Component != "" {
		description += fmt.Sprintf("\n**Component**: `%s`", summary.Component)
	}

	var errorFields []models.DiscordEmbedField
	if len(summary.ErrorMessages) > 0 {
		errMsgStr := ""
		for i, msg := range summary.ErrorMessages {
			if i >= 5 { // Limit to 5 error messages in the embed
				errMsgStr += fmt.Sprintf("\n...and %d more errors.", len(summary.ErrorMessages)-5)
				break
			}
			errMsgStr += fmt.Sprintf("- %s\n", truncateString(msg, maxFieldValueLength-5))
		}
		errorFields = append(errorFields, models.DiscordEmbedField{
			Name:  "Error Details",
			Value: truncateString(errMsgStr, maxFieldValueLength),
		})
	} else {
		description += "\nNo specific error messages provided."
	}

	embed := models.DiscordEmbed{
		Title:       defaultCriticalErrorTitle,
		Description: truncateString(description, maxDescriptionLength),
		Color:       colorRed,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields:      errorFields,
		Footer: &models.DiscordEmbedFooter{
			Text:    "MonsterInc Scanner - Critical Failure",
			IconURL: monsterIncIconURL,
		},
	}

	return models.DiscordMessagePayload{
		Content: mentions,
		Embeds:  []models.DiscordEmbed{embed},
		AllowedMentions: &models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		},
	}
}
