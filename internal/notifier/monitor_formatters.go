package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
)

// FormatInitialMonitoredURLsMessage formats the message for initial monitored URLs
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

// FormatAggregatedFileChangesMessage formats the message for aggregated file changes
func FormatAggregatedFileChangesMessage(changes []models.FileChangeInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	content := buildMentions(cfg.MentionRoleIDs)
	if content != "" {
		content += "\n"
	}

	stats := calculateMonitorAggregatedStats(changes)
	batchStats := calculateBatchStatsFromChanges(changes)
	description := buildFileChangesDescription(stats, batchStats)
	embed := buildFileChangesEmbed(description, changes)
	return buildStandardPayloadWithMentions(embed, cfg, content)
}

// buildFileChangesDescription creates the description for file changes message
func buildFileChangesDescription(stats models.MonitorAggregatedStats, batchStats models.BatchStats) string {
	return fmt.Sprintf(
		"🔔 **File changes detected**\n\n"+
			"**Total Changes:** %d\n"+
			"**Total Extracted Paths:** %d\n"+
			"**Total Batches:** %d",
		stats.TotalChanges,
		stats.TotalPaths,
		batchStats.TotalBatches,
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

	addBatchInfoField(embedBuilder, changes)
	addContentTypeBreakdownField(embedBuilder, changes)
	addChangedURLsField(embedBuilder, changes)

	return embedBuilder.Build()
}

// addBatchInfoField adds batch information field
func addBatchInfoField(embedBuilder *DiscordEmbedBuilder, changes []models.FileChangeInfo) {
	if len(changes) == 0 {
		return
	}

	batchInfo := changes[0].BatchInfo
	if batchInfo == nil {
		return
	}

	embedBuilder.AddField("📅 Batch Information", fmt.Sprintf(
		"**Batch Size:** %d\n"+
			"**Batch Number:** %d\n"+
			"**Total Batches:** %d",
		batchInfo.BatchSize,
		batchInfo.BatchNumber,
		batchInfo.TotalBatches,
	), true)
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

func calculateBatchStatsFromChanges(changes []models.FileChangeInfo) models.BatchStats {
	if len(changes) == 0 {
		return models.BatchStats{}
	}

	// Extract batch info from changes - use the first change with batch info
	var batchInfo *models.BatchInfo
	for _, change := range changes {
		if change.BatchInfo != nil {
			batchInfo = change.BatchInfo
			break
		}
	}

	if batchInfo == nil {
		return models.BatchStats{
			UsedBatching:       false,
			TotalBatches:       1,
			CompletedBatches:   1,
			TotalURLsProcessed: len(changes),
		}
	}

	return models.BatchStats{
		UsedBatching:       true,
		TotalBatches:       batchInfo.TotalBatches,
		CompletedBatches:   batchInfo.BatchNumber, // Current batch number indicates progress
		MaxBatchSize:       batchInfo.BatchSize,
		TotalURLsProcessed: len(changes),
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

// FormatAggregatedMonitorErrorsMessage formats the message for aggregated monitor errors
func FormatAggregatedMonitorErrorsMessage(errors []models.MonitorFetchErrorInfo, cfg config.NotificationConfig) models.DiscordMessagePayload {
	content := buildMentions(cfg.MentionRoleIDs)
	if content != "" {
		content += "\n"
	}

	batchStats := calculateBatchStatsFromErrors(errors)
	description := buildMonitorErrorsDescription(errors, batchStats)
	embed := buildMonitorErrorsEmbed(description, errors)
	return buildStandardPayloadWithMentions(embed, cfg, content)
}

// buildMonitorErrorsDescription creates the description for monitor errors message
func buildMonitorErrorsDescription(errors []models.MonitorFetchErrorInfo, batchStats models.BatchStats) string {
	baseDesc := fmt.Sprintf(
		"⚠️ **Monitor errors detected**\n\n"+
			"**Total Errors:** %d",
		len(errors),
	)

	if batchStats.UsedBatching {
		baseDesc += fmt.Sprintf("\n**Total Batches:** %d", batchStats.TotalBatches)
	}

	return baseDesc
}

// buildMonitorErrorsEmbed creates the embed for monitor errors message
func buildMonitorErrorsEmbed(description string, errors []models.MonitorFetchErrorInfo) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("⚠️ Monitor Errors").
		WithDescription(description).
		WithColor(ErrorEmbedColor).
		WithTimestamp(time.Now()).
		WithFooter("MonsterInc Monitor", "")

	addErrorBatchInfoField(embedBuilder, errors)
	addErrorSamplesField(embedBuilder, errors)
	return embedBuilder.Build()
}

// FormatMonitorCycleCompleteMessage formats the message for monitor cycle completion
func FormatMonitorCycleCompleteMessage(data models.MonitorCycleCompleteData, cfg config.NotificationConfig) models.DiscordMessagePayload {
	description := buildCycleCompleteDescription(data)
	embed := buildCycleCompleteEmbed(description, data)
	return buildStandardPayload(embed)
}

// buildCycleCompleteDescription creates the description for cycle complete message
func buildCycleCompleteDescription(data models.MonitorCycleCompleteData) string {
	statusIcon := "✅"
	statusText := "completed successfully"

	if len(data.ChangedURLs) > 0 {
		statusText = fmt.Sprintf("completed with %d changes detected", len(data.ChangedURLs))
	}

	baseDesc := fmt.Sprintf(
		"%s **Monitoring cycle %s**\n\n"+
			"**Cycle ID:** `%s`\n"+
			"**Total Monitored:** %d\n"+
			"**Changed URLs:** %d",
		statusIcon,
		statusText,
		data.CycleID,
		data.TotalMonitored,
		len(data.ChangedURLs),
	)

	// Add batch information if available
	if data.BatchStats != nil && data.BatchStats.UsedBatching {
		baseDesc += fmt.Sprintf(
			"\n**Batch Processing:** %d/%d batches completed",
			data.BatchStats.CompletedBatches,
			data.BatchStats.TotalBatches,
		)
	}

	return baseDesc
}

// buildCycleCompleteEmbed creates the embed for cycle complete message
func buildCycleCompleteEmbed(description string, data models.MonitorCycleCompleteData) models.DiscordEmbed {
	embedBuilder := NewDiscordEmbedBuilder().
		WithTitle("🔄 Monitor Cycle Complete").
		WithDescription(description).
		WithColor(SuccessEmbedColor).
		WithTimestamp(data.Timestamp).
		WithFooter("MonsterInc Monitor", "")

	addCycleBatchStatsField(embedBuilder, data.BatchStats)
	addChangedURLsSummaryField(embedBuilder, data.ChangedURLs)
	addCycleReportField(embedBuilder, data.ReportPath)

	return embedBuilder.Build()
}

// addCycleBatchStatsField adds batch statistics field for cycle complete message
func addCycleBatchStatsField(embedBuilder *DiscordEmbedBuilder, batchStats *models.BatchStats) {
	if batchStats == nil || !batchStats.UsedBatching {
		return
	}

	embedBuilder.AddField("📊 Batch Statistics", fmt.Sprintf(
		"**Total Batches:** %d\n"+
			"**Completed Batches:** %d\n"+
			"**Max Batch Size:** %d\n"+
			"**URLs Processed:** %d",
		batchStats.TotalBatches,
		batchStats.CompletedBatches,
		batchStats.MaxBatchSize,
		batchStats.TotalURLsProcessed,
	), true)
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

func calculateBatchStatsFromErrors(errors []models.MonitorFetchErrorInfo) models.BatchStats {
	if len(errors) == 0 {
		return models.BatchStats{}
	}

	// Extract batch info from errors - use the first error with batch info
	var batchInfo *models.BatchInfo
	for _, error := range errors {
		if error.BatchInfo != nil {
			batchInfo = error.BatchInfo
			break
		}
	}

	if batchInfo == nil {
		return models.BatchStats{
			UsedBatching:       false,
			TotalBatches:       1,
			CompletedBatches:   1,
			TotalURLsProcessed: len(errors),
		}
	}

	return models.BatchStats{
		UsedBatching:       true,
		TotalBatches:       batchInfo.TotalBatches,
		CompletedBatches:   batchInfo.BatchNumber,
		MaxBatchSize:       batchInfo.BatchSize,
		TotalURLsProcessed: len(errors),
	}
}

// addErrorBatchInfoField adds batch information field for errors
func addErrorBatchInfoField(embedBuilder *DiscordEmbedBuilder, errors []models.MonitorFetchErrorInfo) {
	if len(errors) == 0 {
		return
	}

	batchInfo := errors[0].BatchInfo
	if batchInfo == nil {
		return
	}

	embedBuilder.AddField("📅 Batch Information", fmt.Sprintf(
		"**Batch Size:** %d\n"+
			"**Batch Number:** %d\n"+
			"**Total Batches:** %d",
		batchInfo.BatchSize,
		batchInfo.BatchNumber,
		batchInfo.TotalBatches,
	), true)
}
