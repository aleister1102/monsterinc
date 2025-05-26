# Package: internal/config

This package is responsible for managing application configuration for MonsterInc.

## Overview

The `config` package defines the structure of the application's configuration, provides default values, loads configuration from files (YAML or JSON) and environment variables, and validates the loaded configuration.

## Key Components

-   `config.go`: Defines all configuration structs (`GlobalConfig`, `InputConfig`, `HttpxRunnerConfig`, `CrawlerConfig`, `ReporterConfig`, `StorageConfig`, `NotificationConfig`, `LogConfig`, `DiffConfig`, `MonitorConfig`, `NormalizerConfig`, `SchedulerConfig`), their default values, and functions to create new configurations with these defaults (e.g., `NewDefaultGlobalConfig()`). It also contains the `LoadGlobalConfig` function.
-   `loader.go`: Defines the `GetConfigPath` function. This function determines the configuration file path to be loaded by `LoadGlobalConfig` based on the following priority:
    1.  A path provided directly via a command-line flag (e.g., `-globalconfig` in `main.go`).
    2.  The value of the `MONSTERINC_CONFIG_PATH` environment variable.
    3.  `config.yaml` in the current working directory.
    4.  `config.json` in the current working directory.
    5.  `config.yaml` in the directory containing the executable.
    6.  `config.json` in the directory containing the executable.
    If no file is found through these methods, `LoadGlobalConfig` will proceed with defaults (and environment variable overrides if enabled).
-   `LoadGlobalConfig(providedPath string) (*GlobalConfig, error)` (in `config.go`):
    -   Starts by initializing a `GlobalConfig` with default values.
    -   Calls `GetConfigPath(providedPath)` to determine the actual configuration file path.
    -   If a valid file path is found, it attempts to read and unmarshal the file. YAML is preferred if the file extension is `.yaml` or `.yml`; otherwise, JSON is assumed.
    -   **Note on YAML**: To enable YAML parsing, the import `gopkg.in/yaml.v3` must be present in `go.mod` and used in `config.go`.
    -   (Optional) After loading from file (if any), it can be configured to override values with environment variables.
    -   **Note on Environment Variables**: To enable environment variable overrides, a library like `github.com/kelseyhightower/envconfig` would typically be used. Ensure it is in your `go.mod` and integrated into `LoadGlobalConfig`. Environment variables should be prefixed (e.g., `MONSTERINC_HTTPXRUNNERCONFIG_THREADS=50`).
-   `validator.go`: Implements `ValidateConfig(*GlobalConfig) error` which uses the `go-playground/validator/v10` library to validate the fields of the loaded `GlobalConfig` based on struct tags (e.g., `required`, `min`, `max`, `url`, `fileexists`, custom validators like `loglevel`, `logformat`, `mode`).
    -   **Note on Validator**: The import `github.com/go-playground/validator/v10` is required. Ensure it is in your `go.mod` and run `go mod tidy`.

## Configuration File

-   An example configuration file `config.example.yaml` (and `config.json` for an alternative) is provided in the project root, showcasing available options, their default values, and comments explaining each field.
-   The application automatically searches for `config.yaml` or `config.json` in default locations (CWD, executable directory) or uses the path specified via the `-globalconfig` command-line flag or `MONSTERINC_CONFIG_PATH` environment variable.

## Usage in `main.go`

1.  A command-line flag `-globalconfig` (aliased as `-gc`) is defined to allow users to specify a configuration file path. This can be empty to trigger automatic path detection.
2.  `config.LoadGlobalConfig(globalConfigFileFlagValue)` is called to load the configuration settings. `GetConfigPath` within `LoadGlobalConfig` handles the path resolution logic.
3.  `config.ValidateConfig()` is called to ensure the loaded configuration is valid before proceeding.
4.  The validated `GlobalConfig` object (or its sub-structs) is then passed to various application modules (crawler, reporter, scheduler, etc.) as needed.

## Important Notes for Developers

-   **Dependencies**: This package relies on external libraries for YAML parsing and validation. Ensure these are correctly managed in your `go.mod` file:
    -   `gopkg.in/yaml.v3` (for YAML)
    -   `github.com/go-playground/validator/v10` (for validation)
    -   (Optionally, `github.com/kelseyhightower/envconfig` if implementing environment variable overrides)
-   The logic for overriding configuration with environment variables is currently commented out in `LoadGlobalConfig`. If this functionality is needed, uncomment the relevant section and ensure the `envconfig` library is imported and added to `go.mod`. 