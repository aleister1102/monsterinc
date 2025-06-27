package config

// NotificationConfig defines configuration for notifications
type NotificationConfig struct {
	MentionRoleIDs                  []string `json:"mention_role_ids,omitempty" yaml:"mention_role_ids,omitempty"`
	MonitorServiceDiscordWebhookURL string   `json:"monitor_service_discord_webhook_url,omitempty" yaml:"monitor_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
	NotifyOnFailure                 bool     `json:"notify_on_failure" yaml:"notify_on_failure"`
	NotifyOnScanStart               bool     `json:"notify_on_scan_start" yaml:"notify_on_scan_start"`
	NotifyOnSuccess                 bool     `json:"notify_on_success" yaml:"notify_on_success"`
	ScanServiceDiscordWebhookURL    string   `json:"scan_service_discord_webhook_url,omitempty" yaml:"scan_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
}

// NewDefaultNotificationConfig creates default notification configuration
func NewDefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		MentionRoleIDs:                  []string{},
		MonitorServiceDiscordWebhookURL: "",
		NotifyOnFailure:                 true,
		NotifyOnScanStart:               false,
		NotifyOnSuccess:                 false,
		ScanServiceDiscordWebhookURL:    "",
	}
}
