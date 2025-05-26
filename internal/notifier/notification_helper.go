package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"net/http"
	"time"
)

// DiscordWebhookPayload represents a Discord webhook message
type DiscordWebhookPayload struct {
	Content string         `json:"content"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents an embed in a Discord message
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents a field in a Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// NotificationHelper handles sending notifications
type NotificationHelper struct {
	config *config.NotificationConfig
	logger *log.Logger
}

// NewNotificationHelper creates a new NotificationHelper
func NewNotificationHelper(cfg *config.NotificationConfig, logger *log.Logger) *NotificationHelper {
	return &NotificationHelper{
		config: cfg,
		logger: logger,
	}
}

// SendScanStartNotification sends a notification when a scan starts
func (nh *NotificationHelper) SendScanStartNotification(targetSource string) error {
	if !nh.config.NotifyOnScanStart || nh.config.DiscordWebhookURL == "" {
		return nil
	}

	embed := DiscordEmbed{
		Title:       "🔍 MonsterInc Scan Started",
		Description: "Automated scan cycle has begun",
		Color:       0x3498db, // Blue
		Fields: []DiscordEmbedField{
			{
				Name:   "Target Source",
				Value:  targetSource,
				Inline: true,
			},
			{
				Name:   "Start Time",
				Value:  time.Now().Format("2006-01-02 15:04:05 MST"),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return nh.sendDiscordNotification([]DiscordEmbed{embed})
}

// SendScanSuccessNotification sends a notification when a scan completes successfully
func (nh *NotificationHelper) SendScanSuccessNotification(targetSource string, reportPath string, duration time.Duration, urlStats map[string]int) error {
	if !nh.config.NotifyOnSuccess || nh.config.DiscordWebhookURL == "" {
		return nil
	}

	fields := []DiscordEmbedField{
		{
			Name:   "Target Source",
			Value:  targetSource,
			Inline: true,
		},
		{
			Name:   "Duration",
			Value:  fmt.Sprintf("%.2f minutes", duration.Minutes()),
			Inline: true,
		},
		{
			Name:   "Report",
			Value:  reportPath,
			Inline: false,
		},
	}

	// Add URL statistics if available
	if urlStats != nil {
		statsValue := fmt.Sprintf("New: %d | Existing: %d | Old: %d",
			urlStats["new"], urlStats["existing"], urlStats["old"])
		fields = append(fields, DiscordEmbedField{
			Name:   "URL Changes",
			Value:  statsValue,
			Inline: false,
		})
	}

	embed := DiscordEmbed{
		Title:       "✅ MonsterInc Scan Completed Successfully",
		Description: "Automated scan cycle has completed without errors",
		Color:       0x2ecc71, // Green
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return nh.sendDiscordNotification([]DiscordEmbed{embed})
}

// SendScanFailureNotification sends a notification when a scan fails
func (nh *NotificationHelper) SendScanFailureNotification(targetSource string, err error, attempts int) error {
	if !nh.config.NotifyOnFailure || nh.config.DiscordWebhookURL == "" {
		return nil
	}

	errorMsg := "Unknown error"
	if err != nil {
		errorMsg = err.Error()
		// Truncate very long error messages
		if len(errorMsg) > 1000 {
			errorMsg = errorMsg[:997] + "..."
		}
	}

	embed := DiscordEmbed{
		Title:       "❌ MonsterInc Scan Failed",
		Description: "Automated scan cycle failed after all retry attempts",
		Color:       0xe74c3c, // Red
		Fields: []DiscordEmbedField{
			{
				Name:   "Target Source",
				Value:  targetSource,
				Inline: true,
			},
			{
				Name:   "Total Attempts",
				Value:  fmt.Sprintf("%d", attempts),
				Inline: true,
			},
			{
				Name:   "Error",
				Value:  fmt.Sprintf("```\n%s\n```", errorMsg),
				Inline: false,
			},
			{
				Name:   "Time",
				Value:  time.Now().Format("2006-01-02 15:04:05 MST"),
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return nh.sendDiscordNotification([]DiscordEmbed{embed})
}

// SendCriticalErrorNotification sends a notification for critical errors
func (nh *NotificationHelper) SendCriticalErrorNotification(context string, err error) error {
	if !nh.config.NotifyOnCriticalError || nh.config.DiscordWebhookURL == "" {
		return nil
	}

	errorMsg := "Unknown error"
	if err != nil {
		errorMsg = err.Error()
		if len(errorMsg) > 1000 {
			errorMsg = errorMsg[:997] + "..."
		}
	}

	embed := DiscordEmbed{
		Title:       "🚨 MonsterInc Critical Error",
		Description: "A critical error occurred in the application",
		Color:       0x9b59b6, // Purple
		Fields: []DiscordEmbedField{
			{
				Name:   "Context",
				Value:  context,
				Inline: false,
			},
			{
				Name:   "Error",
				Value:  fmt.Sprintf("```\n%s\n```", errorMsg),
				Inline: false,
			},
			{
				Name:   "Time",
				Value:  time.Now().Format("2006-01-02 15:04:05 MST"),
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return nh.sendDiscordNotification([]DiscordEmbed{embed})
}

// sendDiscordNotification sends a notification to Discord webhook
func (nh *NotificationHelper) sendDiscordNotification(embeds []DiscordEmbed) error {
	// Build content with role mentions if configured
	content := ""
	if len(nh.config.MentionRoleIDs) > 0 {
		for _, roleID := range nh.config.MentionRoleIDs {
			content += fmt.Sprintf("<@&%s> ", roleID)
		}
	}

	payload := DiscordWebhookPayload{
		Content: content,
		Embeds:  embeds,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		nh.logger.Printf("[ERROR] NotificationHelper: Failed to marshal Discord payload: %v", err)
		return err
	}

	req, err := http.NewRequest("POST", nh.config.DiscordWebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		nh.logger.Printf("[ERROR] NotificationHelper: Failed to create Discord request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		nh.logger.Printf("[ERROR] NotificationHelper: Failed to send Discord notification: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		nh.logger.Printf("[ERROR] NotificationHelper: Discord webhook returned status %d", resp.StatusCode)
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	nh.logger.Println("[INFO] NotificationHelper: Discord notification sent successfully")
	return nil
}
