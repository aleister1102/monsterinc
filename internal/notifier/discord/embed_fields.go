package discord

// DiscordEmbedField represents a field in an embed.
type DiscordEmbedField struct {
	Name   string `json:"name"`             // Name of the field
	Value  string `json:"value"`            // Value of the field
	Inline bool   `json:"inline,omitempty"` // Whether or not this field should display inline
}

// NewDiscordEmbedField creates a new Discord embed field
func NewDiscordEmbedField(name, value string, inline bool) DiscordEmbedField {
	return DiscordEmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
}
