package models

import "time"

// AssetType defines the type of the asset, typically based on the HTML tag.
type AssetType string

// Constants for known asset types, can be expanded.
const (
	AssetTypeLink   AssetType = "a"
	AssetTypeScript AssetType = "script"
	AssetTypeStyle  AssetType = "link" // Assuming 'link' tags for stylesheets are primary styles
	AssetTypeImage  AssetType = "img"
	AssetTypeIframe AssetType = "iframe"
	AssetTypeForm   AssetType = "form"
	AssetTypeObject AssetType = "object"
	AssetTypeEmbed  AssetType = "embed"
	// Add more as needed
)

// URLSource defines the tag and attribute from which a URL was extracted.
// Moved from internal/crawler/asset.go
type URLSource struct {
	Tag       string `json:"tag"`
	Attribute string `json:"attribute"`
}

// Asset represents a generic asset discovered by the crawler.
// This replaces the older ExtractedAsset to avoid confusion and provide a clearer type.
type Asset struct {
	AbsoluteURL    string    `json:"absolute_url"`
	SourceTag      string    `json:"source_tag,omitempty"`  // e.g., "a", "img"
	SourceAttr     string    `json:"source_attr,omitempty"` // e.g., "href", "src"
	Type           AssetType `json:"type,omitempty"`        // Type of asset, e.g., script, image, link
	DiscoveredAt   time.Time `json:"discovered_at,omitempty"`
	DiscoveredFrom string    `json:"discovered_from,omitempty"` // URL of the page where this asset was found
}
