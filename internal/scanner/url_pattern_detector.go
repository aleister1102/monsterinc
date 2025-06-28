package scanner

import (
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// URLPatternDetector handles pattern detection for auto-calibrate functionality
type URLPatternDetector struct {
	config        config.AutoCalibrateConfig
	logger        zerolog.Logger
	patternCounts map[string]int
	patternMutex  sync.RWMutex
}

// NewURLPatternDetector creates a new pattern detector
func NewURLPatternDetector(config config.AutoCalibrateConfig, logger zerolog.Logger) *URLPatternDetector {
	return &URLPatternDetector{
		config:        config,
		logger:        logger,
		patternCounts: make(map[string]int),
	}
}

// ShouldSkipByPattern checks if URL should be skipped based on pattern similarity
func (upd *URLPatternDetector) ShouldSkipByPattern(normalizedURL string) bool {
	// Generate pattern for the URL
	pattern, err := upd.generateURLPattern(normalizedURL)
	if err != nil {
		upd.logger.Debug().Err(err).Str("url", normalizedURL).Msg("Failed to generate URL pattern")
		return false
	}

	// Check pattern count
	upd.patternMutex.RLock()
	currentCount := upd.patternCounts[pattern]
	upd.patternMutex.RUnlock()

	// If we've exceeded the limit for this pattern, skip
	if currentCount >= upd.config.MaxSimilarURLs {
		if upd.config.EnableSkipLogging {
			upd.logger.Info().
				Str("url", normalizedURL).
				Str("pattern", pattern).
				Int("current_count", currentCount).
				Int("max_similar", upd.config.MaxSimilarURLs).
				Msg("Skipping URL due to similar pattern (auto-calibrate)")
		}
		return true
	}

	// Record this pattern
	upd.patternMutex.Lock()
	upd.patternCounts[pattern]++
	upd.patternMutex.Unlock()

	return false
}

// generateURLPattern creates a pattern from a URL for similarity detection
func (upd *URLPatternDetector) generateURLPattern(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Build pattern: scheme://host/filtered_path?filtered_query#filtered_fragment
	var pattern strings.Builder

	// Scheme and host remain the same
	pattern.WriteString(parsedURL.Scheme)
	pattern.WriteString("://")
	pattern.WriteString(parsedURL.Host)

	// Filter path segments
	if parsedURL.Path != "" {
		filteredPath := upd.filterPathSegments(parsedURL.Path)
		pattern.WriteString(filteredPath)
	}

	// Filter query parameters
	if parsedURL.RawQuery != "" {
		filteredQuery := upd.filterQueryParameters(parsedURL.Query())
		if filteredQuery != "" {
			pattern.WriteString("?")
			pattern.WriteString(filteredQuery)
		}
	}

	// Filter fragment
	if parsedURL.Fragment != "" {
		if upd.isVariableFragment(parsedURL.Fragment) {
			pattern.WriteString("#<variable>")
		} else {
			pattern.WriteString("#")
			pattern.WriteString(parsedURL.Fragment)
		}
	}

	return pattern.String(), nil
}

// filterQueryParameters filters query parameters for pattern generation
func (upd *URLPatternDetector) filterQueryParameters(params url.Values) string {
	var filteredParams []string

	for key, values := range params {
		if upd.isIgnoredParameter(key) {
			continue
		}

		// For pattern matching, we only care about parameter names, not values
		// unless they appear to be static
		for _, value := range values {
			if upd.isVariableFragment(value) {
				filteredParams = append(filteredParams, key+"=<variable>")
			} else {
				filteredParams = append(filteredParams, key+"="+value)
			}
		}
	}

	return strings.Join(filteredParams, "&")
}

// isIgnoredParameter checks if a parameter should be ignored in pattern matching
func (upd *URLPatternDetector) isIgnoredParameter(paramName string) bool {
	for _, ignored := range upd.config.IgnoreParameters {
		if strings.EqualFold(paramName, ignored) {
			return true
		}
	}
	return false
}

// filterPathSegments filters path segments for pattern generation
func (upd *URLPatternDetector) filterPathSegments(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	var filteredSegments []string

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		// Check if this looks like a locale code
		if upd.config.AutoDetectLocales && upd.isLocaleCode(segment) {
			filteredSegments = append(filteredSegments, "<locale>")
			continue
		}

		// Check if this looks like a variable (ID, hash, etc.)
		if upd.isVariableFragment(segment) {
			filteredSegments = append(filteredSegments, "<variable>")
		} else {
			filteredSegments = append(filteredSegments, segment)
		}
	}

	if len(filteredSegments) == 0 {
		return "/"
	}

	return "/" + strings.Join(filteredSegments, "/")
}

// isLocaleCode checks if a segment appears to be a locale code
func (upd *URLPatternDetector) isLocaleCode(segment string) bool {
	// Check custom locale codes first
	for _, code := range upd.config.CustomLocaleCodes {
		if strings.EqualFold(segment, code) {
			return true
		}
	}

	// Check built-in locale codes
	return upd.isBuiltinLocaleCode(segment) ||
		upd.isISO639Language(segment) ||
		upd.isISO3166Country(segment) ||
		upd.isCompoundLocaleCode(segment) ||
		upd.isSpecialLocaleCode(segment)
}

// isBuiltinLocaleCode checks against common locale codes
func (upd *URLPatternDetector) isBuiltinLocaleCode(segment string) bool {
	commonLocales := []string{
		"en", "es", "fr", "de", "it", "pt", "ru", "ja", "ko", "zh",
		"ar", "hi", "tr", "pl", "nl", "sv", "da", "no", "fi", "cs",
		"hu", "ro", "bg", "hr", "sk", "sl", "et", "lv", "lt", "mt",
		"en-us", "en-gb", "en-ca", "en-au", "es-es", "es-mx", "fr-fr",
		"fr-ca", "de-de", "de-at", "de-ch", "it-it", "pt-pt", "pt-br",
		"zh-cn", "zh-tw", "zh-hk", "ja-jp", "ko-kr",
	}

	segment = strings.ToLower(segment)
	for _, locale := range commonLocales {
		if segment == locale {
			return true
		}
	}
	return false
}

// isISO639Language checks if segment is a valid ISO 639 language code
func (upd *URLPatternDetector) isISO639Language(segment string) bool {
	if len(segment) != 2 && len(segment) != 3 {
		return false
	}

	// Common ISO 639-1 codes (2 letters)
	iso639Codes := []string{
		"aa", "ab", "ae", "af", "ak", "am", "an", "ar", "as", "av",
		"ay", "az", "ba", "be", "bg", "bh", "bi", "bm", "bn", "bo",
		"br", "bs", "ca", "ce", "ch", "co", "cr", "cs", "cu", "cv",
		"cy", "da", "de", "dv", "dz", "ee", "el", "en", "eo", "es",
		"et", "eu", "fa", "ff", "fi", "fj", "fo", "fr", "fy", "ga",
		"gd", "gl", "gn", "gu", "gv", "ha", "he", "hi", "ho", "hr",
		"ht", "hu", "hy", "hz", "ia", "id", "ie", "ig", "ii", "ik",
		"io", "is", "it", "iu", "ja", "jv", "ka", "kg", "ki", "kj",
		"kk", "kl", "km", "kn", "ko", "kr", "ks", "ku", "kv", "kw",
		"ky", "la", "lb", "lg", "li", "ln", "lo", "lt", "lu", "lv",
		"mg", "mh", "mi", "mk", "ml", "mn", "mr", "ms", "mt", "my",
		"na", "nb", "nd", "ne", "ng", "nl", "nn", "no", "nr", "nv",
		"ny", "oc", "oj", "om", "or", "os", "pa", "pi", "pl", "ps",
		"pt", "qu", "rm", "rn", "ro", "ru", "rw", "sa", "sc", "sd",
		"se", "sg", "si", "sk", "sl", "sm", "sn", "so", "sq", "sr",
		"ss", "st", "su", "sv", "sw", "ta", "te", "tg", "th", "ti",
		"tk", "tl", "tn", "to", "tr", "ts", "tt", "tw", "ty", "ug",
		"uk", "ur", "uz", "ve", "vi", "vo", "wa", "wo", "xh", "yi",
		"yo", "za", "zh", "zu",
	}

	segment = strings.ToLower(segment)
	for _, code := range iso639Codes {
		if segment == code {
			return true
		}
	}
	return false
}

// isISO3166Country checks if segment is a valid ISO 3166 country code
func (upd *URLPatternDetector) isISO3166Country(segment string) bool {
	if len(segment) != 2 {
		return false
	}

	// Common ISO 3166-1 alpha-2 country codes
	iso3166Codes := []string{
		"ad", "ae", "af", "ag", "ai", "al", "am", "ao", "aq", "ar",
		"as", "at", "au", "aw", "ax", "az", "ba", "bb", "bd", "be",
		"bf", "bg", "bh", "bi", "bj", "bl", "bm", "bn", "bo", "bq",
		"br", "bs", "bt", "bv", "bw", "by", "bz", "ca", "cc", "cd",
		"cf", "cg", "ch", "ci", "ck", "cl", "cm", "cn", "co", "cr",
		"cu", "cv", "cw", "cx", "cy", "cz", "de", "dj", "dk", "dm",
		"do", "dz", "ec", "ee", "eg", "eh", "er", "es", "et", "fi",
		"fj", "fk", "fm", "fo", "fr", "ga", "gb", "gd", "ge", "gf",
		"gg", "gh", "gi", "gl", "gm", "gn", "gp", "gq", "gr", "gs",
		"gt", "gu", "gw", "gy", "hk", "hm", "hn", "hr", "ht", "hu",
		"id", "ie", "il", "im", "in", "io", "iq", "ir", "is", "it",
		"je", "jm", "jo", "jp", "ke", "kg", "kh", "ki", "km", "kn",
		"kp", "kr", "kw", "ky", "kz", "la", "lb", "lc", "li", "lk",
		"lr", "ls", "lt", "lu", "lv", "ly", "ma", "mc", "md", "me",
		"mf", "mg", "mh", "mk", "ml", "mm", "mn", "mo", "mp", "mq",
		"mr", "ms", "mt", "mu", "mv", "mw", "mx", "my", "mz", "na",
		"nc", "ne", "nf", "ng", "ni", "nl", "no", "np", "nr", "nu",
		"nz", "om", "pa", "pe", "pf", "pg", "ph", "pk", "pl", "pm",
		"pn", "pr", "ps", "pt", "pw", "py", "qa", "re", "ro", "rs",
		"ru", "rw", "sa", "sb", "sc", "sd", "se", "sg", "sh", "si",
		"sj", "sk", "sl", "sm", "sn", "so", "sr", "ss", "st", "sv",
		"sx", "sy", "sz", "tc", "td", "tf", "tg", "th", "tj", "tk",
		"tl", "tm", "tn", "to", "tr", "tt", "tv", "tw", "tz", "ua",
		"ug", "um", "us", "uy", "uz", "va", "vc", "ve", "vg", "vi",
		"vn", "vu", "wf", "ws", "ye", "yt", "za", "zm", "zw",
	}

	segment = strings.ToLower(segment)
	for _, code := range iso3166Codes {
		if segment == code {
			return true
		}
	}
	return false
}

// isCompoundLocaleCode checks for compound locale codes like en-US, zh-CN
func (upd *URLPatternDetector) isCompoundLocaleCode(segment string) bool {
	parts := strings.Split(segment, "-")
	if len(parts) != 2 {
		return false
	}

	// Check if first part is language and second is country
	return upd.isISO639Language(parts[0]) && upd.isISO3166Country(parts[1])
}

// isSpecialLocaleCode checks for special locale variations
func (upd *URLPatternDetector) isSpecialLocaleCode(segment string) bool {
	specialCodes := []string{
		"root", "default", "international", "worldwide", "global",
		"int", "www", "web", "mobile", "m", "api", "app",
	}

	segment = strings.ToLower(segment)
	for _, code := range specialCodes {
		if segment == code {
			return true
		}
	}
	return false
}

// isVariableFragment checks if a segment looks like a variable value
func (upd *URLPatternDetector) isVariableFragment(fragment string) bool {
	if len(fragment) == 0 {
		return false
	}

	// Check for common variable patterns
	if upd.isNumeric(fragment) {
		return true
	}

	if upd.isHexLike(fragment) {
		return true
	}

	// Check for UUID-like patterns
	if len(fragment) == 36 && strings.Count(fragment, "-") == 4 {
		return true
	}

	// Check for timestamp-like patterns
	if len(fragment) >= 8 && upd.isNumeric(fragment) {
		return true
	}

	// Check for encoded values (contains % encoding)
	if strings.Contains(fragment, "%") {
		return true
	}

	// Check for base64-like patterns (long alphanumeric strings)
	if len(fragment) > 16 && isAlphaNumeric(fragment) {
		return true
	}

	// Check for session-like patterns
	sessionPrefixes := []string{"sid", "session", "token", "auth", "key", "id", "uid", "ref"}
	for _, prefix := range sessionPrefixes {
		if strings.HasPrefix(strings.ToLower(fragment), prefix) && len(fragment) > len(prefix)+4 {
			return true
		}
	}

	return false
}

// isNumeric checks if a string contains only digits
func (upd *URLPatternDetector) isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// isHexLike checks if a string looks like hexadecimal
func (upd *URLPatternDetector) isHexLike(s string) bool {
	if len(s) < 8 { // Must be at least 8 characters to be considered hex-like
		return false
	}
	_, err := strconv.ParseInt(s, 16, 64)
	return err == nil
}

// isAlphaNumeric checks if string contains only alphanumeric characters
func isAlphaNumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// GetPatternStats returns statistics about detected patterns
func (upd *URLPatternDetector) GetPatternStats() map[string]int {
	upd.patternMutex.RLock()
	defer upd.patternMutex.RUnlock()

	// Return a copy to avoid race conditions
	stats := make(map[string]int)
	for pattern, count := range upd.patternCounts {
		stats[pattern] = count
	}
	return stats
}

// Reset clears all pattern tracking data
func (upd *URLPatternDetector) Reset() {
	upd.patternMutex.Lock()
	defer upd.patternMutex.Unlock()
	upd.patternCounts = make(map[string]int)

	upd.logger.Debug().Msg("URL pattern detector reset")
}
