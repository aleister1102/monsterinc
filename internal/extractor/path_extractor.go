package extractor

import (
	"bytes"
	"fmt"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/html"
	// urlhandler will be used by methods within this package, but not directly by the struct itself for now.
)

// compiledRegex holds a pre-compiled regex with its metadata.
// This is internal to PathExtractor.
type compiledRegex struct {
	name    string         // A descriptive name for the regex (e.g., "absolute_url", "custom_api_endpoint")
	pattern *regexp.Regexp // The compiled regular expression
	typeStr string         // The type to assign to models.ExtractedPath.Type if this regex matches
}

// PathExtractor is responsible for extracting paths and URLs from various content types.
type PathExtractor struct {
	logger            zerolog.Logger
	compiledJSRegexes []compiledRegex // Stores pre-compiled regexes for JS extraction
	extractJSComments bool            // Placeholder for future use, e.g., to also search in JS comments
}

// NewPathExtractor creates a new instance of PathExtractor.
// If customJSRegexStrings is empty or all patterns fail to compile, default regexes will be used.
func NewPathExtractor(log zerolog.Logger, customJSRegexStrings []string, jsComments bool) (*PathExtractor, error) {
	pe := &PathExtractor{
		logger:            log.With().Str("component", "PathExtractor").Logger(),
		extractJSComments: jsComments,
	}

	if len(customJSRegexStrings) > 0 {
		var successfullyCompiledRegexes []compiledRegex
		for _, patternStr := range customJSRegexStrings {
			// Each custom regex string should be the core pattern to find within quotes.
			// Example: If user provides `foo/bar/[a-z]+`, the effective regex becomes `(?:"|')(foo/bar/[a-z]+)(?:"|')`
			// We need to be careful about how these patterns are defined and used.
			// For simplicity, let's assume the provided pattern is already complete or designed to be wrapped.
			// Let's assume the custom pattern string itself is what should be captured.
			// The config should specify the full regex pattern if it's complex, or just the path part.
			// For now, let's assume the user provides the *capturing group* content.
			// A more robust solution might involve a config structure for regexes like: {name: "myApi", pattern: "/api/v3/(\w+)", type: "js_api_v3"}

			compiledPattern, err := regexp.Compile(patternStr) // Compile the user-provided pattern directly
			if err != nil {
				pe.logger.Error().Err(err).Str("custom_pattern", patternStr).Msg("Failed to compile custom JS regex string from config")
				// Optionally continue and skip this regex, or return an error for the whole constructor
				// For now, let's skip faulty regexes from config and rely on defaults if all fail.
				continue
			}
			successfullyCompiledRegexes = append(successfullyCompiledRegexes, compiledRegex{
				name:    fmt.Sprintf("custom_js_regex_%s", patternStr), // Create a name based on pattern
				pattern: compiledPattern,
				typeStr: "js_string_custom_regex", // Generic type for all custom regexes for now
			})
		}
		if len(successfullyCompiledRegexes) > 0 {
			pe.compiledJSRegexes = successfullyCompiledRegexes
			pe.logger.Info().Int("count", len(pe.compiledJSRegexes)).Msg("Loaded custom JS regexes from config")
		} else {
			pe.logger.Warn().Msg("No custom JS regexes were successfully compiled from config. Loading default JS regexes.")
			pe.loadDefaultJSRegexes()
		}
	} else {
		// No custom regexes provided, load defaults.
		pe.logger.Info().Msg("No custom JS regexes provided in config. Loading default JS regexes.")
		pe.loadDefaultJSRegexes()
	}

	return pe, nil
}

// loadDefaultJSRegexes populates the extractor with a default set of regexes for JS.
func (pe *PathExtractor) loadDefaultJSRegexes() {
	pe.compiledJSRegexes = []compiledRegex{
		{name: "absolute_url", pattern: regexp.MustCompile(`(?i)(?:"|')((?:https?|ftp):\/\/[^\s"']+)`), typeStr: "js_string_url_absolute"},
		{name: "scheme_relative_url", pattern: regexp.MustCompile(`(?i)(?:"|')(\/\/[^\s"']+)`), typeStr: "js_string_url_schemerel"},
		{name: "relative_path_api", pattern: regexp.MustCompile(`(?i)(?:"|')((?:\/api|\/v[1-9]\d*)\/[^"']+)`), typeStr: "js_string_path_api"},
		{name: "relative_path_common", pattern: regexp.MustCompile(`(?i)(?:"|')((?:\.\.?\/|\/|\w+\/)[^"',\s;\)\]\}\<\>]+)`), typeStr: "js_string_path_relative"},
		{name: "simple_abs_path", pattern: regexp.MustCompile(`(?i)(?:"|')(\/[^"',\s;\)\]\}\<\>]{2,})`), typeStr: "js_string_path_absolute"},
	}
	pe.logger.Debug().Int("count", len(pe.compiledJSRegexes)).Msg("Loaded default JS regexes for PathExtractor")
}

// ExtractPaths analyzes the given content and extracts all found paths/URLs.
// contentType can be "text/html" or "application/javascript".
func (pe *PathExtractor) ExtractPaths(sourceURL string, content []byte, contentType string) ([]models.ExtractedPath, error) {
	pe.logger.Info().Str("sourceURL", sourceURL).Str("contentType", contentType).Int("contentLength", len(content)).Msg("Attempting to extract paths")

	var extractedPaths []models.ExtractedPath

	baseParsedURL, err := url.Parse(sourceURL)
	if err != nil {
		pe.logger.Error().Err(err).Str("sourceURL", sourceURL).Msg("Failed to parse base source URL")
		return nil, fmt.Errorf("failed to parse base source URL '%s': %w", sourceURL, err)
	}

	switch contentType {
	case "text/html":
		pe.logger.Debug().Msg("Starting HTML parsing for path extraction")
		doc, err := html.Parse(bytes.NewReader(content))
		if err != nil {
			pe.logger.Error().Err(err).Msg("Failed to parse HTML content")
			return nil, fmt.Errorf("error parsing HTML: %w", err)
		}

		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode {
				var attributeName string
				var pathType string
				var isInlineScript bool = false

				switch n.Data {
				case "a", "link":
					attributeName = "href"
					pathType = "html_attr_" + n.Data
				case "img", "iframe", "embed": // script is handled separately now
					attributeName = "src"
					pathType = "html_attr_" + n.Data
				case "script":
					hasSrc := false
					for _, attr := range n.Attr {
						if attr.Key == "src" {
							hasSrc = true
							attributeName = "src" // Will be processed by the common attribute logic below
							pathType = "html_attr_script_src"
							break
						}
					}
					if !hasSrc {
						isInlineScript = true
					}
				case "form":
					attributeName = "action"
					pathType = "html_attr_form"
				case "object":
					attributeName = "data"
					pathType = "html_attr_object"
				}

				// Handle attributes for paths (href, src, action, data)
				if attributeName != "" {
					for _, attr := range n.Attr {
						if attr.Key == attributeName {
							rawPath := strings.TrimSpace(attr.Val)
							if rawPath == "" || strings.HasPrefix(rawPath, "javascript:") || strings.HasPrefix(rawPath, "mailto:") || strings.HasPrefix(rawPath, "tel:") || strings.HasPrefix(rawPath, "#") {
								continue
							}
							pe.logger.Debug().Str("tag", n.Data).Str(attributeName, rawPath).Msgf("Found %s attribute", attributeName)
							resolvedURL, err := urlhandler.ResolveURL(rawPath, baseParsedURL)
							if err != nil {
								pe.logger.Warn().Err(err).Str("rawPath", rawPath).Str("base", baseParsedURL.String()).Msg("Failed to resolve URL")
								continue
							}
							extractedPath := models.ExtractedPath{
								SourceURL:            sourceURL,
								ExtractedRawPath:     rawPath,
								ExtractedAbsoluteURL: resolvedURL,
								Context:              fmt.Sprintf("%s[%s]", n.Data, attributeName),
								Type:                 pathType,
								DiscoveryTimestamp:   time.Now().UTC(),
							}
							extractedPaths = append(extractedPaths, extractedPath)
							// No break here, a tag could have multiple attributes that are paths (though unlikely for these specific ones)
						}
					}
				}

				// Handle inline script content
				if isInlineScript {
					var scriptContent strings.Builder
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						if c.Type == html.TextNode {
							scriptContent.WriteString(c.Data)
						}
					}
					contentStr := strings.TrimSpace(scriptContent.String())
					if contentStr != "" {
						pe.logger.Debug().Int("inline_script_length", len(contentStr)).Msg("Found inline script content to process")
						// Recursively call ExtractPaths for this JavaScript content.
						inlineScriptPaths, err := pe.ExtractPaths(sourceURL, []byte(contentStr), "application/javascript")
						if err != nil {
							pe.logger.Error().Err(err).Str("source_of_inline_script", sourceURL).Msg("Error extracting paths from inline script content")
						} else if len(inlineScriptPaths) > 0 {
							// Enrich context for paths found in inline scripts
							for i := range inlineScriptPaths {
								inlineScriptPaths[i].Context = fmt.Sprintf("html_inline_script > %s", inlineScriptPaths[i].Context)
							}
							extractedPaths = append(extractedPaths, inlineScriptPaths...)
						}
					}
				}

			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(doc) // Start traversal

		// pe.logger.Debug().Msg("HTML content parsed successfully (traversal pending)") // Redundant now
	case "application/javascript", "text/javascript": // text/javascript is common too
		pe.logger.Debug().Msg("Starting JavaScript content analysis for path extraction")
		contentStr := string(content)

		// The local `regexes` variable is removed. We use `pe.compiledJSRegexes` directly.

		foundJSRawPaths := make(map[string]string) // To store raw path and its type to avoid duplicate processing

		for _, r := range pe.compiledJSRegexes { // Iterate over PathExtractor's compiled regexes
			// If the regex pattern in config is meant to be inside quotes, the regex engine needs to handle that.
			// The default regexes already look for quotes: `(?:"|')(` + path_pattern + `)(?:"|')`
			// If custom regexes from config are just `path_pattern`, they might not work as expected
			// unless they also include logic for quotes, or we wrap them.
			// Current assumption: customJSRegexStrings from config are patterns that ALREADY account for being inside strings OR are global finders.
			// For this iteration, let's assume the custom regexes are complete as provided by the user.
			// And they should have ONE capturing group for the path itself.
			matches := r.pattern.FindAllStringSubmatch(contentStr, -1)
			for _, match := range matches {
				if len(match) > 1 { // Ensure there's a capturing group for the path
					rawPath := strings.TrimSpace(match[1])
					if rawPath == "" || len(rawPath) < 2 { // Skip empty or very short strings
						continue
					}
					// Check if this raw path was already found by a more specific regex or itself
					if _, exists := foundJSRawPaths[rawPath]; exists {
						continue
					}
					foundJSRawPaths[rawPath] = r.typeStr
				}
			}
		}

		for rawPath, pathType := range foundJSRawPaths {
			pe.logger.Debug().Str("rawPath", rawPath).Str("type", pathType).Msg("Found potential path in JS content")
			resolvedURL, err := urlhandler.ResolveURL(rawPath, baseParsedURL) // baseParsedURL is from the HTML page or the JS file URL
			if err != nil {
				pe.logger.Warn().Err(err).Str("rawPath", rawPath).Str("base", baseParsedURL.String()).Msg("Failed to resolve URL from JS content")
				continue
			}

			extractedPath := models.ExtractedPath{
				SourceURL:            sourceURL, // This is the URL of the JS file itself, or the HTML page containing the inline JS
				ExtractedRawPath:     rawPath,
				ExtractedAbsoluteURL: resolvedURL,
				Context:              fmt.Sprintf("JS_string_literal (type: %s)", pathType),
				Type:                 pathType,
				DiscoveryTimestamp:   time.Now().UTC(),
			}
			extractedPaths = append(extractedPaths, extractedPath)
		}

		pe.logger.Debug().Int("num_potential_js_paths", len(foundJSRawPaths)).Msg("JavaScript content analysis finished")

	default:
		pe.logger.Warn().Str("contentType", contentType).Msg("Unsupported content type for path extraction")
		// Optionally return an error or an empty slice. For now, empty slice.
	}

	// Deduplicate paths based on ExtractedAbsoluteURL
	deduplicatedPaths := make([]models.ExtractedPath, 0, len(extractedPaths))
	seenAbsoluteURLs := make(map[string]struct{})

	for _, path := range extractedPaths {
		if _, seen := seenAbsoluteURLs[path.ExtractedAbsoluteURL]; !seen {
			deduplicatedPaths = append(deduplicatedPaths, path)
			seenAbsoluteURLs[path.ExtractedAbsoluteURL] = struct{}{}
		}
	}

	pe.logger.Info().Int("numPathsFoundInitial", len(extractedPaths)).Int("numPathsDeduplicated", len(deduplicatedPaths)).Msg("Path extraction attempt finished")
	return deduplicatedPaths, nil
}
