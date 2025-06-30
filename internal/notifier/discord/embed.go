package discord

// DiscordEmbed represents a Discord embed object.
type DiscordEmbed struct {
	Title       string                 `json:"title,omitempty"`       // Title of embed
	Description string                 `json:"description,omitempty"` // Description of embed
	URL         string                 `json:"url,omitempty"`         // URL of embed
	Timestamp   string                 `json:"timestamp,omitempty"`   // ISO8601 timestamp
	Color       int                    `json:"color,omitempty"`       // Color code of the embed
	Footer      *DiscordEmbedFooter    `json:"footer,omitempty"`
	Image       *DiscordEmbedImage     `json:"image,omitempty"`
	Thumbnail   *DiscordEmbedThumbnail `json:"thumbnail,omitempty"`
	Author      *DiscordEmbedAuthor    `json:"author,omitempty"`
	Fields      []DiscordEmbedField    `json:"fields,omitempty"` // Array of embed field objects
}

// DiscordEmbedFooter represents the footer of an embed.
type DiscordEmbedFooter struct {
	Text    string `json:"text"`               // Footer text
	IconURL string `json:"icon_url,omitempty"` // URL of footer icon (only supports http(s) and attachments)
}

// NewDiscordEmbedFooter creates a new Discord embed footer
func NewDiscordEmbedFooter(text, iconURL string) *DiscordEmbedFooter {
	return &DiscordEmbedFooter{
		Text:    text,
		IconURL: iconURL,
	}
}

// DiscordEmbedImage represents the image of an embed.
type DiscordEmbedImage struct {
	URL string `json:"url"` // Source URL of image (only supports http(s) and attachments)
}

// NewDiscordEmbedImage creates a new Discord embed image
func NewDiscordEmbedImage(url string) *DiscordEmbedImage {
	return &DiscordEmbedImage{URL: url}
}

// DiscordEmbedThumbnail represents the thumbnail of an embed.
type DiscordEmbedThumbnail struct {
	URL string `json:"url"` // Source URL of thumbnail (only supports http(s) and attachments)
}

// NewDiscordEmbedThumbnail creates a new Discord embed thumbnail
func NewDiscordEmbedThumbnail(url string) *DiscordEmbedThumbnail {
	return &DiscordEmbedThumbnail{URL: url}
}

// DiscordEmbedAuthor represents the author of an embed.
type DiscordEmbedAuthor struct {
	Name    string `json:"name"`               // Name of author
	URL     string `json:"url,omitempty"`      // URL of author (only supports http(s))
	IconURL string `json:"icon_url,omitempty"` // URL of author icon (only supports http(s) and attachments)
}

// NewDiscordEmbedAuthor creates a new Discord embed author
func NewDiscordEmbedAuthor(name, url, iconURL string) *DiscordEmbedAuthor {
	return &DiscordEmbedAuthor{
		Name:    name,
		URL:     url,
		IconURL: iconURL,
	}
}
