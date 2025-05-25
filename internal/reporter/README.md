# Package reporter

This package is responsible for generating reports from scan results, currently focusing on HTML reports.

## Components

### `HtmlReporter`

- **`NewHtmlReporter(cfg *config.ReporterConfig, appLogger *log.Logger) (*HtmlReporter, error)`**: Constructor for `HtmlReporter`. It initializes the reporter with configuration (e.g., template paths, items per page) and a logger. It also parses the HTML template (either from an embedded source or a custom path).
- **`GenerateReport(probeResults []models.ProbeResult, urlDiffs map[string]models.URLDiffResult, outputPath string) error`**: This is the main method for generating the HTML report.
    1. It calls `prepareReportData` to transform `probeResults` and `urlDiffs` into a `models.ReportPageData` struct suitable for the template.
    2. `prepareReportData` also populates statistics (total, success, failed results), unique filterable values (status codes, content types, technologies, root targets), and serializes probe results to JSON for client-side JavaScript interaction.
    3. It embeds custom CSS and JavaScript assets into `ReportPageData` if `EmbedAssets` is configured.
    4. It executes the parsed Go HTML template (`report.html.tmpl`) with `ReportPageData`.
    5. The generated HTML content is written to the specified `outputPath`.
    6. It handles a configuration `GenerateEmptyReport` to decide whether to create a report if no probe results are available.

### `prepareReportData` (internal helper method)

- Takes `probeResults []models.ProbeResult` and `urlDiffs map[string]models.URLDiffResult`.
- Iterates through `probeResults`, converting each to `models.ProbeResultDisplay`.
- If `urlDiffs` data is available for a probe result's `RootTargetURL`, it attempts to find the corresponding `URLStatus` from the diff results and assign it to `ProbeResultDisplay.URLStatus`.
- Collects various statistics and unique values for filtering dropdowns in the report.
- Marshals `ProbeResultDisplay` slice into JSON for JavaScript use (`ReportPageData.ProbeResultsJSON`).
- Stores the raw `urlDiffs` map in `ReportPageData.URLDiffs` for potential direct use in the template (e.g., for the "Old/Missing URLs" section).

## Templates and Assets

- **`templates/report.html.tmpl`**: The Go HTML template used to render the report. It includes:
    - Display of probe results in a sortable, filterable table.
    - Filters for status code, content type, URL diff status, technologies, and global search.
    - A section for listing "Old/Missing URLs" based on the `URLDiffs` data.
    - Client-side JavaScript (`assets/js/report.js`) for interactivity (filtering, sorting, pagination, details modal).
    - Custom CSS (`assets/css/styles.css`) for styling.
- Assets (CSS, JS) can be embedded directly into the HTML report or linked if not embedded, based on configuration.

## Data Models

- `models.ProbeResult`: Input data for scan results.
- `models.URLDiffResult`: Input data for URL diff statuses.
- `models.ReportPageData`: The main struct passed to the HTML template, containing all necessary data and configurations for rendering.
- `models.ProbeResultDisplay`: A version of `ProbeResult` tailored for display, including the `URLStatus`.

## Configuration

- Relies on `config.ReporterConfig` for settings like:
    - `OutputDir`: Directory to save reports.
    - `EmbedAssets`: Whether to embed CSS/JS in the HTML.
    - `TemplatePath`: Optional custom path to the HTML template.
    - `GenerateEmptyReport`: Whether to generate a report if there are no results.
    - `ReportTitle`: Custom title for the report.
    - `DefaultItemsPerPage`: For pagination.
    - `EnableDataTables`: To include DataTables library for enhanced table features.

## Logging

- `HtmlReporter` uses a `log.Logger` for its operations, including initialization, report generation steps, and any errors encountered.

## Features

-   **HTML Report Generation**: Creates a single, self-contained (or CDN-linked for common libraries) HTML file.
-   **Interactive UI**: The HTML report includes features such as:
    -   Global search across multiple fields.
    -   Filtering by Status Code, Content Type, and Technologies.
    -   Sorting by various columns (Input URL, Final URL, Status Code, Title, etc.).
    -   Pagination to handle large datasets.
    -   Multi-target navigation via a sidebar if multiple root targets were scanned.
    -   Modal view for detailed information of each probe result, including headers and body snippets.
-   **Customizable**: Report title and items per page can be configured.
-   **Asset Embedding**: Custom CSS and JavaScript are embedded into the HTML report. Common libraries like Bootstrap and jQuery are linked via CDN by default.

## Core Components

-   `html_reporter.go`: Contains the main logic for `HtmlReporter`, including parsing Go HTML templates and populating them with data.
-   `templates/report.html.tmpl`: The Go HTML template file that defines the structure of the report.
-   `assets/`: Directory containing static assets:
    -   `css/styles.css`: Custom CSS for styling the HTML report.
    -   `js/report.js`: Custom JavaScript (using jQuery) for interactivity (search, sort, filter, pagination, modal views, multi-target navigation).

## Configuration

The reporter's behavior is configured via `ReporterConfig` (defined in `internal/config/config.go`), which includes options such as:

-   `OutputDir`: Directory where reports will be saved (though `DefaultOutputHTMLPath` specifies the full path for the main report).
-   `EmbedAssets`: Whether to embed custom assets (currently always true for custom CSS/JS).
-   `TemplatePath`: Custom path to an HTML template file (if not using the embedded one).
-   `GenerateEmptyReport`: Whether to generate a report if there are no results.
-   `ReportTitle`: Custom title for the HTML report.
-   `DefaultItemsPerPage`: Default number of items to show per page in the report table.
-   `EnableDataTables`: Controls whether DataTables.js CDN links are included (though current interactivity is custom JS).
-   `DefaultOutputHTMLPath`: The default full path (including filename) where the HTML report will be saved.

## Usage

An `HtmlReporter` instance is created using `reporter.NewHtmlReporter()`, passing the `ReporterConfig` and a logger.
The report is generated by calling the `GenerateReport()` method with the probe results and the desired output file path.

Example integration in `cmd/monsterinc/main.go`:

```go
// Assuming gCfg is your loaded GlobalConfig and probeResults is []models.ProbeResult

reporterCfg := &gCfg.ReporterConfig
htmlReporter, reporterErr := reporter.NewHtmlReporter(reporterCfg, log.Default())
if reporterErr != nil {
    log.Printf("[ERROR] Main: Failed to initialize HTML reporter: %v", reporterErr)
} else {
    outputFile := reporterCfg.DefaultOutputHTMLPath
    if outputFile == "" {
        outputFile = "monsterinc_report.html" 
        log.Printf("[WARN] Main: ReporterConfig.DefaultOutputHTMLPath is not set. Using default: %s", outputFile)
    }
    err := htmlReporter.GenerateReport(probeResults, nil, outputFile)
    if err != nil {
        log.Printf("[ERROR] Main: Failed to generate HTML report: %v", err)
    } else {
        log.Printf("[INFO] Main: HTML report generated successfully: %s", outputFile)
    }
}
```

## Future Considerations

-   Allow full offline asset embedding (Bootstrap, jQuery) via configuration.
-   More advanced theming options (e.g., dark mode improvements).
-   Unit tests for JavaScript functionality. 