## Relevant Files

- `internal/reporter/html_reporter.go` - Core logic for generating HTML reports from probe results.
- `internal/reporter/html_reporter_test.go` - Unit tests for `html_reporter.go`.
- `internal/reporter/templates/report.html.tmpl` - Go HTML template for the report (using `html/template`).
- `internal/reporter/assets/css/styles.css` - Custom CSS for the report, including Nunito font, color scheme, and layout styles.
- `internal/reporter/assets/js/report.js` - Custom JavaScript for interactivity (search, sort, pagination, multi-target navigation, event handlers, modal details).
- `internal/models/report_data.go` - Structs defining the data passed to the HTML template (e.g., `ReportPageData`, `ProbeResultDisplay`).
- `internal/config/config.go` - Contains `ReporterConfig` for the HTML reporter (e.g., items per page, enabling DataTables).
- `cmd/monsterinc/main.go` - To integrate the call to the HTML reporter.

### Notes

- CSS and JS assets are embedded into the HTML template via Go's `embed` package for a self-contained report.
- Bootstrap and DataTables are loaded via CDN to reduce embedded size and leverage browser caching.
- Unit tests should be created/updated for the report generation logic in `html_reporter_test.go`.
- Use `go test ./...` to run tests.

## Tasks

- [X] 1.0 Setup HTML Report Generation Core
  - [X] 1.1 Define `ReportPageData` and `ProbeResultDisplay` structs in `internal/models/report_data.go` to hold data for the template.
  - [X] 1.2 Create `HtmlReporter` struct in `internal/reporter/html_reporter.go`.
  - [X] 1.3 Implement `NewHtmlReporter(cfg *config.ReporterConfig)` in `html_reporter.go`.
  - [X] 1.4 Implement `GenerateReport(probeResults []models.ProbeResult, outputPath string)` method in `HtmlReporter`.
  - [X] 1.5 Handle the case where `probeResults` is empty (log and do not generate report as per FR2).
  - [X] 1.6 Create a basic HTML template `report.html.tmpl` in `internal/reporter/templates/`.
  - [X] 1.7 Implement logic in `GenerateReport` to parse and execute the template with `ReportPageData`.
  - [X] 1.8 Write basic unit tests for `GenerateReport` focusing on file creation and empty input handling.

- [X] 2.0 Implement Data Display and Structure
  - [X] 2.1 Design the HTML table structure in `report.html.tmpl` to display all required fields (Input URL, Final URL, Status, Title, Technologies, Web Server, Content Type, Length, IPs, Actions).
  - [X] 2.2 Pass necessary data (headers, results) from `GenerateReport` to the template.
  - [X] 2.3 Ensure data is correctly iterated and rendered in the table using template actions.
  - [X] 2.4 Implement basic styling for the table for readability (padding, borders, custom color scheme).

- [X] 3.0 Develop Interactive Features (Search, Sort, Filter)
  - [X] 3.1 Add a global text input field for search in `report.html.tmpl`.
  - [X] 3.2 Implement client-side JavaScript in `report.js` for global search across all relevant table columns (leveraging DataTables if enabled).
  - [X] 3.3 Add dropdown menus for `StatusCode` and `ContentType` in `report.html.tmpl`.
  - [X] 3.4 Populate dropdowns with unique values present in the current dataset (done via Go template rendering into HTML, JS uses these).
  - [X] 3.5 Implement JavaScript filtering logic based on dropdown selections (leveraging DataTables if enabled).
  - [X] 3.6 Implement JavaScript search/filter for the `Technologies` field (text input, leveraging DataTables if enabled).
  - [X] 3.7 Make table headers clickable for sorting (functionality provided by DataTables if enabled, or custom JS).
  - [X] 3.8 Implement client-side JavaScript sorting for these columns in `report.js` (toggle asc/desc) (functionality provided by DataTables if enabled, or custom JS).
  - [X] 3.9 Set default sort order (DataTables handles this, or can be set in JS).
  - [X] 3.10 Evaluate and integrate DataTables.js if it simplifies tasks 3.1-3.8 and meets UI criteria. (DataTables is integrated as an optional feature via config).

- [X] 4.0 Implement Pagination and Multi-Target Navigation
  - [X] 4.1 Add pagination controls (e.g., Prev, Next, Page Numbers) to `report.html.tmpl` (DataTables handles this, or custom JS).
  - [X] 4.2 Implement client-side JavaScript pagination logic in `report.js` (DataTables handles this, or custom JS has been implemented).
  - [X] 4.3 Add a top menu structure in `report.html.tmpl` for multi-target navigation.
  - [X] 4.4 Populate the navigation menu in the Go template based on unique root targets from the input data.
  - [X] 4.5 Implement client-side JavaScript logic in `report.js` to filter/display results based on selected root target.

- [X] 5.0 UI/UX Styling and Refinements
  - [X] 5.1 Integrate Bootstrap CSS framework (via CDN) into `report.html.tmpl`.
  - [X] 5.2 Apply Nunito font (or similar sans-serif) via CSS.
  - [X] 5.3 Implement general layout (containers, spacing) using Bootstrap grid/utilities and custom CSS.
  - [X] 5.4 Style elements: rounded corners, subtle box shadows, custom color scheme (blue/green focus) in `styles.css`.
  - [X] 5.5 Ensure interactive elements have clear hover/click feedback (CSS and/or JS).
  - [X] 5.6 Test and ensure report responsiveness on common desktop screen sizes.
  - [X] 5.7 Address long string display: implement truncation with tooltips or scrollable cells for fields like Title (CSS/JS handles this).
  - [X] 5.8 Ensure footer is always at the bottom of the page, regardless of content height.

- [X] 6.0 Configuration and Finalization
  - [X] 6.1 Define `ReporterConfig` struct in `internal/config/config.go` (e.g., `DefaultItemsPerPage`, `EnableDataTables`).
  - [X] 6.2 Load `ReporterConfig` (as part of `GlobalConfig`).
  - [X] 6.3 Use configured `DefaultItemsPerPage` and `EnableDataTables` in report generation.
  - [X] 6.4 Implement embedding of local CSS/JS assets (`styles.css`, `report.js`) and use CDN for libraries.
  - [X] 6.5 Update `cmd/monsterinc/main.go` to call the `HtmlReporter.GenerateReport` after `httpx` probing if results are available. (Verified and confirmed as implemented)
  - [X] 6.6 Add logging for key events in `HtmlReporter` (report generation start/end, errors). 