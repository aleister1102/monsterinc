# Package: internal/config

This package is responsible for managing application configuration for MonsterInc.

## Overview

The `config` package defines the structure of the application's configuration, provides default values, loads configuration from files (YAML or JSON) and environment variables, and validates the loaded configuration.

## Key Components

-   `config.go`: Defines all configuration structs (`GlobalConfig`, `InputConfig`, `HttpxRunnerConfig`, `CrawlerConfig`, `ReporterConfig`, `StorageConfig`, `NotificationConfig`, `LogConfig`, `DiffConfig`, `MonitorConfig`, `NormalizerConfig`), their default values, and functions to create new configurations with these defaults (e.g., `NewDefaultGlobalConfig()`).
-   `loader.go` (conceptually, logic is within `config.go`'s `LoadGlobalConfig` function): Handles loading the configuration. 
    -   It starts by initializing a `GlobalConfig` with default values.
    -   If a configuration file path is provided (defaulting to `config.yaml` in `main.go`), it attempts to read and unmarshal the file. YAML is preferred if the file extension is `.yaml` or `.yml`; otherwise, JSON is assumed.
    -   **Note on YAML**: To enable YAML parsing, the import `gopkg.in/yaml.v3` must be added to `config.go` and the YAML unmarshalling section within `LoadGlobalConfig` must be uncommented.
    -   After loading from file (if any), it attempts to override values with environment variables.
    -   **Note on Environment Variables**: To enable environment variable overrides, the import `github.com/kelseyhightower/envconfig` must be added to `config.go` and the `envconfig.Process` section within `LoadGlobalConfig` must be uncommented. Environment variables should be prefixed (e.g., `MONSTERINC_HTTPXRUNNERCONFIG_THREADS=50`).
-   `validator.go`: Implements `ValidateConfig(*GlobalConfig) error` which uses the `go-playground/validator/v10` library to validate the fields of the loaded `GlobalConfig` based on struct tags (e.g., `required`, `min`, `max`, `url`, `fileexists`, custom validators like `loglevel`, `logformat`, `mode`).
    -   **Note on Validator**: The import `github.com/go-playground/validator/v10` is required. Ensure it is in your `go.mod` and run `go mod tidy`.

## Configuration File

-   An example configuration file `config.example.yaml` is provided in the project root, showcasing all available options, their default values, and comments explaining each field.
-   The application expects the configuration file to be named `config.yaml` by default (or `config.json` if preferred and YAML is not set up), located in the working directory or specified via the `-globalconfig` command-line flag.

## Usage in `main.go`

1.  A command-line flag `-globalconfig` (defaulting to `config.yaml`) is defined to specify the configuration file path.
2.  `config.LoadGlobalConfig()` is called to load the configuration settings.
3.  `config.ValidateConfig()` is called to ensure the loaded configuration is valid before proceeding.
4.  The validated `GlobalConfig` object (or its sub-structs) is then passed to various application modules (crawler, reporter, etc.) as needed.

## Important Notes for Developers

-   **Dependencies**: This package relies on external libraries for YAML parsing, environment variable processing, and validation. Ensure these are correctly managed in your `go.mod` file:
    -   `gopkg.in/yaml.v3` (for YAML)
    -   `github.com/kelseyhightower/envconfig` (for environment variables)
    -   `github.com/go-playground/validator/v10` (for validation)
-   Remember to uncomment the relevant code sections in `config.go` and add the imports if you wish to use YAML parsing or environment variable overrides fully. 