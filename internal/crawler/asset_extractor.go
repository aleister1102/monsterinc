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

	// Check if URL is in scope before discovering for crawling
	if hae.crawlerInstance != nil && hae.crawlerInstance.scope != nil {
		isAllowed, err := hae.crawlerInstance.scope.IsURLAllowed(absoluteURL)
		if err != nil {
			hae.crawlerInstance.logger.Debug().
				Str("url", absoluteURL).
				Err(err).
				Msg("Asset URL scope check failed")
			// Still create asset for reporting but don't discover for crawling
		} else if !isAllowed {
			hae.crawlerInstance.logger.Debug().
				Str("url", absoluteURL).
				Str("source_page", baseDiscoveryURL).
				Msg("Asset URL not in scope, skipping crawl discovery")
			// Create asset for reporting but don't discover for crawling
			return &models.Asset{
				AbsoluteURL:    absoluteURL,
				SourceTag:      hae.getTagName(selection),
				SourceAttr:     attributeName,
				Type:           assetType,
				DiscoveredAt:   time.Now(),
				DiscoveredFrom: baseDiscoveryURL,
			}
		}
	}

	// Discover URL for crawling only if in scope
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

	// Additional checks for problematic URLs
	if hae.hasProblematicPatterns(urlStr) {
		hae.crawlerInstance.logger.Warn().
			Str("url", urlStr).
			Msg("Skipping URL with problematic patterns")
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

	// Check if we have more than 15 segments (reduced threshold)
	if len(segments) > 15 {
		hae.crawlerInstance.logger.Warn().
			Str("url", urlStr).
			Int("segment_count", len(segments)).
			Msg("URL has excessive path segments, likely infinite loop")
		return true
	}

	// Check for any segment appearing more than 3 times in the path (reduced threshold)
	segmentCount := make(map[string]int)
	for _, segment := range segments {
		if segment != "" && len(segment) >= 2 {
			segmentCount[segment]++
			if segmentCount[segment] > 3 {
				hae.crawlerInstance.logger.Warn().
					Str("url", urlStr).
					Str("repeated_segment", segment).
					Int("count", segmentCount[segment]).
					Msg("URL has repeated path segment")
				return true
			}
		}
	}

	// Check for obvious infinite loops - 2+ consecutive identical segments
	// (reduced from 3 to be more sensitive)
	for i := 0; i < len(segments)-1; i++ {
		if len(segments[i]) >= 2 && segments[i] != "" &&
			segments[i] == segments[i+1] {
			hae.crawlerInstance.logger.Warn().
				Str("url", urlStr).
				Str("repeated_segment", segments[i]).
				Msg("URL has consecutive repeated path segments")
			return true
		}
	}

	// Check for very long individual path segments (reduced threshold)
	for _, segment := range segments {
		if len(segment) > 200 {
			hae.crawlerInstance.logger.Warn().
				Str("url", urlStr).
				Str("long_segment", segment[:50]+"...").
				Int("segment_length", len(segment)).
				Msg("URL has extremely long path segment")
			return true
		}
	}

	// Check for specific patterns that indicate infinite loops
	// Pattern 1: Same segment appears at beginning and end
	if len(segments) > 3 {
		firstSegment := segments[0]
		lastSegment := segments[len(segments)-1]
		if firstSegment != "" && firstSegment == lastSegment && len(firstSegment) >= 2 {
			hae.crawlerInstance.logger.Warn().
				Str("url", urlStr).
				Str("repeated_segment", firstSegment).
				Msg("URL has same segment at beginning and end")
			return true
		}
	}

	// Pattern 2: Check for alternating patterns like /a/b/a/b/a/b
	if len(segments) >= 6 {
		for i := 0; i < len(segments)-5; i++ {
			if segments[i] != "" && len(segments[i]) >= 2 &&
				segments[i] == segments[i+2] && segments[i] == segments[i+4] &&
				segments[i+1] == segments[i+3] && segments[i+1] == segments[i+5] {
				hae.crawlerInstance.logger.Warn().
					Str("url", urlStr).
					Str("pattern", segments[i]+"/"+segments[i+1]).
					Msg("URL has alternating pattern")
				return true
			}
		}
	}

	return false
}

// hasProblematicPatterns checks for additional problematic URL patterns
func (hae *HTMLAssetExtractor) hasProblematicPatterns(urlStr string) bool {
	// Check for extremely long URLs (potential issue)
	if len(urlStr) > 2000 {
		hae.crawlerInstance.logger.Warn().
			Str("url", urlStr[:100]+"...").
			Int("length", len(urlStr)).
			Msg("URL is extremely long")
		return true
	}

	// Check for URLs with excessive slashes
	slashCount := strings.Count(urlStr, "/")
	if slashCount > 50 {
		hae.crawlerInstance.logger.Warn().
			Str("url", urlStr).
			Int("slash_count", slashCount).
			Msg("URL has excessive slashes")
		return true
	}

	// Check for URLs that contain the same path fragment multiple times
	// This is a simplified check for patterns like /js/js/js/...
	if strings.Contains(urlStr, "/js/js/") ||
		strings.Contains(urlStr, "/css/css/") ||
		strings.Contains(urlStr, "/img/img/") ||
		strings.Contains(urlStr, "/static/static/") ||
		strings.Contains(urlStr, "/assets/assets/") {
		hae.crawlerInstance.logger.Warn().
			Str("url", urlStr).
			Msg("URL contains repeated path fragments")
		return true
	}

	// Check for URLs with recursive directory patterns
	recursivePatterns := []string{
		"/../../../", "/./././", "/./../", "/../../",
	}
	for _, pattern := range recursivePatterns {
		if strings.Contains(urlStr, pattern) {
			hae.crawlerInstance.logger.Warn().
				Str("url", urlStr).
				Str("pattern", pattern).
				Msg("URL contains recursive directory pattern")
			return true
		}
	}

	return false
}

// resolveURL resolves a URL against the base page URL
func (hae *HTMLAssetExtractor) resolveURL(rawURL string) string {
	// Early validation to skip problematic URLs before resolution
	if hae.isInvalidURLPattern(rawURL) {
		hae.crawlerInstance.logger.Debug().
			Str("url", rawURL).
			Msg("Skipping URL with invalid pattern before resolution")
		return ""
	}

	// Use urlhandler.ResolveURL for consistent URL resolution
	resolved, err := urlhandler.ResolveURL(rawURL, hae.basePageURL)
	if err != nil {
		hae.crawlerInstance.logger.Debug().
			Str("url", rawURL).
			Err(err).
			Msg("Failed to resolve URL using urlhandler")
		return ""
	}

	// Final validation after resolution
	if hae.isInvalidURLPattern(resolved) {
		hae.crawlerInstance.logger.Debug().
			Str("resolved_url", resolved).
			Msg("Skipping resolved URL with invalid pattern")
		return ""
	}

	return resolved
}

// isInvalidURLPattern performs early detection of invalid URL patterns
func (hae *HTMLAssetExtractor) isInvalidURLPattern(urlStr string) bool {
	if urlStr == "" {
		return true
	}

	// Skip URLs that are too long (potential issue)
	if len(urlStr) > 1000 {
		return true
	}

	// Skip URLs with obvious repeated patterns immediately
	// Check for simple patterns like /js/js/ or /css/css/
	if strings.Contains(urlStr, "/js/js/") ||
		strings.Contains(urlStr, "/css/css/") ||
		strings.Contains(urlStr, "/img/img/") ||
		strings.Contains(urlStr, "/static/static/") ||
		strings.Contains(urlStr, "/assets/assets/") {
		return true
	}

	// Check for excessive slashes (early detection)
	slashCount := strings.Count(urlStr, "/")
	if slashCount > 30 {
		return true
	}

	// Check for recursive patterns
	recursivePatterns := []string{
		"/../../../", "/./././", "/./../",
	}
	for _, pattern := range recursivePatterns {
		if strings.Contains(urlStr, pattern) {
			return true
		}
	}

	// Check for URLs that are just repeated characters/patterns
	if hae.hasObviousRepeatedChars(urlStr) {
		return true
	}

	return false
}

// hasObviousRepeatedChars checks for URLs with obviously repeated character patterns
func (hae *HTMLAssetExtractor) hasObviousRepeatedChars(urlStr string) bool {
	// Extract just the path part
	if idx := strings.Index(urlStr, "//"); idx != -1 {
		// Find the path part after hostname
		remaining := urlStr[idx+2:]
		if pathIdx := strings.Index(remaining, "/"); pathIdx != -1 {
			urlStr = remaining[pathIdx:]
		}
	}

	// Check for patterns like /js/js/js/js/...
	segments := strings.Split(strings.Trim(urlStr, "/"), "/")
	if len(segments) > 5 {
		// Count occurrences of each segment
		segmentCount := make(map[string]int)
		for _, segment := range segments {
			if segment != "" && len(segment) >= 2 {
				segmentCount[segment]++
			}
		}

		// If any segment appears more than 3 times, it's suspicious
		for _, count := range segmentCount {
			if count > 3 {
				return true
			}
		}
	}

	return false
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
