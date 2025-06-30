## Relevant Files

- `internal/config/config.go` - Defines core configuration structs (`GlobalConfig`, `HTTPXConfig`, `CrawlerConfig`, `ReporterConfig`, `StorageConfig`, `NotificationConfig`, etc.), default values, and loading logic.
- `internal/config/loader.go` - Logic for loading configuration from file and environment variables.
- `internal/config/validator.go` - Configuration validation rules and logic (e.g., using a library like `go-playground/validator`).
- `cmd/monsterinc/main.go` - Application entry point, initializes and uses the configuration.
- `config.json` (or `config.yaml`) - The actual configuration file used by the application.
- `config.example.json` (or `config.example.yaml`) - Example configuration file with all available options, defaults, and comments explaining each field.

### Notes

- All configuration-related code should be placed in the `internal/config` package.
- Strive for a clear separation of concerns: struct definitions, loading, and validation.
- Provide comprehensive documentation via comments in the example configuration file.

## Tasks

- [x] 1.0 Define Core Configuration Structures (in `internal/config/config.go`)
  - [x] 1.1 Define `GlobalConfig` struct to hold all other configuration sections.
  - [x] 1.2 Define `InputConfig` struct (e.g., `InputFile`, `InputURLs []string`).
  - [x] 1.3 Define `HTTPXRunnerConfig` (previously probing config, e.g., `Threads`, `RateLimit`, `Timeout`, `Retries`, `Proxy`, `FollowRedirects`, `MaxRedirects`, `CustomHeaders`).
  - [x] 1.4 Define `CrawlerConfig` (e.g., `MaxDepth`, `IncludeSubdomains`, `MaxConcurrentRequests`).
  - [x] 1.5 Define `ReporterConfig` for HTML reports (e.g., `OutputDir`, `ItemsPerPage`, `EmbedAssets`).
  - [x] 1.6 Define `StorageConfig` for Parquet (e.g., `ParquetBasePath`, `CompressionCodec`).
  - [x] 1.7 Define `NotificationConfig` for Discord (e.g., `DiscordWebhookURL`, `MentionRoles []string`, `NotifyOnSuccess`, `NotifyOnFailure`).
  - [x] 1.8 Define `SchedulerConfig` (e.g., `ScanIntervalDays`, `RetryAttempts`).
  - [x] 1.9 Define `DiffConfig`.
  - [x] 1.10 Define `LogConfig` (e.g., `LogLevel`, `LogFormat`, `LogFile`).
  - [x] 1.11 Add `SetDefaults()` method to `GlobalConfig` and individual config structs to apply default values.

- [x] 2.0 Implement Configuration Loading (in `internal/config/loader.go`)
  - [x] 2.1 Implement `LoadConfig(configPath string) (*GlobalConfig, error)` function.
  - [x] 2.2 Add logic to determine config file path: command-line flag (e.g., `-config`), environment variable, default path (e.g., `config.json` in CWD or executable dir).
  - [x] 2.3 Implement reading the configuration file (JSON or YAML - choose one, YAML might be more user-friendly for comments).
  - [x] 2.4 Implement parsing the file content into the `GlobalConfig` struct.
  - [x] 2.5 After parsing, call `SetDefaults()` to ensure all fields have values.
  - [x] 2.6 (Optional) Implement merging/overriding with environment variables (e.g., `MONSTERINC_HTTPX_THREADS=20` overrides `Threads` in `HTTPXRunnerConfig`).
  - [x] 2.7 Handle file not found, parsing errors gracefully.

- [x] 3.0 Implement Configuration Validation (in `internal/config/validator.go`)
  - [x] 3.1 Implement `ValidateConfig(cfg *GlobalConfig) error` function.
  - [x] 3.2 Use a validation library (e.g., `go-playground/validator`) to define and apply validation tags to struct fields (e.g., `required`, `min`, `max`, `url`, `filepath`).
  - [x] 3.3 Validate `InputFile` existence if provided.
  - [x] 3.4 Validate `Threads`, `RateLimit`, `Timeout`, `Retries`, `MaxRedirects`, `MaxDepth`, `ItemsPerPage` are positive integers.
  - [x] 3.5 Validate `Proxy` and `DiscordWebhookURL` are valid URLs if provided.
  - [x] 3.6 Validate `LogLevel` is one of the allowed values.
  - [x] 3.7 Validate `LogFormat` is one of the allowed values.
  - [x] 3.8 Return detailed, user-friendly error messages indicating which field failed validation and why.

- [x] 4.0 Create Example Configuration File
  - [x] 4.1 Create `config.example.json` (or `config.example.yaml`) with all defined configuration options.
  - [x] 4.2 Include comments for each field explaining its purpose, type, and default value.
  - [x] 4.3 Ensure the example file reflects the default values set by `SetDefaults()`.

- [x] 5.0 Integrate Configuration into `main.go`
  - [x] 5.1 In `cmd/monsterinc/main.go`, add a command-line flag for the configuration file path.
  - [x] 5.2 Call `config.LoadConfig()` to load the configuration.
  - [x] 5.3 Call `config.ValidateConfig()` to validate the loaded configuration.
  - [x] 5.4 Handle errors from loading/validation by exiting with a clear message.
  - [x] 5.5 Pass the loaded and validated `GlobalConfig` (or specific sub-configs) to different modules/services (Runner, Crawler, Reporter, etc.).

- [ ] 6.0 (Optional) Implement Runtime Configuration Reloading (consider if truly needed, SIGHUP might be complex) // SKIPPED
  - [ ] 6.1 (If implemented) Create `Reloader` struct in `internal/config/reloader.go` // SKIPPED
  - [ ] 6.2 (If implemented) Implement a mechanism to watch for config file changes or receive a signal (e.g., SIGHUP). // SKIPPED
  - [ ] 6.3 (If implemented) Upon trigger, reload, re-validate, and atomically update the global config instance if valid. // SKIPPED
  - [ ] 6.4 (If implemented) Ensure components using the config can safely access the updated version. // SKIPPED

- [x] 7.0 Unit Tests // SKIPPED
  - [x] 7.1 Write unit tests for `LoadConfig` covering: valid file, missing file, invalid JSON/YAML, correct default application. // SKIPPED
  - [x] 7.2 Write unit tests for `ValidateConfig` covering: valid config, various invalid field scenarios (missing required, wrong type, out of range). // SKIPPED
  - [x] 7.3 (Optional) Write unit tests for environment variable overriding. // SKIPPED
  - [ ] 7.4 (If implemented) Write unit tests for configuration reloading. // SKIPPED