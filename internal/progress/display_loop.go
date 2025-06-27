package progress

import (
	"fmt"
	"strings"
	"time"
)

// displayLoop vòng lặp hiển thị progress
func (pdm *ProgressDisplayManager) displayLoop() {
	for {
		select {
		case <-pdm.ctx.Done():
			return
		case <-pdm.stopChan:
			return
		case <-pdm.displayTicker.C:
			pdm.displayProgress()
		case <-pdm.triggerDisplay:
			pdm.displayProgress()
		}
	}
}

// displayProgress hiển thị progress hiện tại dưới dạng log thông thường
func (pdm *ProgressDisplayManager) displayProgress() {
	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	scanInfo := pdm.scanProgress.Info()
	monitorInfo := pdm.monitorProgress.Info()

	var builder strings.Builder

	scanOutput := pdm.formatScanProgress(scanInfo)
	if scanOutput != "" {
		builder.WriteString(scanOutput)
	}

	monitorOutput := pdm.formatMonitorProgress(monitorInfo)
	if monitorOutput != "" {
		if builder.Len() > 0 {
			builder.WriteString(" | ")
		}
		builder.WriteString(monitorOutput)
	}

	output := builder.String()

	if output != "" && output != pdm.lastDisplayed {
		pdm.logProgressAsInfoMessage(output)
		pdm.lastDisplayed = output
	}
}

func (pdm *ProgressDisplayManager) formatScanProgress(info ProgressInfo) string {
	if info.Status == ProgressStatusIdle && (info.Current == 0 && info.BatchInfo == nil) {
		return ""
	}

	var builder strings.Builder
	percentage := info.GetPercentage()
	icon := pdm.getStatusIcon(info.Status)
	progressBar := pdm.createProgressBar(percentage, 20)

	if info.BatchInfo != nil {
		displayBatch := info.BatchInfo.CurrentBatch
		if displayBatch == 0 {
			displayBatch = 1
		}

		if info.BatchInfo.TotalURLs > 0 {
			builder.WriteString(fmt.Sprintf("🔍 Scan [Batch %d/%d]: %s %s %.1f%% (%d/%d) | URLs: %d/%d",
				displayBatch, info.BatchInfo.TotalBatches, icon, progressBar, percentage, info.Current, info.Total, info.BatchInfo.ProcessedURLs, info.BatchInfo.TotalURLs))
		} else {
			builder.WriteString(fmt.Sprintf("🔍 Scan [Batch %d/%d]: %s %s %.1f%% (%d/%d)",
				displayBatch, info.BatchInfo.TotalBatches, icon, progressBar, percentage, info.Current, info.Total))
		}
	} else if info.Total > 0 {
		builder.WriteString(fmt.Sprintf("🔍 Scan: %s %s %.1f%% (%d/%d)",
			icon, progressBar, percentage, info.Current, info.Total))
	} else {
		return ""
	}

	if info.Stage != "" {
		builder.WriteString(fmt.Sprintf(" | %s", info.Stage))
	}

	if pdm.config.ShowETAEstimation && info.EstimatedETA > 0 && info.Status == ProgressStatusRunning {
		builder.WriteString(fmt.Sprintf(" | ETA: %s", pdm.formatDuration(info.EstimatedETA)))
	}

	if info.Message != "" {
		builder.WriteString(fmt.Sprintf(" | %s", info.Message))
	}

	return builder.String()
}

func (pdm *ProgressDisplayManager) formatMonitorProgress(info ProgressInfo) string {
	if info.Status == ProgressStatusIdle {
		return ""
	}

	var builder strings.Builder
	percentage := info.GetPercentage()
	icon := pdm.getStatusIcon(info.Status)
	progressBar := pdm.createProgressBar(percentage, 15)

	if info.BatchInfo != nil && info.BatchInfo.TotalBatches > 1 {
		builder.WriteString(fmt.Sprintf("👁 Monitor [Batch %d/%d]: %s %s %.1f%% (%d/%d)",
			info.BatchInfo.CurrentBatch, info.BatchInfo.TotalBatches, icon, progressBar, percentage, info.Current, info.Total))
	} else {
		builder.WriteString(fmt.Sprintf("👁 Monitor: %s %s %.1f%% (%d/%d)",
			icon, progressBar, percentage, info.Current, info.Total))
	}

	if info.MonitorInfo != nil {
		builder.WriteString(fmt.Sprintf(" | P:%d F:%d C:%d",
			info.MonitorInfo.ProcessedURLs, info.MonitorInfo.FailedURLs, info.MonitorInfo.CompletedURLs))
	}

	if info.Message != "" {
		builder.WriteString(fmt.Sprintf(" | %s", info.Message))
	}

	return builder.String()
}

// logProgressAsInfoMessage log progress như một info message thông thường
func (pdm *ProgressDisplayManager) logProgressAsInfoMessage(content string) {
	pdm.logger.Info().Msg(content)
}

// getStatusIcon trả về icon tương ứng với status
func (pdm *ProgressDisplayManager) getStatusIcon(status ProgressStatus) string {
	switch status {
	case ProgressStatusRunning:
		return "⏳"
	case ProgressStatusComplete:
		return "✅"
	case ProgressStatusError:
		return "❌"
	case ProgressStatusCancelled:
		return "🚫"
	case ProgressStatusIdle:
		return "💤"
	default:
		return "❓"
	}
}

// createProgressBar tạo thanh progress bar
func (pdm *ProgressDisplayManager) createProgressBar(percentage float64, width int) string {
	if width <= 0 {
		return ""
	}

	filled := int((percentage / 100.0) * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s]", bar)
}

// formatDuration định dạng duration thành chuỗi readable
func (pdm *ProgressDisplayManager) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}
