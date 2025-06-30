package discord

import (
	"time"
)

// DiscordEmbedBuilder helps in constructing DiscordEmbed objects.
type DiscordEmbedBuilder struct {
	embed     DiscordEmbed
	validator *DiscordEmbedValidator
}

// NewDiscordEmbedBuilder creates a new Discord embed builder
func NewDiscordEmbedBuilder() *DiscordEmbedBuilder {
	return &DiscordEmbedBuilder{
		embed:     DiscordEmbed{},
		validator: NewDiscordEmbedValidator(),
	}
}

// WithTitle sets the embed title
func (deb *DiscordEmbedBuilder) WithTitle(title string) *DiscordEmbedBuilder {
	deb.embed.Title = title
	return deb
}

// WithDescription sets the embed description
func (deb *DiscordEmbedBuilder) WithDescription(description string) *DiscordEmbedBuilder {
	deb.embed.Description = description
	return deb
}

// WithTimestamp sets the embed timestamp
func (deb *DiscordEmbedBuilder) WithTimestamp(timestamp time.Time) *DiscordEmbedBuilder {
	deb.embed.Timestamp = timestamp.Format(time.RFC3339)
	return deb
}

// WithColor sets the embed color
func (deb *DiscordEmbedBuilder) WithColor(color int) *DiscordEmbedBuilder {
	deb.embed.Color = color
	return deb
}

// WithFooter sets the embed footer
func (deb *DiscordEmbedBuilder) WithFooter(text, iconURL string) *DiscordEmbedBuilder {
	deb.embed.Footer = NewDiscordEmbedFooter(text, iconURL)
	return deb
}

// WithAuthor sets the embed author
func (deb *DiscordEmbedBuilder) WithAuthor(name, url, iconURL string) *DiscordEmbedBuilder {
	deb.embed.Author = NewDiscordEmbedAuthor(name, url, iconURL)
	return deb
}

// AddField adds a field to the embed
func (deb *DiscordEmbedBuilder) AddField(name, value string, inline bool) *DiscordEmbedBuilder {
	field := NewDiscordEmbedField(name, value, inline)
	deb.embed.Fields = append(deb.embed.Fields, field)
	return deb
}

// Validate validates the current embed
func (deb *DiscordEmbedBuilder) Validate() error {
	return deb.validator.ValidateEmbed(deb.embed)
}

// Build builds the Discord embed with validation
func (deb *DiscordEmbedBuilder) Build() DiscordEmbed {
	if err := deb.Validate(); err != nil {
		return DiscordEmbed{}
	}
	return deb.embed
}
