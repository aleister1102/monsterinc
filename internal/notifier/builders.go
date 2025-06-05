package notifier

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// DiscordEmbedBuilder helps in constructing models.DiscordEmbed objects.
type DiscordEmbedBuilder struct {
	embed models.DiscordEmbed
}

// NewDiscordEmbedBuilder creates a new instance of DiscordEmbedBuilder.
func NewDiscordEmbedBuilder() *DiscordEmbedBuilder {
	return &DiscordEmbedBuilder{
		embed: models.DiscordEmbed{}, // Initialize with an empty embed
	}
}

// WithTitle sets the Title for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithTitle(title string) *DiscordEmbedBuilder {
	b.embed.Title = title
	return b
}

// WithDescription sets the Description for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithDescription(description string) *DiscordEmbedBuilder {
	b.embed.Description = description
	return b
}

// WithURL sets the URL for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithURL(url string) *DiscordEmbedBuilder {
	b.embed.URL = url
	return b
}

// WithTimestamp sets the Timestamp for the DiscordEmbed.
// It accepts a time.Time object and formats it to ISO8601.
func (b *DiscordEmbedBuilder) WithTimestamp(timestamp time.Time) *DiscordEmbedBuilder {
	b.embed.Timestamp = timestamp.Format(time.RFC3339)
	return b
}

// WithColor sets the Color for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithColor(color int) *DiscordEmbedBuilder {
	b.embed.Color = color
	return b
}

// WithFooter sets the Footer for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithFooter(text string, iconURL string) *DiscordEmbedBuilder {
	b.embed.Footer = &models.DiscordEmbedFooter{
		Text:    text,
		IconURL: iconURL,
	}
	return b
}

// WithImage sets the Image for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithImage(url string) *DiscordEmbedBuilder {
	b.embed.Image = &models.DiscordEmbedImage{
		URL: url,
	}
	return b
}

// WithThumbnail sets the Thumbnail for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithThumbnail(url string) *DiscordEmbedBuilder {
	b.embed.Thumbnail = &models.DiscordEmbedThumbnail{
		URL: url,
	}
	return b
}

// WithAuthor sets the Author for the DiscordEmbed.
func (b *DiscordEmbedBuilder) WithAuthor(name string, url string, iconURL string) *DiscordEmbedBuilder {
	b.embed.Author = &models.DiscordEmbedAuthor{
		Name:    name,
		URL:     url,
		IconURL: iconURL,
	}
	return b
}

// AddField adds a DiscordEmbedField to the DiscordEmbed.
func (b *DiscordEmbedBuilder) AddField(name string, value string, inline bool) *DiscordEmbedBuilder {
	b.embed.Fields = append(b.embed.Fields, models.DiscordEmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	})
	return b
}

// WithFields sets all Fields for the DiscordEmbed, replacing any existing ones.
func (b *DiscordEmbedBuilder) WithFields(fields []models.DiscordEmbedField) *DiscordEmbedBuilder {
	b.embed.Fields = fields
	return b
}

// Build returns the constructed models.DiscordEmbed object.
func (b *DiscordEmbedBuilder) Build() models.DiscordEmbed {
	return b.embed
}

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
