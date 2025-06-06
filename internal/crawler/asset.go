package crawler

import (
	"bytes"
	"net/url"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/PuerkitoBio/goquery"
)

// ExtractAssetsFromHTML parses HTML content and extracts various assets
func ExtractAssetsFromHTML(
	htmlContent []byte,
	basePageURL *url.URL,
	crawlerInstance *Crawler,
) []models.Asset {
	extractor := NewHTMLAssetExtractor(basePageURL, crawlerInstance)
	return extractor.Extract(htmlContent)
}

// AssetExtractor defines the mapping between HTML tags and their asset attributes
type AssetExtractor struct {
	Tag       string
	Attribute string
}

// HTMLAssetExtractor handles extraction of assets from HTML content
type HTMLAssetExtractor struct {
	basePageURL     *url.URL
	crawlerInstance *Crawler
	extractors      []AssetExtractor
}

// NewHTMLAssetExtractor creates a new HTML asset extractor instance
func NewHTMLAssetExtractor(basePageURL *url.URL, crawlerInstance *Crawler) *HTMLAssetExtractor {
	return &HTMLAssetExtractor{
		basePageURL:     basePageURL,
		crawlerInstance: crawlerInstance,
		extractors:      getDefaultAssetExtractors(),
	}
}

// getDefaultAssetExtractors returns the default set of asset extractors
func getDefaultAssetExtractors() []AssetExtractor {
	return []AssetExtractor{
		{"a", "href"},
		{"link", "href"},
		{"script", "src"},
		{"img", "src"},
		{"iframe", "src"},
		{"form", "action"},
		{"object", "data"},
		{"embed", "src"},
	}
}

// Extract performs the asset extraction from HTML content
func (hae *HTMLAssetExtractor) Extract(htmlContent []byte) []models.Asset {
	doc, err := hae.parseHTML(htmlContent)
	if err != nil {
		hae.crawlerInstance.logger.Error().Err(err).Msg("Failed to parse HTML content for asset extraction")
		return []models.Asset{}
	}

	// Pre-allocate slice with estimated capacity for better performance
	assets := make([]models.Asset, 0, 50)
	baseDiscoveryURL := hae.getBaseDiscoveryURL()

	// Single pass extraction instead of multiple loops
	hae.extractAllAssetsInSinglePass(doc, baseDiscoveryURL, &assets)

	return assets
}

// extractAllAssetsInSinglePass extracts all assets in a single DOM traversal for better performance
func (hae *HTMLAssetExtractor) extractAllAssetsInSinglePass(doc *goquery.Document, baseDiscoveryURL string, assets *[]models.Asset) {
	// Extract all common asset elements in one pass
	doc.Find("a[href], link[href], script[src], img[src], iframe[src], form[action], object[data], embed[src]").Each(func(i int, s *goquery.Selection) {
		tagName := goquery.NodeName(s)
		var attrName string

		switch tagName {
		case "a", "link":
			attrName = "href"
		case "script", "img", "iframe", "embed":
			attrName = "src"
		case "form":
			attrName = "action"
		case "object":
			attrName = "data"
		default:
			return
		}

		if asset := hae.createAssetFromSelection(s, tagName, attrName, baseDiscoveryURL); asset != nil {
			*assets = append(*assets, *asset)
		}
	})

	// Handle special case for srcset attributes
	doc.Find("img[srcset], source[srcset]").Each(func(i int, s *goquery.Selection) {
		if srcset, exists := s.Attr("srcset"); exists && strings.TrimSpace(srcset) != "" {
			urls := hae.parseSrcsetURLs(srcset)
			for _, rawURL := range urls {
				if asset := hae.createAssetFromURL(rawURL, s, "srcset", models.AssetTypeImage, baseDiscoveryURL); asset != nil {
					*assets = append(*assets, *asset)
				}
			}
		}
	})
}

// createAssetFromSelection creates an asset from a goquery selection with optimized attribute access
func (hae *HTMLAssetExtractor) createAssetFromSelection(selection *goquery.Selection, tagName, attrName, baseDiscoveryURL string) *models.Asset {
	attrValue, exists := selection.Attr(attrName)
	if !exists || strings.TrimSpace(attrValue) == "" {
		return nil
	}

	assetType := hae.determineAssetTypeOptimized(tagName, selection)
	return hae.createAssetFromURL(attrValue, selection, attrName, assetType, baseDiscoveryURL)
}

// determineAssetTypeOptimized optimized version of asset type determination
func (hae *HTMLAssetExtractor) determineAssetTypeOptimized(tagName string, selection *goquery.Selection) models.AssetType {
	switch tagName {
	case "a":
		return models.AssetType(tagName)
	case "link":
		if rel := selection.AttrOr("rel", ""); rel == "stylesheet" {
			return models.AssetTypeStyle
		}
		return models.AssetType("link")
	case "script", "img", "iframe", "form", "object", "embed":
		return models.AssetType(tagName)
	default:
		return models.AssetType(tagName)
	}
}

// parseHTML parses HTML content into a goquery document
func (hae *HTMLAssetExtractor) parseHTML(htmlContent []byte) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
	if err != nil {
		return nil, common.WrapError(err, "failed to parse HTML content")
	}
	return doc, nil
}

// getBaseDiscoveryURL returns the base URL for asset discovery
func (hae *HTMLAssetExtractor) getBaseDiscoveryURL() string {
	if hae.basePageURL != nil {
		return hae.basePageURL.String()
	}

	hae.crawlerInstance.logger.Warn().Msg("Base page URL is nil, relative URLs might be skipped")
	return ""
}

// parseSrcsetURLs parses URLs from srcset attribute
func (hae *HTMLAssetExtractor) parseSrcsetURLs(srcset string) []string {
	var urls []string
	parts := strings.Split(srcset, ",")

	for _, part := range parts {
		if extractedURL := hae.extractURLFromSrcsetPart(strings.TrimSpace(part)); extractedURL != "" {
			urls = append(urls, extractedURL)
		}
	}

	return urls
}

// extractURLFromSrcsetPart extracts URL from a single srcset part
func (hae *HTMLAssetExtractor) extractURLFromSrcsetPart(part string) string {
	if part == "" {
		return ""
	}

	fields := strings.Fields(part)
	if len(fields) > 0 {
		return fields[0]
	}

	return ""
}

// createAssetFromURL creates an asset model from a URL
func (hae *HTMLAssetExtractor) createAssetFromURL(
	rawURL string,
	selection *goquery.Selection,
	attributeName string,
	assetType models.AssetType,
	baseDiscoveryURL string,
) *models.Asset {
	trimmedURL := strings.TrimSpace(rawURL)

	if hae.shouldSkipURL(trimmedURL) {
		return nil
	}

	absoluteURL := hae.resolveURL(trimmedURL)
	if absoluteURL == "" {
		return nil
	}

	// Discover URL for crawling
	hae.discoverURLForCrawling(absoluteURL)

	return &models.Asset{
		AbsoluteURL:    absoluteURL,
		SourceTag:      hae.getTagName(selection),
		SourceAttr:     attributeName,
		Type:           assetType,
		DiscoveredAt:   time.Now(),
		DiscoveredFrom: baseDiscoveryURL,
	}
}

// shouldSkipURL checks if URL should be skipped from extraction
func (hae *HTMLAssetExtractor) shouldSkipURL(urlStr string) bool {
	if urlStr == "" {
		return true
	}

	skipPrefixes := []string{"data:", "mailto:", "tel:", "javascript:"}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(urlStr, prefix) {
			return true
		}
	}

	// Check for infinite loop patterns (repeated path segments)
	if hae.hasRepeatedPathSegments(urlStr) {
		hae.crawlerInstance.logger.Warn().
			Str("url", urlStr).
			Msg("Skipping URL with repeated path segments (potential infinite loop)")
		return true
	}

	return false
}

// hasRepeatedPathSegments detects URLs with repeated path segments that might indicate an infinite loop
func (hae *HTMLAssetExtractor) hasRepeatedPathSegments(urlStr string) bool {
	err := urlhandler.ValidateURLFormat(urlStr)
	if err != nil {
		return false
	}

	parsed, err := urlhandler.NormalizeURL(urlStr)
	if err != nil {
		return false
	}

	// Parse URL to extract path
	u, err := url.Parse(parsed)
	if err != nil {
		return false
	}

	// Check for patterns like /vendor/vendor/vendor or /path/path/path
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return false
	}

	segments := strings.Split(path, "/")

	// Look for 3+ consecutive identical segments
	for i := 0; i < len(segments)-2; i++ {
		if segments[i] != "" && segments[i] == segments[i+1] && segments[i] == segments[i+2] {
			return true
		}
	}

	// Check if we have more than 10 segments (also suspicious)
	if len(segments) > 10 {
		return true
	}

	return false
}

// resolveURL resolves a URL against the base page URL
func (hae *HTMLAssetExtractor) resolveURL(rawURL string) string {
	// Use urlhandler.ResolveURL for consistent URL resolution
	resolved, err := urlhandler.ResolveURL(rawURL, hae.basePageURL)
	if err != nil {
		hae.crawlerInstance.logger.Debug().
			Str("url", rawURL).
			Err(err).
			Msg("Failed to resolve URL using urlhandler")
		return ""
	}
	return resolved
}

// getTagName safely extracts tag name from selection
func (hae *HTMLAssetExtractor) getTagName(selection *goquery.Selection) string {
	if len(selection.Nodes) > 0 {
		return selection.Nodes[0].Data
	}
	return ""
}

// discoverURLForCrawling adds URL to crawler's discovery queue
func (hae *HTMLAssetExtractor) discoverURLForCrawling(absoluteURL string) {
	if hae.crawlerInstance != nil {
		// Track the parent URL for this discovered URL
		if hae.basePageURL != nil {
			hae.crawlerInstance.TrackURLParent(absoluteURL, hae.basePageURL.String())
		}
		hae.crawlerInstance.DiscoverURL(absoluteURL, hae.basePageURL)
	}
}
