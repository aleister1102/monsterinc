package extractor

import (
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/BishopFox/jsluice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// PathExtractor is responsible for extracting paths from content, now using jsluice.
type PathExtractor struct {
	logger zerolog.Logger
}

// NewPathExtractor creates a new PathExtractor.
// customJSRegexStrings and jsComments are no longer used.
func NewPathExtractor(log zerolog.Logger) (*PathExtractor, error) {
	pe := &PathExtractor{
		logger: log.With().Str("component", "PathExtractor").Logger(),
	}
	pe.logger.Info().Msg("PathExtractor initialized using jsluice.")
	return pe, nil
}

var generalPlaceholderRegex = regexp.MustCompile(`\$\{[a-zA-Z0-9_]+\}`)

// makeGeneralTemplateMatcherForDomain creates a jsluice URL matcher for template-like URLs for a specific domain.
func makeGeneralTemplateMatcherForDomain(domain string, urlType string) func(*jsluice.Node) *jsluice.URL {
	pe := &PathExtractor{
		logger: log.With().Str("component", "PathExtractor").Logger(),
	}
	return func(n *jsluice.Node) *jsluice.URL {
		val := n.DecodedString()
		pe.logger.Debug().Str("val", val).Msg("makeGeneralTemplateMatcherForDomain")
		isHTTP := strings.Contains(val, "http://") || strings.Contains(val, "https://")

		if isHTTP && strings.Contains(val, domain) && generalPlaceholderRegex.MatchString(val) {
			// Replace all placeholders for parsing check, as url.Parse might fail on raw template.
			testURL := generalPlaceholderRegex.ReplaceAllString(val, "placeholder")
			parsed, err := url.Parse(testURL)
			if err == nil && parsed.Scheme != "" && (strings.Contains(parsed.Host, domain)) {
				// Check that the original string contained the host part, to avoid overly broad matches from ReplaceAllString
				if strings.Contains(val, parsed.Host) {
					return &jsluice.URL{URL: val, Type: urlType}
				}
			}
		}
		return nil
	}
}

// makeBaseURLMatcher creates a jsluice URL matcher for base URLs on known domains.
func makeBaseURLMatcher(domain string, urlType string) func(*jsluice.Node) *jsluice.URL {
	return func(n *jsluice.Node) *jsluice.URL {
		val := n.DecodedString()
		isHTTP := strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://")
		doesNotContainPlaceholder := !generalPlaceholderRegex.MatchString(val) // Ensure it's not a template URL

		if isHTTP && doesNotContainPlaceholder && strings.Contains(val, domain) {
			parsedURL, err := url.Parse(val)
			if err == nil && parsedURL.Scheme != "" && (parsedURL.Host == domain || strings.HasSuffix(parsedURL.Host, "."+domain)) {
				if parsedURL.Path == "" || parsedURL.Path == "/" {
					return &jsluice.URL{URL: val, Type: urlType}
				}
			}
		}
		return nil
	}
}

// ExtractPaths uses jsluice to extract URLs and paths from JavaScript content.
// It attempts to resolve relative URLs based on the sourceURL.
func (pe *PathExtractor) ExtractPaths(sourceURL string, content []byte, contentType string) ([]models.ExtractedPath, error) {
	if !strings.Contains(contentType, "javascript") && !strings.HasSuffix(sourceURL, ".js") {
		// jsluice is primarily for JavaScript; skip for other content types.
		// We might want to be more nuanced here in the future if jsluice handles HTML or other types.
		return []models.ExtractedPath{}, nil
	}

	pe.logger.Debug().Str("source_url", sourceURL).Msg("Analyzing content with jsluice")

	analyzer := jsluice.NewAnalyzer(content)

	// Add custom matchers
	// These domains are from the user's example; consider making this configurable.
	// TODO: Make this configurable.
	domains := []string{"myoas.com", "wanyol.com", "oppoit.com"}

	// TODO: Add more matchers.
	for _, domain := range domains {
		analyzer.AddURLMatcher(jsluice.URLMatcher{Type: "string", Fn: makeBaseURLMatcher(domain, "custom_base_"+strings.ReplaceAll(domain, ".", "_"))})
		analyzer.AddURLMatcher(jsluice.URLMatcher{Type: "string", Fn: makeGeneralTemplateMatcherForDomain(domain, "custom_tpl_"+strings.ReplaceAll(domain, ".", "_"))})
	}

	jsluiceURLs := analyzer.GetURLs()

	var extractedPaths []models.ExtractedPath
	base, err := url.Parse(sourceURL)
	if err != nil {
		pe.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Failed to parse sourceURL, cannot resolve relative paths")
		// Continue without resolving relative paths, or return error, depending on desired strictness.
		// For now, we'll let jsluice give us what it can, and they might be absolute already.
	}

	for _, jsluiceURL := range jsluiceURLs {
		rawPath := jsluiceURL.URL // This is the URL/path string jsluice extracted
		absoluteURL := rawPath    // Assume absolute first

		if base != nil { // Only try to resolve if base URL was parsed successfully
			resolved, resolveErr := urlhandler.ResolveURL(rawPath, base)
			if resolveErr == nil {
				absoluteURL = resolved
			} else {
				pe.logger.Warn().Err(resolveErr).Str("raw_path", rawPath).Str("base_url", base.String()).Msg("Failed to resolve relative path, using original")
			}
		}

		// jsluiceURL.Type gives context like "fetchArgument", "locationAssignment", etc.
		// jsluiceURL.Source provides the line of code.
		pathType := jsluiceURL.Type
		if pathType == "" {
			pathType = "jsluice_extracted_unknown_type" // Default if jsluice doesn't provide a specific type
		}

		codeContext := jsluiceURL.Source // jsluice.URL.Source should be populated by matchers

		extractedPath := models.ExtractedPath{
			SourceURL:            sourceURL,
			ExtractedRawPath:     rawPath,
			ExtractedAbsoluteURL: absoluteURL,
			Context:              codeContext,
			Type:                 pathType,
			DiscoveryTimestamp:   time.Now(),
		}
		extractedPaths = append(extractedPaths, extractedPath)
		pe.logger.Debug().Str("source_url", sourceURL).Str("raw_path", rawPath).Str("absolute_url", absoluteURL).Str("type", pathType).Str("context", codeContext).Msg("Extracted path with jsluice")
	}

	pe.logger.Info().Str("source_url", sourceURL).Int("count", len(extractedPaths)).Msg("Finished extracting paths with jsluice")
	return extractedPaths, nil
}

// loadDefaultJSRegexes is no longer needed and can be removed.
// compileRegex is no longer needed and can be removed.
