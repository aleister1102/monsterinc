package models

// URLSource defines the tag and attribute from which a URL was extracted.
// Moved from internal/crawler/asset.go
type URLSource struct {
	Tag       string `json:"tag"`
	Attribute string `json:"attribute"`
}

// ExtractedAsset represents a URL found in an HTML document or other resource.
// Moved from internal/crawler/asset.go
type ExtractedAsset struct {
	AbsoluteURL string `json:"absolute_url"`
	SourceTag   string `json:"source_tag,omitempty"`  // e.g., "a", "img"
	SourceAttr  string `json:"source_attr,omitempty"` // e.g., "href", "src"
	// ContextText string `json:"context_text,omitempty"` // Optional: text content of the link, or surrounding text
	// DiscoveredFrom string `json:"discovered_from,omitempty"` // URL of the page/resource where this asset was found
}

// Note: Consider adding more fields to ExtractedAsset if needed for reporting or analysis,
// such as the original relative URL, depth of discovery, etc.
