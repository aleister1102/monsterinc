package config

// NormalizerSettings holds configuration specific to the URL normalization process.
// Task 3.1: Define normalizer settings in configuration structure.
// Currently, there are no specific configurable settings for the normalizer
// as per the PRD for 'target-input-normalization'.
// The default scheme is hardcoded to "http://".
// This struct is created for future extensibility, e.g., making the default scheme configurable.
type NormalizerSettings struct {
	// Example for future: DefaultScheme string `json:"default_scheme" toml:"default_scheme"`
}

// NewDefaultNormalizerSettings creates a new NormalizerSettings with default values.
// Since there are no settings yet, it returns an empty struct.
func NewDefaultNormalizerSettings() NormalizerSettings {
	return NormalizerSettings{
		// Example for future: DefaultScheme: "http",
	}
}

// LoadNormalizerSettings loads the normalizer specific settings.
// Task 3.2: Implement configuration loading for normalizer settings.
// For now, as there are no actual settings to load from a global config file
// for the normalizer module itself, this function returns the default settings.
// In a real scenario, this function would likely take a pointer to a global
// application configuration struct (e.g., from internal/config/config.go)
// and extract the normalizer-specific part, or load a dedicated normalizer config file.
func LoadNormalizerSettings() (NormalizerSettings, error) {
	// Placeholder: In the future, this might involve:
	// 1. Reading from a global AppConfig struct.
	//    e.g., if AppConfig has a `Normalizer NormalizerSettings` field.
	// 2. Validating loaded settings.
	// For now, we just return the defaults.
	return NewDefaultNormalizerSettings(), nil
}
