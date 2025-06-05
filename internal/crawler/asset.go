package crawler

import (
	"bytes"
	"net/url"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"

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

	var assets []models.Asset
	baseDiscoveryURL := hae.getBaseDiscoveryURL()

	for _, extractor := range hae.extractors {
		extractedAssets := hae.extractAssetsFromElements(doc, extractor, baseDiscoveryURL)
		assets = append(assets, extractedAssets...)
	}

	return assets
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

// extractAssetsFromElements extracts assets from HTML elements matching the extractor
func (hae *HTMLAssetExtractor) extractAssetsFromElements(
	doc *goquery.Document,
	extractor AssetExtractor,
	baseDiscoveryURL string,
) []models.Asset {
	var assets []models.Asset

	doc.Find(extractor.Tag).Each(
		func(i int, s *goquery.Selection) {
			elementAssets := hae.extractAssetsFromElement(s, extractor, baseDiscoveryURL)
			assets = append(assets, elementAssets...)
		},
	)

	return assets
}

// extractAssetsFromElement extracts assets from a single HTML element
func (hae *HTMLAssetExtractor) extractAssetsFromElement(
	selection *goquery.Selection,
	extractor AssetExtractor,
	baseDiscoveryURL string,
) []models.Asset {
	attrValue, exists := selection.Attr(extractor.Attribute)
	if !exists || strings.TrimSpace(attrValue) == "" {
		return []models.Asset{}
	}

	urls := hae.parseAttributeURLs(attrValue, extractor.Attribute)
	assetType := hae.determineAssetType(selection, extractor.Tag)

	var assets []models.Asset
	for _, rawURL := range urls {
		if asset := hae.createAssetFromURL(rawURL, selection, extractor.Attribute, assetType, baseDiscoveryURL); asset != nil {
			assets = append(assets, *asset)
		}
	}

	return assets
}

// parseAttributeURLs parses URLs from an attribute value
func (hae *HTMLAssetExtractor) parseAttributeURLs(attrValue, attributeName string) []string {
	if attributeName == "srcset" {
		return hae.parseSrcsetURLs(attrValue)
	}
	return []string{attrValue}
}

// parseSrcsetURLs parses URLs from srcset attribute
func (hae *HTMLAssetExtractor) parseSrcsetURLs(srcset string) []string {
	var urls []string
	parts := strings.Split(srcset, ",")

	for _, part := range parts {
		if url := hae.extractURLFromSrcsetPart(strings.TrimSpace(part)); url != "" {
			urls = append(urls, url)
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

// determineAssetType determines the asset type based on HTML element
func (hae *HTMLAssetExtractor) determineAssetType(selection *goquery.Selection, tagName string) models.AssetType {
	switch tagName {
	case "a":
		return models.AssetType(tagName)
	case "link":
		return hae.determineLinkAssetType(selection)
	case "script", "img", "iframe", "form", "object", "embed":
		return models.AssetType(tagName)
	default:
		hae.crawlerInstance.logger.Warn().
			Str("tag", tagName).
			Msg("Unknown tag type for asset extraction, using tag name as type")
		return models.AssetType(tagName)
	}
}

// determineLinkAssetType determines specific asset type for link elements
func (hae *HTMLAssetExtractor) determineLinkAssetType(selection *goquery.Selection) models.AssetType {
	if rel := selection.AttrOr("rel", ""); rel == "stylesheet" {
		return models.AssetTypeStyle
	}
	return models.AssetType("link")
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
func (hae *HTMLAssetExtractor) shouldSkipURL(url string) bool {
	if url == "" {
		return true
	}

	skipPrefixes := []string{"data:", "mailto:", "tel:", "javascript:"}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}

	return false
}

// resolveURL resolves a URL against the base page URL
func (hae *HTMLAssetExtractor) resolveURL(rawURL string) string {
	if hae.basePageURL != nil {
		return hae.resolveAgainstBase(rawURL)
	}

	return hae.validateAbsoluteURL(rawURL)
}

// resolveAgainstBase resolves URL against base page URL
func (hae *HTMLAssetExtractor) resolveAgainstBase(rawURL string) string {
	resolved, err := hae.basePageURL.Parse(rawURL)
	if err != nil {
		hae.crawlerInstance.logger.Debug().
			Str("url", rawURL).
			Str("base", hae.basePageURL.String()).
			Err(err).
			Msg("Failed to resolve URL against base")
		return ""
	}

	return resolved.String()
}

// validateAbsoluteURL validates and returns absolute URL
func (hae *HTMLAssetExtractor) validateAbsoluteURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || !parsed.IsAbs() {
		hae.crawlerInstance.logger.Debug().
			Str("url", rawURL).
			Msg("URL is relative but no base URL provided, or unparsable")
		return ""
	}

	return parsed.String()
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
		hae.crawlerInstance.DiscoverURL(absoluteURL, hae.basePageURL)
	}
}
