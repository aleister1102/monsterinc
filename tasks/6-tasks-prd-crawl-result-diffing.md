## Relevant Files

- `internal/differ/url_differ.go` - Core logic for comparing URL lists from current and previous scans.
- `internal/differ/url_differ_test.go` - Unit tests for `url_differ.go`.
- `internal/datastore/parquet_reader.go` - Logic to read historical URL lists from Parquet files. This might be a new file or integrated into an existing datastore component.
- `internal/datastore/parquet_reader_test.go` - Unit tests for `parquet_reader.go`.
- `internal/models/url_diff.go` - Structs to represent the diff output (e.g., `URLDiffResult` with lists of new, old, existing URLs and their status).
- `internal/reporter/html_reporter.go` - To be modified to accept and use `URLDiffResult`.
- `internal/reporter/templates/report.html.tmpl` - To be modified to display the diff status (e.g., new column, icons, separate section for old URLs).
- `internal/config/diff_config.go` - Configuration for the differ (if any specific settings are needed, e.g., how many previous scans to consider, though PRD implies most recent).
- `cmd/monsterinc/main.go` - To integrate the call to the `UrlDiffer` service.

### Notes

- Efficiently finding and reading the correct historical Parquet file for a specific `RootTargetURL` is crucial.
- The visual representation of diff status in the HTML report needs careful consideration for clarity (FR4.2).
- Unit tests should cover various scenarios: new URLs, old URLs, no changes, first scan, corrupted previous scan data (FR5, Open Question 2).
- Consider how to pass the `RootTargetURL` context through the system for accurate diffing.

## Tasks

- [ ] 1.0 Setup URL Differ Core Logic
  - [ ] 1.1 Define `URLStatus` enum or const (e.g., `StatusNew`, `StatusOld`, `StatusExisting`) in `internal/models/url_diff.go`.
  - [ ] 1.2 Define `DiffedURL` struct in `internal/models/url_diff.go` (e.g., `NormalizedURL string, Status URLStatus, LastSeenData *httpxrunner.ProbeResult` - where `LastSeenData` could be for old URLs).
  - [ ] 1.3 Define `URLDiffResult` struct in `internal/models/url_diff.go` (e.g., `RootTargetURL string, Results []DiffedURL`, or separate lists for New, Old, Existing).
  - [ ] 1.4 Create `UrlDiffer` struct in `internal/differ/url_differ.go` (dependencies: `ParquetReader`).
  - [ ] 1.5 Implement `NewUrlDiffer(parquetReader *datastore.ParquetReader)` in `url_differ.go`.
  - [ ] 1.6 Implement `Compare(currentScanResults []httpxrunner.ProbeResult, rootTargetURL string)` method in `UrlDiffer` returning `(*models.URLDiffResult, error)`.

- [ ] 2.0 Implement Historical Data Retrieval
  - [ ] 2.1 Create `ParquetReader` struct in `internal/datastore/parquet_reader.go` (dependency: `StorageConfig`).
  - [ ] 2.2 Implement `NewParquetReader(cfg *config.StorageConfig)`.
  - [ ] 2.3 Implement `FindMostRecentScanURLs(rootTargetURL string) ([]string, error)` in `ParquetReader` (FR1.2, FR2).
        *This involves: listing files in `data/YYYYMMDD/`, finding the latest relevant Parquet file, reading it, and filtering URLs for the given `rootTargetURL`.*
  - [ ] 2.4 Handle cases where no previous scan data is found for the `rootTargetURL` (should return empty list, no error - for FR5).
  - [ ] 2.5 Add error handling for Parquet file reading/parsing issues in `ParquetReader`.
  - [ ] 2.6 Write unit tests for `ParquetReader`, especially `FindMostRecentScanURLs`.

- [ ] 3.0 Implement URL Comparison and Status Assignment
  - [ ] 3.1 In `UrlDiffer.Compare`, call `ParquetReader.FindMostRecentScanURLs` to get historical URLs.
  - [ ] 3.2 Implement logic to compare current normalized URLs with historical normalized URLs (FR3).
        *Use sets or maps for efficient comparison.*
  - [ ] 3.3 Assign `StatusNew` to URLs in current scan but not in historical.
  - [ ] 3.4 Assign `StatusOld` to URLs in historical but not in current scan.
  - [ ] 3.5 Assign `StatusExisting` to URLs present in both.
  - [ ] 3.6 Populate the `URLDiffResult` struct.
  - [ ] 3.7 Write unit tests for `UrlDiffer.Compare` covering different scenarios.

- [ ] 4.0 Integrate Diff Results into HTML Report (liaise with `httpx-html-reporting` feature)
  - [ ] 4.1 Modify `HtmlReporter.GenerateReport` in `internal/reporter/html_reporter.go` to accept `URLDiffResult` (or the status per URL) as additional input.
        *Alternatively, the `UrlDiffer` could be a dependency of `HtmlReporter`.*
  - [ ] 4.2 Update `ReportPageData` or `ProbeResultDisplay` in `internal/models/report_data.go` to include the `URLStatus`.
  - [ ] 4.3 Modify `report.html.tmpl` to display the `URLStatus`:
        *   Add a new "Status" column to the main results table (FR4.2).
        *   Use visual indicators (colors, icons) for New/Old status.
  - [ ] 4.4 Implement a section in the HTML report to list "Old/Missing" URLs, potentially with their last known data if available (FR4.2, Open Question 1).
        *This might involve fetching minimal last-known data for old URLs if required by the display.*
  - [ ] 4.5 Ensure the JavaScript in `report.js` can handle/filter/sort based on the new Status column if necessary.

- [ ] 5.0 Handling Special Cases and Error Conditions
  - [ ] 5.1 Implement first scan handling in `UrlDiffer.Compare`: if no previous data, all current URLs are `StatusExisting` (or `StatusNew` if preferred for first report) (FR5).
  - [ ] 5.2 If `ParquetReader` returns an error (e.g., corrupted file), `UrlDiffer.Compare` should log the error and treat as a first scan for diffing purposes (Open Question 2).
  - [ ] 5.3 Add logging for key events in `UrlDiffer` and `ParquetReader`.
  - [ ] 5.4 Update `cmd/monsterinc/main.go`: after Parquet storage and before HTML reporting, call the `UrlDiffer.Compare` for each root target. Pass results to HTML reporter. 