package notifier

import "github.com/aleister1102/monsterinc/internal/models"

// DiscordMessagePayloadBuilder helps in constructing models.DiscordMessagePayload objects.
type DiscordMessagePayloadBuilder struct {
	payload models.DiscordMessagePayload
}

// NewDiscordMessagePayloadBuilder creates a new instance of DiscordMessagePayloadBuilder.
func NewDiscordMessagePayloadBuilder() *DiscordMessagePayloadBuilder {
	return &DiscordMessagePayloadBuilder{
		payload: models.DiscordMessagePayload{}, // Initialize with an empty payload
	}
}

// WithContent sets the Content for the DiscordMessagePayload.
func (b *DiscordMessagePayloadBuilder) WithContent(content string) *DiscordMessagePayloadBuilder {
	b.payload.Content = content
	return b
}

// WithUsername sets the Username for the DiscordMessagePayload.
func (b *DiscordMessagePayloadBuilder) WithUsername(username string) *DiscordMessagePayloadBuilder {
	b.payload.Username = username
	return b
}

// WithAvatarURL sets the AvatarURL for the DiscordMessagePayload.
func (b *DiscordMessagePayloadBuilder) WithAvatarURL(avatarURL string) *DiscordMessagePayloadBuilder {
	b.payload.AvatarURL = avatarURL
	return b
}

// AddEmbed adds a models.DiscordEmbed to the DiscordMessagePayload.
func (b *DiscordMessagePayloadBuilder) AddEmbed(embed models.DiscordEmbed) *DiscordMessagePayloadBuilder {
	b.payload.Embeds = append(b.payload.Embeds, embed)
	return b
}

// WithAllowedMentions sets the AllowedMentions for the DiscordMessagePayload.
func (b *DiscordMessagePayloadBuilder) WithAllowedMentions(allowedMentions models.AllowedMentions) *DiscordMessagePayloadBuilder {
	b.payload.AllowedMentions = &allowedMentions
	return b
}

// Build returns the constructed models.DiscordMessagePayload object.
func (b *DiscordMessagePayloadBuilder) Build() models.DiscordMessagePayload {
	return b.payload
}
