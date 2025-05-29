package extractor

import (
	"fmt"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"net/url"
	"regexp"
	"strings"
	"time"

	// fmt might be needed if unique urlTypes per regex are desired later
	// "fmt"

	"github.com/BishopFox/jsluice"
	"github.com/rs/zerolog"
)

// PathExtractor is responsible for extracting paths from content.
// It uses jsluice for JavaScript AST-based analysis and then applies
// custom regexes from config for a full-content scan.
type PathExtractor struct {
	logger        zerolog.Logger
	customRegexes []*regexp.Regexp // For manual full-content scanning
}

// NewPathExtractor creates a new PathExtractor.
// Custom regexes from config are compiled for manual scanning.
func NewPathExtractor(log zerolog.Logger, extractorCfg config.ExtractorConfig) (*PathExtractor, error) {
	pe := &PathExtractor{
		logger: log.With().Str("component", "PathExtractor").Logger(),
	}

	if len(extractorCfg.CustomRegexes) > 0 {
		pe.logger.Info().Int("count", len(extractorCfg.CustomRegexes)).Msg("Compiling custom regexes from configuration for manual full-content scan...")
		for _, regexStr := range extractorCfg.CustomRegexes {
			re, err := regexp.Compile(regexStr)
			if err != nil {
				pe.logger.Error().Err(err).Str("regex", regexStr).Msg("Failed to compile custom regex for manual scan, skipping.")
			} else {
				pe.customRegexes = append(pe.customRegexes, re)
			}
		}
		pe.logger.Info().Int("compiled_count", len(pe.customRegexes)).Msg("Finished compiling custom regexes for manual scan.")
	} else {
		pe.logger.Info().Msg("No custom regexes provided in configuration for manual scan.")
	}

	pe.logger.Info().Msg("PathExtractor initialized.")
	return pe, nil
}

// makeURLMatcher is a placeholder function to demonstrate how one might create
// a jsluice.URLMatcher for specific, future AST-based matching logic.
// It is NOT directly used by the custom regexes from the config in the current setup,
// as those are now handled by manual full-content scanning after jsluice analysis.
func makeURLMatcher(matcherName string, nodeType string, logicFunc func(n *jsluice.Node) *jsluice.URL, logger zerolog.Logger) jsluice.URLMatcher {
	logger.Debug().Str("matcher_name", matcherName).Str("node_type", nodeType).Msg("Placeholder makeURLMatcher called (not actively used for config regexes).")
	// Example of how to return a jsluice.URLMatcher struct.
	// The actual logicFunc would contain specific AST-based matching.
	return jsluice.URLMatcher{
		Type: nodeType, // e.g., "string", "assignment_expression", "call_expression"
		Fn: func(n *jsluice.Node) *jsluice.URL {
			// This is where specific logic for this placeholder matcher would go.
			// For example, call the passed logicFunc or implement inline.
			if logicFunc != nil {
				return logicFunc(n)
			}
			// Default placeholder behavior: find nothing.
			// logger.Trace().Str("matcher_name", matcherName).Msg("Placeholder matcher function executed.")
			return nil
		},
	}
}

// ExtractPaths uses jsluice for JavaScript AST-based analysis, then applies
// custom regexes from config for a full-content scan on the original content.
func (pe *PathExtractor) ExtractPaths(sourceURL string, content []byte, contentType string) ([]models.ExtractedPath, error) {
	var extractedPaths []models.ExtractedPath
	seenAbsPaths := make(map[string]struct{}) // To deduplicate absolute paths

	base, errURLParse := url.Parse(sourceURL)
	if errURLParse != nil {
		pe.logger.Error().Err(errURLParse).Str("source_url", sourceURL).Msg("Failed to parse sourceURL, cannot resolve relative paths robustly.")
		// Continue, but relative path resolution might be affected or impossible.
	}

	// --- Step 1: jsluice AST-based analysis (primarily for JavaScript) ---
	if strings.Contains(contentType, "javascript") || strings.HasSuffix(sourceURL, ".js") {
		pe.logger.Debug().Str("source_url", sourceURL).Msg("Analyzing JS content with jsluice (default matchers)...")
		analyzer := jsluice.NewAnalyzer(content)

		// Note: The loop for adding pe.customRegexes as jsluice matchers is REMOVED.
		// jsluice will use its built-in matchers.
		// If you have other specific AST-based matchers to add in the future,
		// you could do so here using makeURLMatcher, e.g.:
		// placeholderMatcher := makeURLMatcher("myFutureMatcher", "string", func(n *jsluice.Node) *jsluice.URL { return nil }, pe.logger)
		// analyzer.AddURLMatcher(placeholderMatcher)

		jsluiceResults := analyzer.GetURLs()
		pe.logger.Debug().Int("jsluice_default_url_count", len(jsluiceResults)).Msg("jsluice analysis (default matchers) finished.")

		for _, jsluiceRes := range jsluiceResults {
			rawPath := jsluiceRes.URL
			absoluteURL := rawPath

			if base != nil {
				resolved, resolveErr := urlhandler.ResolveURL(rawPath, base)
				if resolveErr == nil {
					absoluteURL = resolved
				} else {
					pe.logger.Warn().Err(resolveErr).Str("raw_path_jsluice", rawPath).Str("base_url", base.String()).Msg("Failed to resolve relative path from jsluice result, using original")
				}
			} else if !strings.HasPrefix(rawPath, "http://") && !strings.HasPrefix(rawPath, "https://") && !strings.HasPrefix(rawPath, "//") {
				pe.logger.Warn().Str("raw_path_jsluice", rawPath).Str("source_url", sourceURL).Msg("SourceURL failed to parse, and jsluice extracted path is relative. Path will be treated as is.")
			}

			if _, exists := seenAbsPaths[absoluteURL]; !exists {
				pathType := jsluiceRes.Type
				if pathType == "" {
					pathType = "jsluice_default_unknown_type"
				}
				codeContext := jsluiceRes.Source

				extractedPath := models.ExtractedPath{
					SourceURL:            sourceURL,
					ExtractedRawPath:     rawPath,
					ExtractedAbsoluteURL: absoluteURL,
					Context:              codeContext,
					Type:                 pathType,
					DiscoveryTimestamp:   time.Now(),
				}
				extractedPaths = append(extractedPaths, extractedPath)
				seenAbsPaths[absoluteURL] = struct{}{}
				pe.logger.Debug().Str("source_url", sourceURL).Str("absolute_url", absoluteURL).Str("type", pathType).Msg("Processed and added path from jsluice (default matchers)")
			} else {
				pe.logger.Debug().Str("absolute_url", absoluteURL).Msg("Skipping duplicate path from jsluice (default matchers).")
			}
		}
	} else {
		pe.logger.Debug().Str("source_url", sourceURL).Str("content_type", contentType).Msg("Content is not JavaScript, skipping jsluice AST-based analysis.")
	}

	// --- Step 2: Manual full-content scan using custom regexes from config ---
	if len(pe.customRegexes) > 0 && len(content) > 0 {
		contentStr := string(content)
		pe.logger.Debug().Int("custom_regex_count", len(pe.customRegexes)).Msg("Starting manual full-content scan with custom config regexes...")

		for i, customRegex := range pe.customRegexes {
			matches := customRegex.FindAllString(contentStr, -1)
			if len(matches) == 0 {
				continue
			}
			pe.logger.Debug().Str("regex", customRegex.String()).Int("match_count", len(matches)).Msg("Manual config regex found matches.")

			for _, match := range matches {
				rawPath := strings.TrimSpace(match) // Trim spaces from raw match
				if rawPath == "" {
					continue
				}
				absoluteURL := rawPath

				// Attempt to parse the matched string as a URL to validate it roughly
				// and to facilitate resolution if it's relative.
				parsedMatch, _ := url.Parse(rawPath) // Error ignored for now, focus on resolution

				if parsedMatch != nil && parsedMatch.Scheme != "" && parsedMatch.Host != "" {
					// Already an absolute URL from regex, use as is, but check host validity
					if !strings.Contains(parsedMatch.Host, ".") {
						pe.logger.Debug().Str("match", rawPath).Str("host", parsedMatch.Host).Msg("Manual regex match is absolute but host seems invalid (no dot), skipping.")
						continue
					}
					// absoluteURL is already rawPath and is absolute
				} else if base != nil { // If not absolute, try to resolve if base is available
					resolved, resolveErr := urlhandler.ResolveURL(rawPath, base)
					if resolveErr == nil {
						absoluteURL = resolved
					} else {
						pe.logger.Warn().Err(resolveErr).Str("raw_match_manual", rawPath).Str("base_url", base.String()).Msg("Failed to resolve manual regex match, using original match as absoluteURL")
						// absoluteURL remains rawPath
					}
				} else if !strings.HasPrefix(rawPath, "http://") && !strings.HasPrefix(rawPath, "https://") && !strings.HasPrefix(rawPath, "//") {
					// Base is nil (sourceURL parse failed) and regex match is relative, cannot resolve.
					pe.logger.Warn().Str("raw_match_manual", rawPath).Str("source_url", sourceURL).Msg("SourceURL failed to parse, and manual regex match is relative. Cannot resolve, skipping.")
					continue
				}

				// Final validation of the (potentially resolved) absoluteURL
				finalParsed, finalParseErr := url.Parse(absoluteURL)
				if finalParseErr != nil || finalParsed.Scheme == "" || finalParsed.Host == "" || !strings.Contains(finalParsed.Host, ".") {
					pe.logger.Debug().Str("absolute_url_candidate", absoluteURL).Err(finalParseErr).Msg("Skipping manual regex match: final URL is invalid or host seems malformed.")
					continue
				}

				if _, exists := seenAbsPaths[absoluteURL]; !exists {
					contextSnippet := ""
					matchStartIndex := strings.Index(contentStr, match)
					if matchStartIndex != -1 {
						start := matchStartIndex - 50
						if start < 0 {
							start = 0
						}
						end := matchStartIndex + len(match) + 50
						if end > len(contentStr) {
							end = len(contentStr)
						}
						contextSnippet = contentStr[start:end]
					}

					extractedPath := models.ExtractedPath{
						SourceURL:            sourceURL,
						ExtractedRawPath:     rawPath,
						ExtractedAbsoluteURL: absoluteURL,
						Context:              contextSnippet,
						Type:                 fmt.Sprintf("manual_config_regex_%d", i),
						DiscoveryTimestamp:   time.Now(),
					}
					extractedPaths = append(extractedPaths, extractedPath)
					seenAbsPaths[absoluteURL] = struct{}{}
					pe.logger.Debug().Str("source_url", sourceURL).Str("absolute_url", absoluteURL).Str("type", extractedPath.Type).Msg("Processed and added path from manual config regex")
				} else {
					pe.logger.Debug().Str("absolute_url", absoluteURL).Msg("Skipping duplicate path from manual config regex.")
				}
			}
		}
	} else if len(pe.customRegexes) > 0 && len(content) == 0 {
		pe.logger.Debug().Msg("Custom regexes configured, but content is empty. Skipping manual scan.")
	}

	pe.logger.Info().Str("source_url", sourceURL).Int("total_unique_extracted_count", len(extractedPaths)).Msg("Finished extracting paths (jsluice defaults + manual config regexes).")
	return extractedPaths, nil
}

// loadDefaultJSRegexes is no longer needed and can be removed.
// compileRegex is no longer needed and can be removed.
