# MonsterInc

MonsterInc is a command-line interface (CLI) tool written in Go, used for gathering information from websites, performing HTTP/HTTPS probing, and generating reports.

## Key Features

-   **Web Crawling**: Collects URLs from websites starting from one or more seed URLs.
    -   Control crawl scope (allowed/disallowed hostnames, subdomains, path regexes).
    -   Customize User-Agent, timeout, depth, number of threads.
    -   Can respect or ignore `robots.txt`.
    -   Checks `Content-Length` before crawling to avoid downloading large files.
-   **HTTP/HTTPS Probing**: Sends requests to collected URLs to get detailed information.
    -   Uses ProjectDiscovery's `httpx` library.
    -   Extracts diverse information: status code, content type, content length, title, web server, headers, IPs, CNAMEs, ASN, TLS information, used technologies.
    -   Customize HTTP method, request URIs, headers, proxy, timeout, retries.
-   **HTML Reporting**: Generates interactive HTML reports from probing results.
    -   Displays results in a table format, searchable, filterable, and sortable.
    -   Embeds custom CSS/JS for a good user interface and experience.
    -   Uses Bootstrap (via CDN) for basic styling.
-   **Parquet Storage**: Writes probing results to Parquet files for later data analysis.
    -   Supports compression codecs: ZSTD (default), SNAPPY, GZIP, UNCOMPRESSED.
    -   Saves files in the directory structure `ParquetBasePath/YYYYMMDD/scan_results_*.parquet`.
-   **Flexible Configuration**: Manages configuration via a YAML file (`config.yaml` preferred) or JSON (`config.json`) and command-line parameters.

## Installation and Build

### Prerequisites

-   Go version 1.23.0 or newer.

### Build

1.  Clone the repository (if applicable):
    ```bash
    git clone <your-repository-url>
    cd monsterinc
    ```
2.  Build the application:
    ```bash
    go build -o monsterinc.exe ./cmd/monsterinc
    ```
    (Or `go build -o monsterinc ./cmd/monsterinc` for Linux/macOS)

## Usage

Run the application from the command line:

```bash
./monsterinc.exe --mode <onetime|automated> [other_options]
```

### Main Command-Line Parameters

-   `--mode <onetime|automated>`: (Required) Execution mode.
    -   `onetime`: Runs once. The report filename will include a detailed timestamp (e.g., `reports/YYYYMMDD-HHMMSS_onetime_report.html`).
    -   `automated`: Runs automatically (e.g., scheduled). The report filename will include the date and mode (e.g., `reports/YYYYMMDD-HHMMSS_automated_report.html`).
-   `-u <path/to/urls.txt>` or `--urlfile <path/to/urls.txt>`: (Optional) Path to a text file containing a list of seed URLs to crawl, one URL per line. If not provided, `input_config.input_urls` from the configuration file will be used.
-   `--globalconfig <path/to/config.yaml>`: (Optional) Path to the configuration file. Defaults to `config.yaml` in the same directory as the executable.

### Examples

```bash
# Run once with a list of URLs from targets.txt
./monsterinc.exe --mode onetime -u targets.txt

# Run automatically, using URLs from the config file (if any)
./monsterinc.exe --mode automated

# Run with a custom configuration file
./monsterinc.exe --mode onetime --globalconfig custom_config.yaml -u targets.txt
```

### Configuration File (`config.yaml` or `config.json`)

The configuration file allows detailed customization of module behavior. By default, the application looks for `config.yaml`. If not found and `config.json` exists, it might be used as a fallback (depending on the current logic in `LoadGlobalConfig`).

See `internal/config/README.md` and the sample `config.example.yaml` (recommended) or `config.json` file for more details on configuration options.

## Project Directory Structure

```
monsterinc/
├── cmd/monsterinc/         # Application entrypoint (main.go)
│   └── README.md
├── internal/                 # Internal application logic code
│   ├── config/             # Configuration management (config.go, validator.go, README.md)
│   ├── core/               # Core business logic (target_manager.go, README.md)
│   ├── crawler/            # Web crawling module (crawler.go, scope.go, asset.go, README.md)
│   ├── datastore/          # Data storage module (parquet_writer.go, parquet_reader.go, README.md)
│   ├── differ/             # Module for comparing differences (url_differ.go, README.md)
│   ├── httpxrunner/        # Wrapper for the httpx library (runner.go, result.go, README.md)
│   ├── logger/             # Logging module (logger.go, README.md)
│   ├── models/             # Data structure definitions (probe_result.go, parquet_schema.go, report_data.go, etc., README.md)
│   ├── notifier/           # (Not yet used) Notification module
│   ├── reporter/           # HTML report generation module
│   │   ├── assets/         # CSS, JS for HTML reports
│   │   ├── templates/      # HTML Template (report.html.tmpl)
│   │   └── html_reporter.go, README.md
│   ├── urlhandler/         # URL handling and normalization (urlhandler.go, file.go, README.md)
│   └── utils/              # (Not yet used) Common utilities
├── reports/                  # Default directory for generated HTML reports (.gitignore-d)
├── database/                 # Default directory for Parquet files (per config, .gitignore-d)
├── tasks/                    # Contains PRD files and task lists (e.g., *.md)
├── .gitignore                # Files and directories ignored by Git
├── config.yaml               # Main application configuration file (should be created from config.example.yaml)
├── config.example.yaml       # Sample YAML configuration file, comprehensive and commented
├── config.json               # Sample JSON configuration file (may not be updated as frequently as YAML)
├── go.mod                    # Go module declaration and dependencies
├── go.sum                    # Dependency checksums
├── monsterinc.exe            # (After build) Executable file for Windows
├── monsterinc                # (After build) Executable file for Linux/macOS
├── PLAN.md                   # Development plan (if any)
└── README.md                 # This README file
```

## Main Modules and Functionality

-   **`config`**: Reads and manages configuration from YAML (preferred) or JSON files.
-   **`urlhandler`**: Normalizes URLs, reads URLs from files.
-   **`crawler`**: Collects URLs from websites.
-   **`httpxrunner`**: Performs HTTP/HTTPS probing on URLs using `httpx`.
-   **`models`**: Defines common data structures.
-   **`datastore`**: Stores results into Parquet files.
-   **`reporter`**: Generates HTML reports from results.
-   **`logger`**: Provides an interface and basic implementation for logging.
-   **`core`**: (Under development) Contains main orchestration logic.

## Contributing

(Information on how to contribute to the project - if applicable) 