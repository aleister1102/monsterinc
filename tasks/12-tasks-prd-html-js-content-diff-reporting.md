## Relevant Files

- `internal/differ/content_differ.go` - Core logic for generating diffs between two versions of content (HTML/JS).
- `internal/differ/beautifier.go` - (Optional) Logic to beautify/normalize HTML and JS content before diffing to reduce noise.
- `internal/datastore/file_history_store.go` - Used to retrieve the previous version of the file content for comparison.
- `internal/reporter/html_diff_reporter.go` - (New or part of `html_reporter.go`) Generates an HTML report showing the content diff.
- `internal/reporter/templates/diff_report.html.tmpl` - (New) Go HTML template for the side-by-side or unified diff view.
- `internal/models/content_diff.go` - (New) Structs to represent diff results (e.g., `ContentDiffResult`, lines added/removed/changed).
- `internal/config/config.go` - May contain `DiffReporterConfig` if specific settings are needed for diff reports.
- `internal/monitor/service.go` - Will trigger the diff generation and reporting when a change is detected.

### Notes

- Choose a suitable Go diff library (e.g., `github.com/sergi/go-diff` or `github.com/pmezard/go-difflib`).
- The diff report should be clear, easy to read, and highlight changes effectively.
- Consider the performance implications of diffing large files.

## Tasks

- [ ] 1.0 Setup Content Differ Core (in `internal/differ/content_differ.go`)
  - [ ] 1.1 Choose and add a Go diff library to `go.mod`.
  - [ ] 1.2 Define `ContentDiffer` struct (dependencies: `logger.Logger`).
  - [ ] 1.3 Implement `NewContentDiffer(logger logger.Logger) *ContentDiffer` constructor.
  - [ ] 1.4 Implement `GenerateDiff(previousContent []byte, currentContent []byte, contentType string) (*models.ContentDiffResult, error)` method (FR1).
        *   `contentType` can be "text/html" or "application/javascript".
        *   The method should return a structured diff (e.g., list of changes with type: add, delete, modify, and the lines involved).

- [ ] 2.0 (Optional) Implement Content Beautification/Normalization (in `internal/differ/beautifier.go`)
  - [ ] 2.1 If deemed necessary to reduce noisy diffs: Implement `BeautifyHTML(htmlContent []byte) ([]byte, error)` (FR2.1).
        *   Use a library or custom logic for basic HTML formatting (consistent indentation, spacing).
  - [ ] 2.2 If deemed necessary: Implement `BeautifyJS(jsContent []byte) ([]byte, error)` (FR2.2).
        *   Use a library (e.g., a Go wrapper for a JS beautifier, or a native Go one if available) for JS formatting.
  - [ ] 2.3 In `ContentDiffer.GenerateDiff`, optionally apply beautification based on `contentType` before diffing.

- [ ] 3.0 Implement HTML Diff Report Generation (in `internal/reporter/html_diff_reporter.go`)
  - [ ] 3.1 Define `HtmlDiffReporter` struct (dependencies: `config.ReporterConfig` (if any diff-specific settings), `logger.Logger`).
  - [ ] 3.2 Implement `NewHtmlDiffReporter(...)` constructor.
  - [ ] 3.3 Implement `GenerateDiffReport(url string, previousContent []byte, currentContent []byte, diffResult *models.ContentDiffResult, outputPath string) error` (FR3.1).
  - [ ] 3.4 Create `diff_report.html.tmpl` in `internal/reporter/templates/`.
        *   Design a side-by-side or unified diff view (FR3.1, FR3.3). Side-by-side is often clearer.
        *   Use different background colors for added, removed, and changed lines/sections.
        *   Display metadata: URL, timestamps of versions (if available).
  - [ ] 3.5 Populate `DiffReportPageData` struct (similar to `ReportPageData`) with necessary data for the template.
  - [ ] 3.6 Parse and execute `diff_report.html.tmpl`.
  - [ ] 3.7 (Optional) Add basic interactivity (e.g., navigating to next/prev change) using minimal JS in the template itself or a small `diff_report.js`.
  - [ ] 3.8 Ensure the report is self-contained or uses assets from the main HTML report if co-located.

- [ ] 4.0 Integrate Diff Reporting into Monitoring Service
  - [ ] 4.1 Modify `internal/monitor/service.go`.
  - [ ] 4.2 When a file change is detected (Task 3.3 in `10-tasks-prd-html-js-file-monitoring.md`):
        *   Retrieve the `previousContent` from `FileHistoryStore` (ensure it stores content if `StoreFullContentOnChange` is true).
        *   Call `ContentDiffer.GenerateDiff` with old and new content.
        *   If diff generation is successful, call `HtmlDiffReporter.GenerateDiffReport` to create the HTML diff file.
        *   The path to this diff report can be included in the Discord notification for the file change.
  - [ ] 4.3 Decide on the naming and location for diff report files (e.g., `output_dir/diffs/<domain>/<filename_timestamp1_vs_timestamp2>.html`).

- [ ] 5.0 Handling Large Files and Errors
  - [ ] 5.1 In `ContentDiffer.GenerateDiff`, add checks for very large files. If files are excessively large (e.g., > configurable `MaxDiffFileSizeMB`), consider: 
        *   Not generating a line-by-line diff but reporting "Content changed, file too large for detailed diff."
        *   Or, if the diff library supports it, generating a summary or partial diff.
  - [ ] 5.2 Handle errors from the diff library gracefully.
  - [ ] 5.3 Handle errors during report generation (file I/O, template execution).
  - [ ] 5.4 Add logging for diff generation and report creation processes.

- [ ] 6.0 Configuration (as part of `MonitorConfig` or a new `DiffReporterConfig`)
  - [ ] 6.1 Add to `internal/config/config.go`:
        *   `MaxDiffFileSizeMB int` (default, e.g., 5MB)
        *   `BeautifyHTMLForDiff bool`
        *   `BeautifyJSForDiff bool`
  - [ ] 6.2 Ensure these are in `config.example.json`.

- [ ] 7.0 Unit Tests
  - [ ] 7.1 Test `ContentDiffer.GenerateDiff` with various inputs: no changes, additions, deletions, modifications for both HTML and JS.
  - [ ] 7.2 Test (optional) beautifier functions if implemented.
  - [ ] 7.3 Test `HtmlDiffReporter.GenerateDiffReport` for correct HTML structure and data population (may involve checking parts of the generated HTML string).
  - [ ] 7.4 Test handling of large files (e.g., skipping diff if oversized).
  - [ ] 7.5 Test integration points where `MonitoringService` calls the differ and reporter. 