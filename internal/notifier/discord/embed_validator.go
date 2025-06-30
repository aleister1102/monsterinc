package discord

import (
	"fmt"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
)

// DiscordEmbedValidator validates Discord embed objects
type DiscordEmbedValidator struct{}

// NewDiscordEmbedValidator creates a new embed validator
func NewDiscordEmbedValidator() *DiscordEmbedValidator {
	return &DiscordEmbedValidator{}
}

// ValidateEmbed validates a Discord embed
func (dev *DiscordEmbedValidator) ValidateEmbed(embed DiscordEmbed) error {
	if len(embed.Title) > 256 {
		return errorwrapper.NewValidationError("title", embed.Title, "title cannot exceed 256 characters")
	}

	if len(embed.Description) > 4096 {
		return errorwrapper.NewValidationError("description", embed.Description, "description cannot exceed 4096 characters")
	}

	if len(embed.Fields) > 25 {
		return errorwrapper.NewValidationError("fields", embed.Fields, "cannot have more than 25 fields")
	}

	// Validate fields
	for i, field := range embed.Fields {
		if len(field.Name) > 256 {
			return errorwrapper.NewValidationError("field_name", field.Name, fmt.Sprintf("field %d name cannot exceed 256 characters", i))
		}
		if len(field.Value) > 1024 {
			return errorwrapper.NewValidationError("field_value", field.Value, fmt.Sprintf("field %d value cannot exceed 1024 characters", i))
		}
		if field.Name == "" {
			return errorwrapper.NewValidationError("field_name", field.Name, fmt.Sprintf("field %d name cannot be empty", i))
		}
		if field.Value == "" {
			return errorwrapper.NewValidationError("field_value", field.Value, fmt.Sprintf("field %d value cannot be empty", i))
		}
	}

	if embed.Footer != nil && len(embed.Footer.Text) > 2048 {
		return errorwrapper.NewValidationError("footer_text", embed.Footer.Text, "footer text cannot exceed 2048 characters")
	}

	if embed.Author != nil && len(embed.Author.Name) > 256 {
		return errorwrapper.NewValidationError("author_name", embed.Author.Name, "author name cannot exceed 256 characters")
	}

	return nil
}
