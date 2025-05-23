## Relevant Files

- `internal/reporter/html_reporter.go` - Core logic for generating HTML reports from probe results.
- `internal/reporter/html_reporter_test.go` - Unit tests for `html_reporter.go`.
- `internal/reporter/templates/report.html.tmpl` - Go HTML template for the report (using `html/template`).
- `internal/reporter/assets/css/bootstrap.min.css` - Bootstrap CSS (downloaded or via CDN).
- `internal/reporter/assets/css/styles.css` - Custom CSS for the report, including Nunito font and other styles.
- `internal/reporter/assets/js/jquery.min.js` - jQuery (if DataTables.js or Bootstrap JS needs it).
- `internal/reporter/assets/js/bootstrap.bundle.min.js` - Bootstrap JavaScript (for any components used).
- `internal/reporter/assets/js/jquery.dataTables.min.js` - DataTables.js library (optional, as per design consideration).
- `internal/reporter/assets/js/report.js` - Custom JavaScript for interactivity (search, sort, pagination, multi-target navigation, event handlers).
- `internal/models/report_data.go` - Structs defining the data passed to the HTML template (e.g., `ReportPageData`, `ProbeResultDisplay`).
- `internal/config/reporter_config.go` - Configuration for the HTML reporter (e.g., items per page, default sort).
- `cmd/monsterinc/main.go` - To integrate the call to the HTML reporter.

### Notes

- Ensure all assets (CSS, JS, Fonts) are embeddable within the single HTML or stored locally and linked relatively for easy distribution.
- Preference is for a single, self-contained HTML file if feasible by embedding assets.
- Unit tests should be created for the report generation logic in `html_reporter_test.go`.
- Use `go test ./...` to run tests.

## Tasks

- [ ] 1.0 Setup HTML Report Generation Core
  - [ ] 1.1 Define `ReportPageData` and `ProbeResultDisplay` structs in `internal/models/report_data.go` to hold data for the template.
  - [ ] 1.2 Create `HtmlReporter` struct in `internal/reporter/html_reporter.go`.
  - [ ] 1.3 Implement `NewHtmlReporter(cfg *config.ReporterConfig)` in `html_reporter.go`.
  - [ ] 1.4 Implement `GenerateReport(probeResults []httpxrunner.ProbeResult, outputPath string)` method in `HtmlReporter`.
  - [ ] 1.5 Handle the case where `probeResults` is empty (log and do not generate report as per FR2).
  - [ ] 1.6 Create a basic HTML template `report.html.tmpl` in `internal/reporter/templates/`.
  - [ ] 1.7 Implement logic in `GenerateReport` to parse and execute the template with `ReportPageData`.
  - [ ] 1.8 Write basic unit tests for `GenerateReport` focusing on file creation and empty input handling.

- [ ] 2.0 Implement Data Display and Structure
  - [ ] 2.1 Design the HTML table structure in `report.html.tmpl` to display all required fields (Original URL/Identifier, FinalURL, StatusCode, ContentLength, ContentType, Title, ServerHeader, Technologies, IPAddress - CNAME/RedirectChain removed previously).
  - [ ] 2.2 Pass necessary data (headers, results) from `GenerateReport` to the template.
  - [ ] 2.3 Ensure data is correctly iterated and rendered in the table using template actions.
  - [ ] 2.4 Implement basic styling for the table for readability (padding, borders).

- [ ] 3.0 Develop Interactive Features (Search, Sort, Filter)
  - [ ] 3.1 Add a global text input field for search in `report.html.tmpl`.
  - [ ] 3.2 Implement client-side JavaScript in `report.js` for global search across all relevant table columns.
  - [ ] 3.3 Add dropdown menus for `StatusCode` and `ContentType` in `report.html.tmpl`.
  - [ ] 3.4 Populate dropdowns in `report.js` with unique values present in the current dataset.
  - [ ] 3.5 Implement JavaScript filtering logic based on dropdown selections.
  - [ ] 3.6 Implement JavaScript search/filter for the `Technologies` field (matching any part of the comma-separated list).
  - [ ] 3.7 Make table headers for `Title` and `StatusCode` clickable for sorting.
  - [ ] 3.8 Implement client-side JavaScript sorting for these columns in `report.js` (toggle asc/desc).
  - [ ] 3.9 Set default sort order to original input URL (or an implicit order from input slice).
  - [ ] (Optional) 3.10 Evaluate and integrate DataTables.js if it simplifies tasks 3.1-3.8 and meets UI criteria.

- [ ] 4.0 Implement Pagination and Multi-Target Navigation
  - [ ] 4.1 Add pagination controls (e.g., Prev, Next, Page Numbers) to `report.html.tmpl`.
  - [ ] 4.2 Implement client-side JavaScript pagination logic in `report.js` (default 10 items/page).
  - [ ] 4.3 Add a side menu or tab structure in `report.html.tmpl` for multi-target navigation.
  - [ ] 4.4 Populate the navigation menu in `report.js` based on unique root targets from the input data.
  - [ ] 4.5 Implement client-side JavaScript logic to filter/display results based on selected root target (e.g., using URL fragments and event listeners).

- [ ] 5.0 UI/UX Styling and Refinements
  - [ ] 5.1 Integrate Bootstrap CSS framework (download or CDN) into `report.html.tmpl`.
  - [ ] 5.2 Apply Nunito font (or similar sans-serif) via CSS.
  - [ ] 5.3 Implement general layout (containers, spacing) using Bootstrap grid/utilities.
  - [ ] 5.4 Style elements: rounded corners, subtle box shadows, gradients (where appropriate) in `styles.css`.
  - [ ] 5.5 Ensure interactive elements have clear hover/click feedback (CSS and/or JS).
  - [ ] 5.6 Test and ensure report responsiveness on common desktop screen sizes.
  - [ ] 5.7 Address long string display: implement truncation with tooltips or scrollable cells for fields like Title.

- [ ] 6.0 Configuration and Finalization
  - [ ] 6.1 Define `ReporterConfig` struct in `internal/config/reporter_config.go` (e.g., `ItemsPerPage`).
  - [ ] 6.2 Load `ReporterConfig` (e.g., as part of `GlobalConfig` or a separate reporter config file).
  - [ ] 6.3 Use configured `ItemsPerPage` in pagination logic.
  - [ ] 6.4 Implement embedding or local linking of all CSS/JS assets to create a self-contained/easily distributable report.
  - [ ] 6.5 Update `cmd/monsterinc/main.go` to call the `HtmlReporter.GenerateReport` after `httpx` probing if results are available.
  - [ ] 6.6 Add logging for key events in `HtmlReporter` (report generation start/end, errors). 