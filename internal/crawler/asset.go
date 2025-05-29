package crawler

import (
	"bytes"
	"net/url"
	"strings"
	"time"

	"monsterinc/internal/models"

	"github.com/PuerkitoBio/goquery"
)

// URLSource is now defined in internal/models/asset.go
// type URLSource struct {
// 	Tag       string
// 	Attribute string
// }

// ExtractedAsset is now defined in internal/models/asset.go
// type ExtractedAsset struct {
// 	AbsoluteURL string
// 	SourceTag   string // e.g., "a"
// 	SourceAttr  string // e.g., "href"
// 	// ContextText string // Optional: text content of the link, or surrounding text
// }

var assetExtractors = []struct {
	Tag       string
	Attribute string
}{
	{"a", "href"},
	{"link", "href"},   // For stylesheets, favicons, etc.
	{"script", "src"},  // For JavaScript files
	{"img", "src"},     // For images
	{"iframe", "src"},  // For embedded frames
	{"form", "action"}, // For form submission URLs
	{"object", "data"}, // For embedded objects
	{"embed", "src"},   // For embedded content
}

// ExtractAssetsFromHTML parses HTML content and extracts various assets like links, scripts, styles, images.
func ExtractAssetsFromHTML(htmlContent []byte, basePageURL *url.URL, crawlerInstance *Crawler) []models.Asset {
	var assets []models.Asset
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
	if err != nil {
		crawlerInstance.logger.Error().Err(err).Msg("Failed to parse HTML content for asset extraction.")
		return assets
	}

	var baseDiscoveryURL string
	if basePageURL != nil {
		baseDiscoveryURL = basePageURL.String()
	} else {
		crawlerInstance.logger.Warn().Msg("AssetExtractor: Base page URL is nil, cannot resolve relative URLs effectively. Relative URLs might be skipped or miscategorized. DiscoveredFrom will be empty.")
	}

	extractFunc := func(i int, s *goquery.Selection, assetType models.AssetType, attributeName string) {
		attrValue, exists := s.Attr(attributeName)
		if !exists || strings.TrimSpace(attrValue) == "" {
			return
		}

		urlsInAttr := []string{attrValue}
		if attributeName == "srcset" {
			urlsInAttr = parseSrcset(attrValue)
		}

		for _, rawURL := range urlsInAttr {
			trimmedRawURL := strings.TrimSpace(rawURL)
			if trimmedRawURL == "" || strings.HasPrefix(trimmedRawURL, "data:") || strings.HasPrefix(trimmedRawURL, "mailto:") || strings.HasPrefix(trimmedRawURL, "tel:") || strings.HasPrefix(trimmedRawURL, "javascript:") {
				continue
			}

			var absoluteURL string

			if basePageURL != nil {
				resolved, errRes := basePageURL.Parse(trimmedRawURL)
				if errRes != nil {
					// log.Printf("[DEBUG] AssetExtractor: Failed to resolve URL '%s' against base '%s': %v", trimmedRawURL, basePageURL.String(), errRes)
					continue
				}
				absoluteURL = resolved.String()
			} else {
				parsedRaw, errParse := url.Parse(trimmedRawURL)
				if errParse != nil || !parsedRaw.IsAbs() {
					// log.Printf("[DEBUG] AssetExtractor: URL '%s' is relative but no base URL provided, or unparsable. Skipping.", trimmedRawURL)
					continue
				}
				absoluteURL = parsedRaw.String()
			}

			asset := models.Asset{
				AbsoluteURL:    absoluteURL,
				SourceTag:      s.Nodes[0].Data,
				SourceAttr:     attributeName,
				Type:           assetType,
				DiscoveredAt:   time.Now(),
				DiscoveredFrom: baseDiscoveryURL,
			}
			assets = append(assets, asset)

			if crawlerInstance != nil {
				crawlerInstance.DiscoverURL(absoluteURL, basePageURL)
			}
		}
	}

	for _, extractor := range assetExtractors {
		doc.Find(extractor.Tag).Each(func(i int, s *goquery.Selection) {
			// Determine AssetType based on tag - this can be more sophisticated
			var assetType models.AssetType
			switch extractor.Tag {
			case "a", "link": // Treat 'link' also as a general link or stylesheet
				// For 'link' tags, could check 'rel' attribute for 'stylesheet' to be more specific
				if s.AttrOr("rel", "") == "stylesheet" {
					assetType = models.AssetTypeStyle
				} else {
					assetType = models.AssetType(extractor.Tag) // Or a more generic AssetTypeLink
				}
			case "script", "img", "iframe", "form", "object", "embed":
				assetType = models.AssetType(extractor.Tag)
			default:
				crawlerInstance.logger.Warn().Str("tag", extractor.Tag).Msg("Unknown tag type for asset extraction, using tag name as type.")
				assetType = models.AssetType(extractor.Tag) // Fallback
			}
			extractFunc(i, s, assetType, extractor.Attribute)
		})
	}
	return assets
}

func parseSrcset(srcset string) []string {
	var urls []string
	parts := strings.Split(srcset, ",")
	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart == "" {
			continue
		}
		urlAndDescriptor := strings.Fields(trimmedPart)
		if len(urlAndDescriptor) > 0 {
			urls = append(urls, urlAndDescriptor[0])
		}
	}
	return urls
}
