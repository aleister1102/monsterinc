package crawler

import (
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// URLPatternDetector detects similar URL patterns and implements auto-calibrate logic
type URLPatternDetector struct {
	config        config.AutoCalibrateConfig
	logger        zerolog.Logger
	patternCounts map[string]int
	patternMutex  sync.RWMutex
	seenURLs      map[string]bool
	urlMutex      sync.RWMutex
}

// NewURLPatternDetector creates a new URL pattern detector
func NewURLPatternDetector(config config.AutoCalibrateConfig, logger zerolog.Logger) *URLPatternDetector {
	return &URLPatternDetector{
		config:        config,
		logger:        logger.With().Str("component", "URLPatternDetector").Logger(),
		patternCounts: make(map[string]int),
		seenURLs:      make(map[string]bool),
	}
}

// ShouldSkipURL determines if a URL should be skipped based on pattern similarity
func (upd *URLPatternDetector) ShouldSkipURL(rawURL string) bool {
	if !upd.config.Enabled {
		return false
	}

	// Check if URL was already seen
	upd.urlMutex.RLock()
	if upd.seenURLs[rawURL] {
		upd.urlMutex.RUnlock()
		return true
	}
	upd.urlMutex.RUnlock()

	// Generate pattern for the URL
	pattern, err := upd.generateURLPattern(rawURL)
	if err != nil {
		upd.logger.Debug().Err(err).Str("url", rawURL).Msg("Failed to generate URL pattern")
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
				Str("url", rawURL).
				Str("pattern", pattern).
				Int("current_count", currentCount).
				Int("max_similar", upd.config.MaxSimilarURLs).
				Msg("Skipping URL due to similar pattern (auto-calibrate)")
		}

		// Mark URL as seen
		upd.urlMutex.Lock()
		upd.seenURLs[rawURL] = true
		upd.urlMutex.Unlock()

		return true
	}

	// Record this URL and increment pattern count
	upd.patternMutex.Lock()
	upd.patternCounts[pattern]++
	upd.patternMutex.Unlock()

	upd.urlMutex.Lock()
	upd.seenURLs[rawURL] = true
	upd.urlMutex.Unlock()

	return false
}

// generateURLPattern generates a pattern string for a URL by removing ignored parameters and path segments
func (upd *URLPatternDetector) generateURLPattern(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Create pattern from scheme, host, port
	pattern := parsedURL.Scheme + "://" + parsedURL.Host

	// Process path with ignored segments
	filteredPath := upd.filterPathSegments(parsedURL.Path)
	pattern += filteredPath

	// Process query parameters
	if parsedURL.RawQuery != "" {
		filteredParams := upd.filterQueryParameters(parsedURL.Query())
		if len(filteredParams) > 0 {
			pattern += "?" + filteredParams
		}
	}

	// Add fragment if present (but not variable parts)
	if parsedURL.Fragment != "" && !upd.isVariableFragment(parsedURL.Fragment) {
		pattern += "#" + parsedURL.Fragment
	}

	return pattern, nil
}

// filterQueryParameters filters out ignored parameters and returns normalized query string
func (upd *URLPatternDetector) filterQueryParameters(params url.Values) string {
	var filteredPairs []string

	for key, values := range params {
		if !upd.isIgnoredParameter(key) {
			for range values {
				// For pattern matching, we don't care about the actual value
				// Just whether the parameter exists
				filteredPairs = append(filteredPairs, key+"=*")
			}
		}
	}

	// Sort for consistent pattern generation
	sort.Strings(filteredPairs)
	return strings.Join(filteredPairs, "&")
}

// isIgnoredParameter checks if a parameter should be ignored for pattern matching
func (upd *URLPatternDetector) isIgnoredParameter(paramName string) bool {
	for _, ignored := range upd.config.IgnoreParameters {
		if strings.EqualFold(paramName, ignored) {
			return true
		}
	}
	return false
}

// filterPathSegments filters out locale codes in path segments and returns normalized path
func (upd *URLPatternDetector) filterPathSegments(path string) string {
	if !upd.config.AutoDetectLocales {
		return path
	}

	// Split path into segments
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 0 || (len(segments) == 1 && segments[0] == "") {
		return "/"
	}

	// Create filtered segments array
	filteredSegments := make([]string, 0, len(segments))

	for _, segment := range segments {
		if upd.isLocaleCode(segment) {
			// Replace detected locale codes with wildcard
			filteredSegments = append(filteredSegments, "*")
		} else {
			filteredSegments = append(filteredSegments, segment)
		}
	}

	// Rebuild path
	result := "/" + strings.Join(filteredSegments, "/")
	if path != "/" && strings.HasSuffix(path, "/") {
		result += "/"
	}

	return result
}

// isLocaleCode checks if a path segment is likely a locale/language code
func (upd *URLPatternDetector) isLocaleCode(segment string) bool {
	if segment == "" {
		return false
	}

	// Check custom locale codes first
	for _, customCode := range upd.config.CustomLocaleCodes {
		if strings.EqualFold(segment, customCode) {
			return true
		}
	}

	// Check built-in locale patterns
	return upd.isBuiltinLocaleCode(segment)
}

// isBuiltinLocaleCode checks against common locale code patterns
func (upd *URLPatternDetector) isBuiltinLocaleCode(segment string) bool {
	segment = strings.ToLower(segment)

	// ISO 639-1 language codes or ISO 3166 country codes (2 letters)
	if len(segment) == 2 {
		return upd.isISO639Language(segment) || upd.isISO3166Country(segment)
	}

	// Language-Country codes (e.g., en-us, zh-cn, pt-br)
	if len(segment) == 5 && segment[2] == '-' {
		langCode := segment[:2]
		countryCode := segment[3:]
		return upd.isISO639Language(langCode) && upd.isISO3166Country(countryCode)
	}

	// Compound locale codes (e.g., chde, chfr, kzkk, jo-ar, jo-en, benl, befr)
	if len(segment) == 4 {
		return upd.isCompoundLocaleCode(segment)
	}

	// Special locale codes with dash (e.g., jo-ar, jo-en)
	if len(segment) > 3 && strings.Contains(segment, "-") {
		return upd.isSpecialLocaleCode(segment)
	}

	return false
}

// isISO639Language checks if it's a valid ISO 639-1 language code
func (upd *URLPatternDetector) isISO639Language(code string) bool {
	// Common ISO 639-1 language codes
	languages := map[string]bool{
		"en": true, "de": true, "fr": true, "es": true, "it": true, "pt": true,
		"nl": true, "ru": true, "ja": true, "ko": true, "zh": true, "ar": true,
		"tr": true, "pl": true, "cs": true, "hu": true, "fi": true, "da": true,
		"sv": true, "no": true, "el": true, "he": true, "th": true, "vi": true,
		"id": true, "ms": true, "hi": true, "bn": true, "ur": true, "fa": true,
		"uk": true, "bg": true, "hr": true, "sk": true, "sl": true, "et": true,
		"lv": true, "lt": true, "ro": true, "sr": true, "mk": true, "sq": true,
		"is": true, "mt": true, "ga": true, "cy": true, "eu": true, "ca": true,
		"gl": true, "ast": true, "oc": true, "co": true, "br": true, "gd": true,
		"kw": true, "gv": true, "fo": true, "nn": true, "nb": true, "se": true,
		"be": true, "kk": true, "ky": true, "uz": true, "tk": true, "mn": true,
		"bo": true, "my": true, "km": true, "lo": true, "si": true, "ta": true,
		"te": true, "kn": true, "ml": true, "or": true, "gu": true, "pa": true,
		"as": true, "ne": true, "dz": true, "am": true, "ti": true, "om": true,
		"so": true, "sw": true, "rw": true, "rn": true, "ny": true, "sn": true,
		"st": true, "tn": true, "ts": true, "ss": true, "ve": true, "xh": true,
		"zu": true, "af": true, "nso": true, "yo": true, "ig": true, "ha": true,
		"ff": true, "wo": true, "bm": true, "ee": true, "tw": true, "ak": true,
		"lg": true, "ln": true, "kg": true, "sg": true, "za": true, "nd": true,
		"nr": true,
	}
	return languages[code]
}

// isISO3166Country checks if it's a valid ISO 3166-1 country code
func (upd *URLPatternDetector) isISO3166Country(code string) bool {
	// Common ISO 3166-1 country codes
	countries := map[string]bool{
		"us": true, "gb": true, "ca": true, "au": true, "de": true, "fr": true,
		"es": true, "it": true, "pt": true, "nl": true, "be": true, "at": true,
		"ch": true, "dk": true, "se": true, "no": true, "fi": true, "is": true,
		"ie": true, "pl": true, "cz": true, "sk": true, "hu": true, "si": true,
		"hr": true, "ba": true, "rs": true, "me": true, "mk": true, "al": true,
		"bg": true, "ro": true, "md": true, "ua": true, "by": true, "ru": true,
		"lt": true, "lv": true, "ee": true, "jp": true, "kr": true, "cn": true,
		"tw": true, "hk": true, "mo": true, "sg": true, "my": true, "th": true,
		"vn": true, "ph": true, "id": true, "bn": true, "mm": true, "kh": true,
		"la": true, "in": true, "pk": true, "af": true, "ir": true, "iq": true,
		"sy": true, "lb": true, "jo": true, "il": true, "ps": true, "sa": true,
		"ye": true, "om": true, "ae": true, "qa": true, "bh": true, "kw": true,
		"tr": true, "cy": true, "ge": true, "am": true, "az": true, "kz": true,
		"kg": true, "tj": true, "tm": true, "uz": true, "mn": true, "np": true,
		"bt": true, "lk": true, "mv": true, "eg": true, "ly": true, "sd": true,
		"tn": true, "dz": true, "ma": true, "mr": true, "ml": true, "bf": true,
		"ne": true, "td": true, "ng": true, "cm": true, "cf": true, "gq": true,
		"ga": true, "cg": true, "cd": true, "ao": true, "zm": true, "mw": true,
		"mz": true, "mg": true, "mu": true, "sc": true, "km": true, "dj": true,
		"so": true, "et": true, "er": true, "ke": true, "ug": true, "tz": true,
		"rw": true, "bi": true, "za": true, "bw": true, "na": true, "sz": true,
		"ls": true, "zw": true, "br": true, "ar": true, "uy": true, "py": true,
		"bo": true, "pe": true, "ec": true, "co": true, "ve": true, "gy": true,
		"sr": true, "gf": true, "cl": true, "mx": true, "gt": true, "bz": true,
		"sv": true, "hn": true, "ni": true, "cr": true, "pa": true, "cu": true,
		"jm": true, "ht": true, "do": true, "pr": true, "vi": true, "ag": true,
		"dm": true, "lc": true, "vc": true, "gd": true, "bb": true, "tt": true,
		"aw": true, "cw": true, "sx": true, "bq": true, "ms": true, "ai": true,
		"kn": true, "bs": true, "tc": true, "vg": true, "ky": true, "bm": true,
	}
	return countries[code]
}

// isCompoundLocaleCode checks compound locale codes like chde, chfr, kzkk, etc.
func (upd *URLPatternDetector) isCompoundLocaleCode(code string) bool {
	// Common compound locale codes
	compounds := map[string]bool{
		"chde": true, "chfr": true, "chit": true, // Switzerland
		"kzkk": true, "kzru": true, // Kazakhstan
		"benl": true, "befr": true, "bede": true, // Belgium
		"caen": true, "cafr": true, // Canada
		"lufr": true, "lude": true, "lulb": true, // Luxembourg
		"inen": true, "inhi": true, // India
		"pken": true, "pkur": true, // Pakistan
		"bden": true, "bdbn": true, // Bangladesh
		"lken": true, "lksi": true, "lkta": true, // Sri Lanka
		"npen": true, "npne": true, // Nepal
		"myen": true, "mymy": true, // Myanmar
		"khen": true, "khkm": true, // Cambodia
		"laen": true, "lalo": true, // Laos
		"mmen": true, "mmmy": true, // Myanmar
		"mden": true, "mdru": true, "mdro": true, // Moldova
		"uaen": true, "uaru": true, "uauk": true, // Ukraine
		"byru": true, "byen": true, "byby": true, // Belarus
		"kzen": true,                             // Kazakhstan alternative
		"kgen": true, "kgru": true, "kgky": true, // Kyrgyzstan
		"tjen": true, "tjru": true, "tjtg": true, // Tajikistan
		"tmen": true, "tmru": true, "tmtk": true, // Turkmenistan
		"uzen": true, "uzru": true, "uzuz": true, // Uzbekistan
		"mnen": true, "mnru": true, "mnmn": true, // Mongolia
		"egen": true, "egar": true, // Egypt
		"tnen": true, "tnar": true, "tnfr": true, // Tunisia
		"dzen": true, "dzar": true, "dzfr": true, // Algeria
		"maen": true, "maar": true, "mafr": true, // Morocco
		"mren": true, "mrar": true, "mrfr": true, // Mauritania
		"mlen": true, "mlfr": true, // Mali
		"bfen": true, "bffr": true, // Burkina Faso
		"neen": true, "nefr": true, // Niger
		"tden": true, "tdfr": true, "tdar": true, // Chad
		"ngen": true, "ngha": true, "ngig": true, // Nigeria
		"cmen": true, "cmfr": true, // Cameroon
		"cfen": true, "cffr": true, // Central African Republic
		"gqen": true, "gqes": true, "gqfr": true, // Equatorial Guinea
		"gaen": true, "gafr": true, // Gabon
		"cgen": true, "cgfr": true, // Congo
		"cden": true, "cdfr": true, // DR Congo
		"aoen": true, "aopt": true, // Angola
		"zmen": true, "zmny": true, // Zambia
		"mwen": true, "mwny": true, // Malawi
		"mzen": true, "mzpt": true, // Mozambique
		"mgen": true, "mgfr": true, "mgmg": true, // Madagascar
		"muen": true, "mufr": true, // Mauritius
		"scen": true, "scfr": true, // Seychelles
		"kmen": true, "kmar": true, "kmfr": true, // Comoros
		"djen": true, "djar": true, "djfr": true, // Djibouti
		"soen": true, "soar": true, // Somalia
		"eten": true, "etar": true, "etam": true, // Ethiopia
		"eren": true, "erar": true, "erti": true, // Eritrea
		"keen": true, "kesw": true, // Kenya
		"ugen": true, "ugsw": true, // Uganda
		"tzen": true, "tzsw": true, // Tanzania
		"rwen": true, "rwrw": true, "rwfr": true, // Rwanda
		"bien": true, "birn": true, "bifr": true, // Burundi
		"zaen": true, "zaaf": true, "zazu": true, // South Africa
		"bwen": true, "bwtn": true, // Botswana
		"naen": true, "naaf": true, // Namibia
		"szen": true, "szss": true, // Eswatini
		"lsen": true, "lsst": true, // Lesotho
		"zwen": true, "zwsn": true, // Zimbabwe
	}
	return compounds[code]
}

// isSpecialLocaleCode checks special locale codes with dashes
func (upd *URLPatternDetector) isSpecialLocaleCode(code string) bool {
	// Special locale codes with dashes
	specials := map[string]bool{
		"jo-ar": true, "jo-en": true, // Jordan
		"ae-ar": true, "ae-en": true, // UAE
		"sa-ar": true, "sa-en": true, // Saudi Arabia
		"kw-ar": true, "kw-en": true, // Kuwait
		"qa-ar": true, "qa-en": true, // Qatar
		"bh-ar": true, "bh-en": true, // Bahrain
		"om-ar": true, "om-en": true, // Oman
		"ye-ar": true, "ye-en": true, // Yemen
		"iq-ar": true, "iq-en": true, // Iraq
		"sy-ar": true, "sy-en": true, // Syria
		"lb-ar": true, "lb-en": true, // Lebanon
		"ps-ar": true, "ps-en": true, // Palestine
		"eg-ar": true, "eg-en": true, // Egypt
		"ly-ar": true, "ly-en": true, // Libya
		"sd-ar": true, "sd-en": true, // Sudan
		"tn-ar": true, "tn-fr": true, // Tunisia
		"dz-ar": true, "dz-fr": true, // Algeria
		"ma-ar": true, "ma-fr": true, // Morocco
		"mr-ar": true, "mr-fr": true, // Mauritania
		"td-ar": true, "td-fr": true, // Chad
		"dj-ar": true, "dj-fr": true, // Djibouti
		"km-ar": true, "km-fr": true, // Comoros
		"zh-cn": true, "zh-tw": true, "zh-hk": true, "zh-mo": true, // Chinese
		"pt-br": true, "pt-pt": true, // Portuguese
		"es-es": true, "es-mx": true, "es-ar": true, "es-co": true, // Spanish
		"fr-fr": true, "fr-ca": true, "fr-be": true, "fr-ch": true, // French
		"en-us": true, "en-gb": true, "en-ca": true, "en-au": true, // English
		"de-de": true, "de-at": true, "de-ch": true, // German
		"it-it": true, "it-ch": true, // Italian
		"nl-nl": true, "nl-be": true, // Dutch
		"sv-se": true, "sv-fi": true, // Swedish
		"no-no": true, "no-bv": true, // Norwegian
		"da-dk": true, "da-gl": true, // Danish
		"fi-fi": true, "fi-se": true, // Finnish
		"is-is": true,                // Icelandic
		"ga-ie": true,                // Irish
		"gd-gb": true,                // Scottish Gaelic
		"cy-gb": true,                // Welsh
		"eu-es": true,                // Basque
		"ca-es": true, "ca-ad": true, // Catalan
		"gl-es": true,                // Galician
		"mt-mt": true,                // Maltese
		"sq-al": true, "sq-mk": true, // Albanian
		"mk-mk": true,                // Macedonian
		"bg-bg": true,                // Bulgarian
		"ro-ro": true, "ro-md": true, // Romanian
		"hr-hr": true,                               // Croatian
		"sr-rs": true, "sr-me": true, "sr-ba": true, // Serbian
		"bs-ba": true,                                              // Bosnian
		"sl-si": true,                                              // Slovenian
		"sk-sk": true,                                              // Slovak
		"cs-cz": true,                                              // Czech
		"pl-pl": true,                                              // Polish
		"hu-hu": true,                                              // Hungarian
		"et-ee": true,                                              // Estonian
		"lv-lv": true,                                              // Latvian
		"lt-lt": true,                                              // Lithuanian
		"be-by": true,                                              // Belarusian
		"uk-ua": true,                                              // Ukrainian
		"ru-ru": true, "ru-by": true, "ru-kz": true, "ru-kg": true, // Russian
		"kk-kz": true,                // Kazakh
		"ky-kg": true,                // Kyrgyz
		"uz-uz": true,                // Uzbek
		"tk-tm": true,                // Turkmen
		"tg-tj": true,                // Tajik
		"mn-mn": true,                // Mongolian
		"ja-jp": true,                // Japanese
		"ko-kr": true,                // Korean
		"th-th": true,                // Thai
		"vi-vn": true,                // Vietnamese
		"my-mm": true,                // Myanmar
		"km-kh": true,                // Khmer
		"lo-la": true,                // Lao
		"si-lk": true,                // Sinhala
		"ta-lk": true, "ta-in": true, // Tamil
		"ne-np": true,                // Nepali
		"dz-bt": true,                // Dzongkha
		"hi-in": true,                // Hindi
		"bn-bd": true, "bn-in": true, // Bengali
		"ur-pk": true, "ur-in": true, // Urdu
		"fa-ir": true, "fa-af": true, // Persian/Farsi
		"ar-sa": true, "ar-eg": true, "ar-ma": true, "ar-dz": true, // Arabic
		"he-il": true,                // Hebrew
		"tr-tr": true, "tr-cy": true, // Turkish
		"az-az": true,                               // Azerbaijani
		"hy-am": true,                               // Armenian
		"ka-ge": true,                               // Georgian
		"sw-ke": true, "sw-tz": true, "sw-ug": true, // Swahili
		"am-et": true,                // Amharic
		"om-et": true,                // Oromo
		"ti-et": true, "ti-er": true, // Tigrinya
		"so-so": true, "so-dj": true, "so-et": true, "so-ke": true, // Somali
		"ha-ng": true, "ha-ne": true, // Hausa
		"yo-ng": true,                                              // Yoruba
		"ig-ng": true,                                              // Igbo
		"ff-gn": true, "ff-ml": true, "ff-mr": true, "ff-ne": true, // Fulah
		"wo-sn": true,                // Wolof
		"bm-ml": true,                // Bambara
		"zu-za": true,                // Zulu
		"xh-za": true,                // Xhosa
		"af-za": true, "af-na": true, // Afrikaans
		"tn-za": true, "tn-bw": true, // Tswana
		"st-za": true, "st-ls": true, // Sotho
		"ss-za": true, "ss-sz": true, // Swati
		"ve-za": true, // Venda
		"ts-za": true, // Tsonga
		"nr-za": true, // Southern Ndebele
		"nd-zw": true, // Northern Ndebele
		"sn-zw": true, // Shona
	}
	return specials[code]
}

// isVariableFragment checks if a fragment appears to be variable (like #a, #123, etc.)
func (upd *URLPatternDetector) isVariableFragment(fragment string) bool {
	// Simple heuristic: if fragment is short and alphanumeric, it's likely variable
	if len(fragment) <= 3 {
		return true
	}

	// Check if it's just a single letter or number
	if len(fragment) == 1 {
		return true
	}

	return false
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
	upd.patternCounts = make(map[string]int)
	upd.patternMutex.Unlock()

	upd.urlMutex.Lock()
	upd.seenURLs = make(map[string]bool)
	upd.urlMutex.Unlock()

	upd.logger.Debug().Msg("URL pattern detector reset")
}
