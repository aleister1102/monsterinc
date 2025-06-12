package config

// SecretsConfig holds the configuration for the secret scanner.
type SecretsConfig struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`
	NotifyOnFound bool          `json:"notify_on_found" yaml:"notify_on_found"`
	SecretsStore  StorageConfig `json:"secrets_store" yaml:"secrets_store"`
}

// NewDefaultSecretsConfig creates a new SecretsConfig with default values.
func NewDefaultSecretsConfig() SecretsConfig {
	return SecretsConfig{
		Enabled:       true,
		NotifyOnFound: true,
		SecretsStore:  NewDefaultStorageConfig(),
	}
}
