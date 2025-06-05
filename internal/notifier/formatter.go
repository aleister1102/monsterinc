package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
)

// Discord formatting constants
const (
	DiscordUsername         = "MonsterInc Security Scanner"
	DiscordAvatarURL        = "https://upload.wikimedia.org/wikipedia/en/thumb/d/da/Monsters_Inc_logo.svg/200px-Monsters_Inc_logo.svg.png"
	DefaultEmbedColor       = 0x2B2D31 // Discord dark theme color
	SuccessEmbedColor       = 0x5CB85C // Bootstrap success green
	ErrorEmbedColor         = 0xD9534F // Bootstrap danger red
	WarningEmbedColor       = 0xF0AD4E // Bootstrap warning orange
	InfoEmbedColor          = 0x5BC0DE // Bootstrap info blue
	MonitorEmbedColor       = 0x6F42C1 // Purple for monitoring
	InterruptEmbedColor     = 0xFD7E14 // Orange for interruptions
	CriticalErrorEmbedColor = 0xDC3545 // Red for critical errors
)

// Utility functions for formatting
func buildMentions(roleIDs []string) string {
	if len(roleIDs) == 0 {
		return ""
	}
	var mentions []string
	for _, roleID := range roleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", roleID))
	}
	return strings.Join(mentions, " ")
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}

func formatDuration(d time.Duration) string {
	return d.Truncate(time.Second).String()
}

// Scan notification formatters
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
		"🚀 **New security scan initiated**\n\n"+
			"**Session ID:** `%s`\n"+
			"**Mode:** %s\n"+
			"**Target Source:** %s\n"+
			"**Total Targets:** %d",
		summary.ScanSessionID,
		strings.ToUpper(summary.ScanMode),
		summary.TargetSource,
		summary.TotalTargets,
	)

	return addTargetURLsToDescription(description, summary.Targets)
}

// addTargetURLsToDescription adds target URLs to description if applicable
func addTargetURLsToDescription(description string, targets []string) string {
	if len(targets) == 0 || len(targets) > 5 {
		return description
	}

	description += "\n\n**Target URLs:**\n"
	for i, target := range targets {
		if i < 5 {
			description += fmt.Sprintf("• `%s`\n", target)
		}
	}
	if len(targets) > 5 {
		description += fmt.Sprintf("• ... and %d more\n", len(targets)-5)
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
	return fmt.Sprintf(
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
	addReportField(embedBuilder, summary.ReportPath)
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

// addReportField adds report field to embed if report exists
func addReportField(embedBuilder *DiscordEmbedBuilder, reportPath string) {
	if reportPath != "" {
		embedBuilder.AddField("📄 Report", "Detailed report is attached below.", false)
	}
}

// addErrorsField adds errors field to embed if errors exist
func addErrorsField(embedBuilder *DiscordEmbedBuilder, errorMessages []string) {
	if len(errorMessages) > 0 {
		errorText := strings.Join(errorMessages, "\n")
		if len(errorText) > 1000 {
			errorText = truncateString(errorText, 1000)
		}
		embedBuilder.AddField("❗ Errors", fmt.Sprintf("```\n%s\n```", errorText), false)
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
		"⚠️ **Scan was interrupted**\n\n"+
			"**Session ID:** `%s`\n"+
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
		WithTitle("🛑 Scan Interrupted").
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
		embedBuilder.AddField("📊 Partial Results",
			fmt.Sprintf("**Probed:** %d\n**Successful:** %d\n**Failed:** %d",
				stats.TotalProbed,
				stats.SuccessfulProbes,
				stats.FailedProbes),
			true)
	}
}

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
		"🚨 **Critical system error occurred**\n\n"+
			"**Component:** %s\n"+
			"**Session ID:** `%s`\n"+
			"**Retry Attempts:** %d",
		summary.Component,
		summary.ScanSessionID,
		summary.RetriesAttempted,
	)
}

// buildCriticalErrorEmbed creates the embed for critical error message
func buildCriticalErrorEmbed(description string, summary models.ScanSummaryData) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("🚨 Critical Error").
		WithDescription(description).
		WithColor(CriticalErrorEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Scanner", "")

	if len(summary.ErrorMessages) > 0 {
		errorText := strings.Join(summary.ErrorMessages, "\n")
		if len(errorText) > 1500 {
			errorText = truncateString(errorText, 1500)
		}
		embedBuilder.AddField("❗ Error Details", fmt.Sprintf("```\n%s\n```", errorText), false)
	}

	return embedBuilder.Build()
}

// Monitor-specific formatters
func FormatInitialMonitoredURLsMessage(monitoredURLs []string, cycleID string, cfg config.NotificationConfig) models.DiscordMessagePayload {
	description := buildMonitoredURLsDescription(monitoredURLs, cycleID)
	embed := buildMonitoredURLsEmbed(description)
	return buildStandardPayload(embed)
}

// buildMonitoredURLsDescription creates the description for monitored URLs message
func buildMonitoredURLsDescription(monitoredURLs []string, cycleID string) string {
	description := fmt.Sprintf(
		"🔍 **Monitoring cycle started**\n\n"+
			"**Cycle ID:** `%s`\n"+
			"**Total URLs:** %d",
		cycleID,
		len(monitoredURLs),
	)

	return addURLSampleToDescription(description, monitoredURLs)
}

// addURLSampleToDescription adds URL samples to description
func addURLSampleToDescription(description string, urls []string) string {
	if len(urls) <= 10 {
		description += "\n\n**Monitored URLs:**\n"
		for _, url := range urls {
			description += fmt.Sprintf("• `%s`\n", url)
		}
	} else {
		description += "\n\n**Sample URLs:**\n"
		for i := 0; i < 8; i++ {
			description += fmt.Sprintf("• `%s`\n", urls[i])
		}
		description += fmt.Sprintf("• ... and %d more URLs", len(urls)-8)
	}
	return description
}

// buildMonitoredURLsEmbed creates the embed for monitored URLs message
func buildMonitoredURLsEmbed(description string) models.DiscordEmbed {
	return NewDiscordEmbedBuilder().
		WithTitle("👁️ File Monitoring Started").
		WithDescription(description).
		WithColor(MonitorEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Monitor", "").
		Build()
}

func FormatAggregatedFileChangesMessage(changes []models.FileChangeInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	content := buildMentions(cfg.MentionRoleIDs)
	if content != "" {
		content += "\n"
	}

	stats := calculateMonitorAggregatedStats(changes)
	description := buildFileChangesDescription(stats)
	embed := buildFileChangesEmbed(description, changes)
	return buildStandardPayloadWithMentions(embed, cfg, content)
}

// buildFileChangesDescription creates the description for file changes message
func buildFileChangesDescription(stats models.MonitorAggregatedStats) string {
	return fmt.Sprintf(
		"🔔 **File changes detected**\n\n"+
			"**Total Changes:** %d\n"+
			"**Total Extracted Paths:** %d",
		stats.TotalChanges,
		stats.TotalPaths,
	)
}

// buildFileChangesEmbed creates the embed for file changes message
func buildFileChangesEmbed(description string, changes []models.FileChangeInfo) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("📝 File Changes Detected").
		WithDescription(description).
		WithColor(WarningEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Monitor", "")

	addContentTypeBreakdownField(embedBuilder, changes)
	addChangedURLsField(embedBuilder, changes)

	return embedBuilder.Build()
}

// addContentTypeBreakdownField adds content type breakdown field
func addContentTypeBreakdownField(embedBuilder *DiscordEmbedBuilder, changes []models.FileChangeInfo) {
	contentTypeBreakdown := calculateContentTypeBreakdown(changes)
	if len(contentTypeBreakdown) > 0 {
		var breakdown strings.Builder
		for contentType, count := range contentTypeBreakdown {
			breakdown.WriteString(fmt.Sprintf("**%s:** %d\n", contentType, count))
		}
		embedBuilder.AddField("📊 Content Types", breakdown.String(), true)
	}
}

// addChangedURLsField adds changed URLs field with samples
func addChangedURLsField(embedBuilder *DiscordEmbedBuilder, changes []models.FileChangeInfo) {
	if len(changes) == 0 {
		return
	}

	sampleSize := 5
	if len(changes) < sampleSize {
		sampleSize = len(changes)
	}

	var changesText strings.Builder
	for i := 0; i < sampleSize; i++ {
		change := changes[i]
		changesText.WriteString(fmt.Sprintf("• %s\n", change.URL))
		if change.DiffReportPath != nil {
			changesText.WriteString("  📄 Report available\n")
		}
	}

	if len(changes) > sampleSize {
		changesText.WriteString(fmt.Sprintf("• ... and %d more changes", len(changes)-sampleSize))
	}

	embedBuilder.AddField("🔍 Changed URLs", changesText.String(), false)
}

// Helper functions
func calculateMonitorAggregatedStats(changes []models.FileChangeInfo) models.MonitorAggregatedStats {
	totalPaths := 0
	for _, change := range changes {
		totalPaths += len(change.ExtractedPaths)
	}

	return models.MonitorAggregatedStats{
		TotalChanges: len(changes),
		TotalPaths:   totalPaths,
	}
}

func calculateContentTypeBreakdown(changes []models.FileChangeInfo) map[string]int {
	breakdown := make(map[string]int)
	for _, change := range changes {
		if change.ContentType != "" {
			breakdown[change.ContentType]++
		} else {
			breakdown["unknown"]++
		}
	}
	return breakdown
}

func createSummaryListField(items []string, itemNamePlural string, maxToShow int, itemPrefix string) string {
	if len(items) == 0 {
		return fmt.Sprintf("No %s", itemNamePlural)
	}

	var result strings.Builder

	showCount := maxToShow
	if len(items) < showCount {
		showCount = len(items)
	}

	for i := 0; i < showCount; i++ {
		result.WriteString(fmt.Sprintf("%s%s\n", itemPrefix, items[i]))
	}

	if len(items) > maxToShow {
		result.WriteString(fmt.Sprintf("%s... and %d more %s", itemPrefix, len(items)-maxToShow, itemNamePlural))
	}

	return result.String()
}

// buildStandardPayload creates a standard payload without mentions
func buildStandardPayload(embed models.DiscordEmbed) models.DiscordMessagePayload {
	return NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		AddEmbed(embed).
		Build()
}

// buildStandardPayloadWithMentions creates a standard payload with mentions
func buildStandardPayloadWithMentions(embed models.DiscordEmbed, cfg config.NotificationConfig, content string) models.DiscordMessagePayload {
	payloadBuilder := NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		WithContent(content).
		AddEmbed(embed)

	if len(cfg.MentionRoleIDs) > 0 {
		payloadBuilder.WithAllowedMentions(models.AllowedMentions{
			Parse: []string{"roles"},
			Roles: cfg.MentionRoleIDs,
		})
	}

	return payloadBuilder.Build()
}

func FormatAggregatedMonitorErrorsMessage(errors []models.MonitorFetchErrorInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	content := buildMentions(cfg.MentionRoleIDs)
	if content != "" {
		content += "\n"
	}

	description := buildMonitorErrorsDescription(errors)
	embed := buildMonitorErrorsEmbed(description, errors)
	return buildStandardPayloadWithMentions(embed, cfg, content)
}

// buildMonitorErrorsDescription creates the description for monitor errors message
func buildMonitorErrorsDescription(errors []models.MonitorFetchErrorInfo) string {
	return fmt.Sprintf(
		"⚠️ **Monitor errors detected**\n\n"+
			"**Total Errors:** %d",
		len(errors),
	)
}

// buildMonitorErrorsEmbed creates the embed for monitor errors message
func buildMonitorErrorsEmbed(description string, errors []models.MonitorFetchErrorInfo) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("⚠️ Monitor Errors").
		WithDescription(description).
		WithColor(ErrorEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Monitor", "")

	addErrorSamplesField(embedBuilder, errors)
	return embedBuilder.Build()
}

// addErrorSamplesField adds error samples field to embed
func addErrorSamplesField(embedBuilder *DiscordEmbedBuilder, errors []models.MonitorFetchErrorInfo) {
	if len(errors) == 0 {
		return
	}

	sampleSize := 5
	if len(errors) < sampleSize {
		sampleSize = len(errors)
	}

	var errorsText strings.Builder
	for i := range sampleSize {
		error := errors[i]
		errorsText.WriteString(fmt.Sprintf("• **%s**\n  ```%s```\n", error.URL, error.Error))
	}

	if len(errors) > sampleSize {
		errorsText.WriteString(fmt.Sprintf("• ... and %d more errors", len(errors)-sampleSize))
	}

	embedBuilder.AddField("❗ Error Details", errorsText.String(), false)
}

func FormatMonitorCycleCompleteMessage(data models.MonitorCycleCompleteData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	description := buildCycleCompleteDescription(data)
	embed := buildCycleCompleteEmbed(description, data)
	return buildStandardPayload(embed)
}

// buildCycleCompleteDescription creates the description for cycle complete message
func buildCycleCompleteDescription(data models.MonitorCycleCompleteData) string {
	return fmt.Sprintf(
		"✅ **Monitoring cycle completed**\n\n"+
			"**Cycle ID:** `%s`\n"+
			"**Total Monitored:** %d\n"+
			"**Changed URLs:** %d",
		data.CycleID,
		data.TotalMonitored,
		len(data.ChangedURLs),
	)
}

// buildCycleCompleteEmbed creates the embed for cycle complete message
func buildCycleCompleteEmbed(description string, data models.MonitorCycleCompleteData) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("🔄 Monitor Cycle Complete").
		WithDescription(description).
		WithColor(SuccessEmbedColor).
		WithTimestamp(data.Timestamp).
		WithFooter("MonsterInc Monitor", "")

	addChangedURLsSummaryField(embedBuilder, data.ChangedURLs)
	addCycleReportField(embedBuilder, data.ReportPath)

	return embedBuilder.Build()
}

// addChangedURLsSummaryField adds changed URLs summary field
func addChangedURLsSummaryField(embedBuilder *DiscordEmbedBuilder, changedURLs []string) {
	if len(changedURLs) > 0 {
		changedURLsText := createSummaryListField(changedURLs, "URLs", 5, "• ")
		embedBuilder.AddField("🔍 Changed URLs", changedURLsText, false)
	}
}

// addCycleReportField adds cycle report field if available
func addCycleReportField(embedBuilder *DiscordEmbedBuilder, reportPath string) {
	if reportPath != "" {
		embedBuilder.AddField("📄 Report", "Cycle report is attached below.", false)
	} else {
		embedBuilder.AddField("📄 Report", "No changes detected - no report generated.", false)
	}
}

func FormatSecondaryReportPartMessage(scanSessionID string, partNumber int, totalParts int, cfg *config.NotificationConfig) models.DiscordMessagePayload {
	description := buildSecondaryReportDescription(scanSessionID, partNumber, totalParts)
	embed := buildSecondaryReportEmbed(description, partNumber, totalParts)
	content := buildSecondaryReportContent(cfg)
	return buildSecondaryReportPayload(embed, content)
}

// buildSecondaryReportDescription creates the description for secondary report message
func buildSecondaryReportDescription(scanSessionID string, partNumber, totalParts int) string {
	return fmt.Sprintf(
		"📄 **Additional report part**\n\n"+
			"**Session ID:** `%s`\n"+
			"**Part:** %d of %d",
		scanSessionID,
		partNumber,
		totalParts,
	)
}

// buildSecondaryReportEmbed creates the embed for secondary report message
func buildSecondaryReportEmbed(description string, partNumber, totalParts int) models.DiscordEmbed {
	return NewDiscordEmbedBuilder().
		WithTitle(fmt.Sprintf("📄 Report Part %d/%d", partNumber, totalParts)).
		WithDescription(description).
		WithColor(DefaultEmbedColor).
		WithTimestamp(time.Now()).
		Build()
}

// buildSecondaryReportContent creates content for secondary report message
func buildSecondaryReportContent(cfg *config.NotificationConfig) string {
	if cfg == nil || len(cfg.MentionRoleIDs) == 0 {
		return ""
	}

	var mentions []string
	for _, roleID := range cfg.MentionRoleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", roleID))
	}
	return strings.Join(mentions, " ") + "\n"
}

// buildSecondaryReportPayload creates payload for secondary report message
func buildSecondaryReportPayload(embed models.DiscordEmbed, content string) models.DiscordMessagePayload {
	return NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		WithContent(content).
		AddEmbed(embed).
		Build()
}
