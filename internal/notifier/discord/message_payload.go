package discord

// DiscordMessagePayload represents the JSON payload sent to a Discord webhook.
type DiscordMessagePayload struct {
	Content   string         `json:"content,omitempty"`    // Message content (text)
	Username  string         `json:"username,omitempty"`   // Override the default webhook username
	AvatarURL string         `json:"avatar_url,omitempty"` // Override the default webhook avatar
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`     // Array of embed objects
}
