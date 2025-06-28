package discord

// DiscordMessagePayloadBuilder helps in constructing models.DiscordMessagePayload objects.
type DiscordMessagePayloadBuilder struct {
	payload DiscordMessagePayload
}

// NewDiscordMessagePayloadBuilder creates a new instance of DiscordMessagePayloadBuilder.
func NewDiscordMessagePayloadBuilder() *DiscordMessagePayloadBuilder {
	return &DiscordMessagePayloadBuilder{
		payload: DiscordMessagePayload{}, // Initialize with an empty payload
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
func (b *DiscordMessagePayloadBuilder) AddEmbed(embed DiscordEmbed) *DiscordMessagePayloadBuilder {
	b.payload.Embeds = append(b.payload.Embeds, embed)
	return b
}

// Build returns the constructed models.DiscordMessagePayload object.
func (b *DiscordMessagePayloadBuilder) Build() DiscordMessagePayload {
	return b.payload
}
