## Relevant Files

- `internal/extractor/path_extractor.go` - Core logic for extracting paths from HTML and JavaScript content.
- `internal/extractor/js_parser.go` - (If using a JS parsing library) Adapter for the JS parsing library (e.g., `robertkrimen/otto` or a more robust AST parser if available and suitable).
- `internal/extractor/html_parser.go` - (If using an HTML parsing library) Adapter for `golang.org/x/net/html` or similar.
- `internal/models/extracted_path.go` (New) - Struct defining an extracted path (e.g., `SourceURL`, `ExtractedURL`, `Context`, `Type` (e.g., script, link, string literal)).
- `internal/normalizer/url_normalizer.go` - (Leverage existing from `1-tasks-prd-target-input-normalization.md`) Used to normalize extracted relative paths to absolute URLs.
- `internal/crawler/service.go` - (Or a new service) To feed these newly extracted paths back into the crawling queue if they are in scope.
- `internal/datastore/path_store.go` (New or part of `parquet_writer.go`) - To store discovered paths and their sources.
- `internal/config/config.go` - May include `ExtractorConfig` if specific settings are needed (e.g., `ExtractPathsFromJSStrings`, `MaxPathDepth`).

### Notes

- Path extraction can be complex, especially from obfuscated JavaScript. Regex is fragile; AST parsing or dedicated tools like `jsluice` (if a Go binding/port exists or can be shelled out to) are more robust.
- Focus on extracting URLs, relative paths, and API endpoints.
- Ensure extracted paths are resolved correctly against their source URL.

## Tasks

- [ ] 1.0 Setup Path Extractor Core (in `internal/extractor/path_extractor.go`)
  - [ ] 1.1 Define `PathExtractor` struct (dependencies: `logger.Logger`, `normalizer.URLNormalizer`).
  - [ ] 1.2 Implement `NewPathExtractor(...)` constructor.
  - [ ] 1.3 Implement `ExtractPaths(sourceURL string, content []byte, contentType string) ([]models.ExtractedPath, error)` method (FR1, FR2).
        *   `contentType` is "text/html" or "application/javascript".

- [ ] 2.0 Implement HTML Path Extraction (in `internal/extractor/html_parser.go` or `path_extractor.go`)
  - [ ] 2.1 Use `golang.org/x/net/html` to parse HTML content.
  - [ ] 2.2 Extract paths from relevant attributes: `<a>[href]`, `<script>[src]`, `<link>[href]`, `<img>[src]`, `<form>[action]`, etc. (FR1.1, FR1.2).
  - [ ] 2.3 Extract paths from inline `<script>` tags and `<style>` tags (content can be passed to JS/CSS extractors if implemented separately) (FR1.3, FR1.4).
  - [ ] 2.4 For each extracted path, record its context (e.g., tag and attribute it was found in).

- [ ] 3.0 Implement JavaScript Path Extraction (in `internal/extractor/js_parser.go` or `path_extractor.go`)
  - [ ] 3.1 Evaluate and choose a method for JS analysis:
        *   Option A: Regex-based (simpler, less accurate, good for string literals, PRD FR2.1 mentions regex for URLs).
        *   Option B: Use a JS parser/AST analyzer (e.g., `robertkrimen/otto` to execute and observe, or a proper AST library if a good Go one exists). This is more robust for dynamic paths (FR2.2, FR2.3, FR2.4).
        *   Option C: Integrate with an external tool like `jsluice` (requires shelling out, managing dependencies).
  - [ ] 3.2 If Regex (Option A): Develop regexes to find URLs, relative paths, and common API endpoint patterns within JS code (string literals, assignments).
  - [ ] 3.3 If JS Parser (Option B/C): Implement logic to traverse the AST or use the tool to identify:
        *   String literals that look like paths/URLs.
        *   Assignments to `window.location`, `document.location`.
        *   Parameters to `fetch()`, `XMLHttpRequest.open()`.
        *   Potentially dynamic path constructions (can be very hard).
  - [ ] 3.4 For each extracted path, record its context (e.g., variable name, function call).

- [ ] 4.0 Path Normalization and Scoping
  - [ ] 4.1 In `PathExtractor.ExtractPaths`, for each raw extracted string:
        *   Use `url_normalizer.NormalizePath(sourceURL, rawPath)` to resolve it to an absolute URL.
        *   Ensure `NormalizePath` handles various cases (absolute, relative, protocol-relative).
  - [ ] 4.2 Filter out invalid or out-of-scope URLs based on `CrawlerConfig` (e.g., `AllowedHostRegex`, `ExcludedHostRegex`). This might be done by the component that consumes these paths (e.g., the crawler).
  - [ ] 4.3 Deduplicate extracted paths from the same source content.

- [ ] 5.0 Storage of Extracted Paths (in `internal/datastore/path_store.go`)
  - [ ] 5.1 Define Parquet schema for `models.ExtractedPath`: `SourceURL`, `ExtractedAbsoluteURL`, `Timestamp`, `Context`, `Type` (FR3.1).
  - [ ] 5.2 Implement `StoreExtractedPaths(paths []models.ExtractedPath) error` to write to Parquet (FR3.2).
        *   Consider if this is a separate Parquet file or part of a larger data store.

- [ ] 6.0 Integration with Crawler/Monitoring
  - [ ] 6.1 The `PathExtractor` will be used by:
        *   The `CrawlerService` (`2-tasks-prd-target-crawling.md`) when it processes a fetched page/JS file.
        *   Potentially by the `MonitoringService` (`10-tasks-prd-html-js-file-monitoring.md`) if new paths in monitored files should be reported or fed back for crawling.
  - [ ] 6.2 New, in-scope, absolute URLs extracted should be added to the crawler's queue (respecting depth limits, etc.).

- [ ] 7.0 Configuration (as part of `CrawlerConfig` or new `ExtractorConfig`)
  - [ ] 7.1 Add to `internal/config/config.go` (if not already covered by `CrawlerConfig`):
        *   `ExtractPathsFromJSComments bool`
        *   `JSPathRegexes []string` (if using regex approach for JS)
  - [ ] 7.2 Ensure these are in `config.example.json`.

- [ ] 8.0 Unit Tests
  - [ ] 8.1 Test HTML path extraction with various HTML structures and path types.
  - [ ] 8.2 Test JavaScript path extraction with different JS snippets, covering string literals, assignments, and function calls (tailor to chosen JS analysis method).
  - [ ] 8.3 Test path normalization logic (delegated to `URLNormalizer` tests mostly).
  - [ ] 8.4 Test `PathExtractor.ExtractPaths` end-to-end for a few sample HTML and JS contents.
  - [ ] 8.5 Test `PathStore` for correct Parquet writing/reading (if applicable). 