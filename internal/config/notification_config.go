package config

// NotificationConfig defines configuration for notifications
type NotificationConfig struct {
	AutoDeletePartialDiffReports    bool     `json:"auto_delete_partial_diff_reports" yaml:"auto_delete_partial_diff_reports"`
	MentionRoleIDs                  []string `json:"mention_role_ids,omitempty" yaml:"mention_role_ids,omitempty"`
	MonitorServiceDiscordWebhookURL string   `json:"monitor_service_discord_webhook_url,omitempty" yaml:"monitor_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
	NotifyOnCriticalError           bool     `json:"notify_on_critical_error" yaml:"notify_on_critical_error"`
	NotifyOnFailure                 bool     `json:"notify_on_failure" yaml:"notify_on_failure"`
	NotifyOnScanStart               bool     `json:"notify_on_scan_start" yaml:"notify_on_scan_start"`
	NotifyOnSuccess                 bool     `json:"notify_on_success" yaml:"notify_on_success"`
	ScanServiceDiscordWebhookURL    string   `json:"scan_service_discord_webhook_url,omitempty" yaml:"scan_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
}

// NewDefaultNotificationConfig creates default notification configuration
func NewDefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		AutoDeletePartialDiffReports:    false,
		MentionRoleIDs:                  []string{},
		MonitorServiceDiscordWebhookURL: "",
		NotifyOnCriticalError:           true,
		NotifyOnFailure:                 true,
		NotifyOnScanStart:               false,
		NotifyOnSuccess:                 false,
		ScanServiceDiscordWebhookURL:    "",
	}
}
