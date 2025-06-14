package notifier

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
)

// FormatScanStartMessage formats the message when a scan starts
func FormatScanStartMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	content := buildMentions(cfg.MentionRoleIDs)
	if content != "" {
		content += "\n"
	}

	description := buildScanStartDescription(summary)
	embed := buildScanStartEmbed(description)
	return buildStandardPayloadWithMentions(embed, cfg, content)
}

// buildScanStartDescription creates the description for scan start message
func buildScanStartDescription(summary models.ScanSummaryData) string {
	description := fmt.Sprintf(
		"🚀 **Scan initialization started**\n\n"+
			"**Session ID:** `%s`\n"+
			"**Mode:** %s\n"+
			"**Target Source:** %s\n"+
			"**Total Targets:** %d",
		summary.ScanSessionID,
		strings.ToUpper(summary.ScanMode),
		summary.TargetSource,
		summary.TotalTargets,
	)

	// Add cycle interval for automated mode
	if summary.CycleMinutes > 0 && summary.ScanMode == "automated" {
		cycleDuration := time.Duration(summary.CycleMinutes) * time.Minute
		description += fmt.Sprintf("\n**Scan Cycle:** Every %s", formatDuration(cycleDuration))
	}

	return addTargetURLsToDescription(description, summary.Targets)
}

// addTargetURLsToDescription adds target URLs to description if applicable
func addTargetURLsToDescription(description string, targets []string) string {
	if len(targets) == 0 {
		return description
	}

	if len(targets) <= 10 {
		description += "\n\n**Target URLs:**\n"
		for _, target := range targets {
			description += fmt.Sprintf("• `%s`\n", target)
		}
	} else {
		description += "\n\n**Sample Target URLs:**\n"
		for i := 0; i < 8; i++ {
			description += fmt.Sprintf("• `%s`\n", targets[i])
		}
		description += fmt.Sprintf("• ... and %d more URLs", len(targets)-8)
	}
	return description
}

// buildScanStartEmbed creates the embed for scan start message
func buildScanStartEmbed(description string) models.DiscordEmbed {
	return NewDiscordEmbedBuilder().
		WithTitle("🛡️ Security Scan Started").
		WithDescription(description).
		WithColor(InfoEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Scanner", "").
		Build()
}

// FormatScanCompleteMessage formats the message when a scan completes
func FormatScanCompleteMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	scanStatus := models.ScanStatus(summary.Status)
	content, embedColor, statusEmoji, titleText := determineScanCompleteMessageStyle(scanStatus, cfg)

	description := buildScanCompleteDescription(summary, statusEmoji)
	embed := buildScanCompleteEmbed(description, titleText, embedColor, summary)

	payloadBuilder := NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		WithContent(content).
		AddEmbed(embed)

	return addMentionsIfNeeded(payloadBuilder, scanStatus.IsFailure(), cfg).Build()
}

// FormatScanCompleteMessageWithReports formats the message when a scan completes with report info
func FormatScanCompleteMessageWithReports(summary models.ScanSummaryData, cfg config.NotificationConfig, hasReports bool) models.DiscordMessagePayload {
	scanStatus := models.ScanStatus(summary.Status)
	content, embedColor, statusEmoji, titleText := determineScanCompleteMessageStyle(scanStatus, cfg)

	description := buildScanCompleteDescription(summary, statusEmoji)
	embed := buildScanCompleteEmbedWithReports(description, titleText, embedColor, summary, hasReports)

	payloadBuilder := NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		WithContent(content).
		AddEmbed(embed)

	return addMentionsIfNeeded(payloadBuilder, scanStatus.IsFailure(), cfg).Build()
}

// determineScanCompleteMessageStyle determines the styling based on scan status
func determineScanCompleteMessageStyle(scanStatus models.ScanStatus, cfg config.NotificationConfig) (string, int, string, string) {
	var content string
	var embedColor int
	var statusEmoji string
	var titleText string

	if scanStatus.IsSuccess() {
		embedColor = SuccessEmbedColor
		statusEmoji = "✅"
		titleText = "Scan Completed Successfully"
	} else if scanStatus.IsFailure() {
		embedColor = ErrorEmbedColor
		statusEmoji = "❌"
		titleText = "Scan Failed"
		if len(cfg.MentionRoleIDs) > 0 {
			content = buildMentions(cfg.MentionRoleIDs) + "\n"
		}
	} else {
		embedColor = WarningEmbedColor
		statusEmoji = "⚠️"
		titleText = "Scan Status Unknown"
	}

	return content, embedColor, statusEmoji, titleText
}

// buildScanCompleteDescription creates the description for scan complete message
func buildScanCompleteDescription(summary models.ScanSummaryData, statusEmoji string) string {
	baseDescription := fmt.Sprintf(
		"%s **Scan execution completed**\n\n"+
			"**Session ID:** `%s`\n"+
			"**Mode:** %s\n"+
			"**Status:** %s\n"+
			"**Duration:** %s",
		statusEmoji,
		summary.ScanSessionID,
		strings.ToUpper(summary.ScanMode),
		strings.ToUpper(summary.Status),
		formatDuration(summary.ScanDuration),
	)

	// Add next scan time for automated mode (calculated from current time + cycle minutes)
	if summary.CycleMinutes > 0 && summary.ScanMode == "automated" {
		nextScanTime := time.Now().Add(time.Duration(summary.CycleMinutes) * time.Minute)
		nextScanFormatted := nextScanTime.Format("2006-01-02 15:04:05 MST")
		cycleDuration := time.Duration(summary.CycleMinutes) * time.Minute
		baseDescription += fmt.Sprintf("\n**Next Scan:** %s (in %s)", nextScanFormatted, formatDuration(cycleDuration))
	}

	// Add batch processing info if this is a multi-part report
	if strings.Contains(summary.ReportPath, "part") && strings.Contains(summary.ReportPath, "of") {
		// Extract part info from report path or report part info
		reportPartInfo := extractReportPartInfo(summary.ReportPath)
		if reportPartInfo != "" {
			baseDescription += fmt.Sprintf("\n%s", reportPartInfo)
		}
	}

	return baseDescription
}

// extractReportPartInfo extracts part information from report path
func extractReportPartInfo(reportPath string) string {
	// Look for pattern like "part_1_of_3" in file name
	if reportPath == "" {
		return ""
	}

	// Simple check for multi-part reports
	if strings.Contains(reportPath, "part") && strings.Contains(reportPath, "of") {
		return "Report is split into multiple parts. This is part X."
	}

	return ""
}

// buildScanCompleteEmbed creates the embed for scan complete message
func buildScanCompleteEmbed(description, titleText string, embedColor int, summary models.ScanSummaryData) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(fmt.Sprintf("🛡️ %s", titleText)).
		WithDescription(description).
		WithColor(embedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Scanner", "")

	addProbeStatsField(embedBuilder, summary.ProbeStats)
	addDiffStatsField(embedBuilder, summary.DiffStats)
	addBatchProcessingField(embedBuilder, summary)
	addReportField(embedBuilder, summary.ReportPath)
	addErrorsField(embedBuilder, summary.ErrorMessages)

	return embedBuilder.Build()
}

// buildScanCompleteEmbedWithReports creates the embed for scan complete message with report info
func buildScanCompleteEmbedWithReports(description, titleText string, embedColor int, summary models.ScanSummaryData, hasReports bool) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle(fmt.Sprintf("🛡️ %s", titleText)).
		WithDescription(description).
		WithColor(embedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Scanner", "")

	addProbeStatsField(embedBuilder, summary.ProbeStats)
	addDiffStatsField(embedBuilder, summary.DiffStats)
	addBatchProcessingField(embedBuilder, summary)

	// Use hasReports parameter instead of relying on summary.ReportPath
	if hasReports {
		embedBuilder.AddField("📄 Report", "Detailed report is attached below.", false)
	}

	addErrorsField(embedBuilder, summary.ErrorMessages)

	return embedBuilder.Build()
}

// addProbeStatsField adds probe statistics field to embed
func addProbeStatsField(embedBuilder *DiscordEmbedBuilder, stats models.ProbeStats) {
	embedBuilder.AddField("🔍 Probe Statistics",
		fmt.Sprintf("**Total Probed:** %d\n**Successful:** %d\n**Failed:** %d\n**Discoverable Items:** %d",
			stats.TotalProbed,
			stats.SuccessfulProbes,
			stats.FailedProbes,
			stats.DiscoverableItems),
		true)
}

// addDiffStatsField adds diff statistics field to embed
func addDiffStatsField(embedBuilder *DiscordEmbedBuilder, stats models.DiffStats) {
	embedBuilder.AddField("📊 Diff Statistics",
		fmt.Sprintf("**New:** %d\n**Existing:** %d\n**Old:** %d\n**Changed:** %d",
			stats.New,
			stats.Existing,
			stats.Old,
			stats.Changed),
		true)
}

// addBatchProcessingField adds batch processing info if applicable
func addBatchProcessingField(embedBuilder *DiscordEmbedBuilder, summary models.ScanSummaryData) {
	// Check if this looks like a batch processing scenario
	if strings.Contains(summary.ReportPath, "part") && strings.Contains(summary.ReportPath, "of") {
		// Try to extract batch info from the file path
		if batchInfo := extractBatchInfoFromPath(summary.ReportPath); batchInfo != "" {
			embedBuilder.AddField("📦 Batch Processing", batchInfo, false)
		}
	}
}

// extractBatchInfoFromPath extracts batch processing information from file path
func extractBatchInfoFromPath(reportPath string) string {
	if reportPath == "" {
		return ""
	}

	// Look for pattern like "part_1_of_3" in filename
	fileName := filepath.Base(reportPath)

	// Use regex to find part X of Y pattern
	re := regexp.MustCompile(`part_(\d+)_of_(\d+)`)
	matches := re.FindStringSubmatch(fileName)

	if len(matches) == 3 {
		partNum := matches[1]
		totalParts := matches[2]
		return fmt.Sprintf("**Report Parts:** %s of %s\n**Processing:** Multi-batch scanning completed", partNum, totalParts)
	}

	return ""
}

// addReportField adds report field to embed if report exists
func addReportField(embedBuilder *DiscordEmbedBuilder, reportPath string) {
	if reportPath != "" {
		embedBuilder.AddField("📄 Report", "Detailed report is attached below.", false)
	}
}

// addErrorsField adds errors field to embed if errors exist
func addErrorsField(embedBuilder *DiscordEmbedBuilder, errorMessages []string) {
	if len(errorMessages) > 0 {
		errorText := compressMultipleErrors(errorMessages, MaxErrorTextLength)
		embedBuilder.AddField("❗ Error", fmt.Sprintf("```\n%s\n```", errorText), false)
	}
}

// addMentionsIfNeeded adds mentions to payload builder if needed
func addMentionsIfNeeded(payloadBuilder *DiscordMessagePayloadBuilder, isFailure bool, cfg config.NotificationConfig) *DiscordMessagePayloadBuilder {
	if isFailure && len(cfg.MentionRoleIDs) > 0 {
		payloadBuilder.WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		})
	}
	return payloadBuilder
}

// FormatInterruptNotificationMessage formats the message when a scan is interrupted
func FormatInterruptNotificationMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	content := buildMentions(cfg.MentionRoleIDs)
	if content != "" {
		content += "\n"
	}

	description := buildInterruptDescription(summary)
	embed := buildInterruptEmbed(description, summary)
	return buildStandardPayloadWithMentions(embed, cfg, content)
}

// buildInterruptDescription creates the description for interrupt message
func buildInterruptDescription(summary models.ScanSummaryData) string {
	return fmt.Sprintf(
		"⚠️ **Scan is interrupted**\n\n"+
			"**Session:** `%s`\n"+
			"**Mode:** %s\n"+
			"**Duration:** %s\n"+
			"**Component:** %s",
		summary.ScanSessionID,
		strings.ToUpper(summary.ScanMode),
		formatDuration(summary.ScanDuration),
		summary.Component,
	)
}

// buildInterruptEmbed creates the embed for interrupt message
func buildInterruptEmbed(description string, summary models.ScanSummaryData) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("🛑 Scan is interrupted").
		WithDescription(description).
		WithColor(InterruptEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Scanner", "")

	addPartialResultsField(embedBuilder, summary.ProbeStats)
	addErrorsField(embedBuilder, summary.ErrorMessages)

	return embedBuilder.Build()
}

// addPartialResultsField adds partial results field if available
func addPartialResultsField(embedBuilder *DiscordEmbedBuilder, stats models.ProbeStats) {
	if stats.TotalProbed > 0 {
		embedBuilder.AddField("📊 Kết Quả Một Phần",
			fmt.Sprintf("**Đã quét:** %d\n**Thành công:** %d\n**Thất bại:** %d",
				stats.TotalProbed,
				stats.SuccessfulProbes,
				stats.FailedProbes),
			true)
	}
}

// FormatCriticalErrorMessage formats the message for critical errors
func FormatCriticalErrorMessage(summary models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	content := buildMentions(cfg.MentionRoleIDs)
	if content != "" {
		content += "\n"
	}

	description := buildCriticalErrorDescription(summary)
	embed := buildCriticalErrorEmbed(description, summary)
	return buildStandardPayloadWithMentions(embed, cfg, content)
}

// buildCriticalErrorDescription creates the description for critical error message
func buildCriticalErrorDescription(summary models.ScanSummaryData) string {
	return fmt.Sprintf(
		"🚨 **Critical System Error**\n\n"+
			"**Component:** %s\n"+
			"**Session:** `%s`\n"+
			"**Retry:** %d times",
		summary.Component,
		summary.ScanSessionID,
		summary.RetriesAttempted,
	)
}

// buildCriticalErrorEmbed creates the embed for critical error message
func buildCriticalErrorEmbed(description string, summary models.ScanSummaryData) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("🚨 Critical System Error").
		WithDescription(description).
		WithColor(CriticalErrorEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Scanner", "")

	if len(summary.ErrorMessages) > 0 {
		errorText := compressMultipleErrors(summary.ErrorMessages, MaxCriticalErrorTextLength)
		embedBuilder.AddField("❗ Error Details", fmt.Sprintf("```\n%s\n```", errorText), false)
	}

	return embedBuilder.Build()
}
