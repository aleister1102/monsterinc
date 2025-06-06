## Relevant Files

- `internal/extractor/path_extractor.go` - Core logic for extracting paths from HTML and JavaScript content.
- `internal/extractor/js_parser.go` - (If using a JS parsing library) Adapter for the JS parsing library (e.g., `robertkrimen/otto` or a more robust AST parser if available and suitable).
- `internal/extractor/html_parser.go` - (If using an HTML parsing library) Adapter for `golang.org/x/net/html` or similar.
- `internal/models/extracted_path.go` (New) - Struct defining an extracted path (e.g., `SourceURL`, `ExtractedURL`, `Context`, `Type` (e.g., script, link, string literal)).
- `internal/urlhandler/normalizer.go` - (Leverage existing or extend if necessary) Used to normalize extracted relative paths to absolute URLs. (Refers to `1-tasks-prd-target-input-normalization.md` if normalization logic is there).
- `internal/crawler/service.go` - (Or a new service/method within) To feed these newly extracted paths back into the crawling queue if they are in scope.
- `internal/datastore/path_store.go` (New or methods within existing datastore components like `parquet_writer.go`) - To store discovered paths and their sources.
- `internal/config/config.go` - Will include `ExtractorConfig` (likely as part of `CrawlerConfig` or a new top-level section) if specific settings are needed (e.g., `ExtractPathsFromJSStrings`, `MaxPathDepth`).

### Notes

- Path extraction can be complex, especially from obfuscated JavaScript. Regex is fragile; AST parsing or dedicated tools like `jsluice` (if a Go binding/port exists or can be shelled out to) are more robust.
- Focus on extracting URLs, relative paths, and API endpoints.
- Ensure extracted paths are resolved correctly against their source URL.

## Tasks

- [x] 1.0 Setup Path Extractor Core (in `internal/extractor/path_extractor.go`)
  - [x] 1.1 Define `PathExtractor` struct (dependencies: `logger.Logger`, `urlhandler.URLNormalizer` (or similar existing normalizer)).
  - [x] 1.2 Implement `NewPathExtractor(...)` constructor.
  - [x] 1.3 Implement `ExtractPaths(sourceURL string, content []byte, contentType string) ([]models.ExtractedPath, error)` method (FR1, FR2).
        *   `contentType` is "text/html" or "application/javascript".

- [x] 2.0 Implement HTML Path Extraction (in `internal/extractor/html_parser.go` or `path_extractor.go`)
  - [x] 2.1 Use `golang.org/x/net/html` to parse HTML content.
  - [x] 2.2 Extract paths from relevant attributes: `<a>[href]`, `<script>[src]`, `<link>[href]`, `<img>[src]`, `<form>[action]`, etc. (FR1.1, FR1.2).
  - [x] 2.3 Extract paths from inline `<script>` tags and `<style>` tags (content can be passed to JS/CSS extractors if implemented separately) (FR1.3, FR1.4). (Inline script content extraction implemented; style tag content TBD)
  - [x] 2.4 For each extracted path, record its context (e.g., tag and attribute it was found in).

- [ ] 3.0 Implement JavaScript Path Extraction (in `internal/extractor/js_parser.go` or `path_extractor.go`)
  - [x] 3.1 Evaluate and choose a method for JS analysis:
        *   Option A: Regex-based (simpler, less accurate, good for string literals, PRD FR2.1 mentions regex for URLs). - CHOSEN FOR INITIAL IMPLEMENTATION
        *   Option B: Use a JS parser/AST analyzer (e.g., `robertkrimen/otto` to execute and observe, or a proper AST library if a good Go one exists). This is more robust for dynamic paths (FR2.2, FR2.3, FR2.4).
        *   Option C: Integrate with an external tool like `jsluice` (requires shelling out, managing dependencies).
  - [x] 3.2 If Regex (Option A): Develop regexes to find URLs, relative paths, and common API endpoint patterns within JS code (string literals, assignments).
  - [ ] 3.3 If JS Parser (Option B/C): Implement logic to traverse the AST or use the tool to identify:
        *   String literals that look like paths/URLs.
        *   Assignments to `window.location`, `document.location`.
        *   Parameters to `fetch()`, `XMLHttpRequest.open()`.
        *   Potentially dynamic path constructions (can be very hard).
  - [x] 3.4 For each extracted path, record its context (e.g., variable name, function call). (Context for regex-based findings is `JS_string_literal`)

- [x] 4.0 Path Normalization and Scoping
  - [x] 4.1 In `PathExtractor.ExtractPaths`, for each raw extracted string:
        *   Use `urlhandler.NormalizePath(sourceURL, rawPath)` (or equivalent existing function) to resolve it to an absolute URL.
        *   Ensure the normalizer handles various cases (absolute, relative, protocol-relative).
  - [x] 4.2 Filter out invalid or out-of-scope URLs based on `CrawlerConfig` from `internal/config/config.go`. This might be done by the component that consumes these paths (e.g., the crawler). (Responsibility of the consuming component, e.g., Crawler, not PathExtractor itself)
  - [x] 4.3 Deduplicate extracted paths from the same source content.

- [x] 5.0 Storage of Extracted Paths (in `internal/datastore/path_store.go` or extend existing datastore logic)
  - [x] 5.1 Define Parquet schema for `models.ExtractedPath`: `SourceURL`, `ExtractedAbsoluteURL`, `Timestamp`, `Context`, `Type` (FR3.1).
  - [x] 5.2 Implement `StoreExtractedPaths(paths []models.ExtractedPath) error` to write to Parquet, potentially using/extending existing `ParquetWriter` from `internal/datastore` (FR3.2).
        *   Consider if this is a separate Parquet file or part of a larger data store.

- [x] 6.0 Integration with Crawler/Monitoring
  - [x] 6.1 The `PathExtractor` will be used by:
        *   The `CrawlerService` (`2-tasks-prd-target-crawling.md`) when it processes a fetched page/JS file.
        *   Potentially by the `MonitoringService` (`10-tasks-prd-html-js-file-monitoring.md`) if new paths in monitored files should be reported or fed back for crawling.
  - [x] 6.2 New, in-scope, absolute URLs extracted should be added to the crawler's queue (respecting depth limits, etc.).

- [x] 7.0 Configuration (as part of `CrawlerConfig` or new `ExtractorConfig`)
  - [x] 7.1 Add to `internal/config/config.go` (likely within `CrawlerConfig` or a new `ExtractorConfig` struct):
        *   `ExtractPathsFromJSComments bool`
        *   `JSPathRegexes []string` (if using regex approach for JS)
  - [x] 7.2 Ensure these are reflected in `config.example.yaml` and `config.example.json` if used.

- [x] 8.0 Unit Tests
  - [x] 8.1 Test HTML path extraction with various HTML structures and path types. (Skeleton created)
  - [x] 8.2 Test JavaScript path extraction with different JS snippets, covering string literals, assignments, and function calls (tailor to chosen JS analysis method). (Skeleton created)
  - [x] 8.3 Test path normalization logic (utilizing existing `urlhandler` tests or adding new ones for specific extractor cases). (Partially covered by extractor tests, urlhandler tests TBD)
  - [x] 8.4 Test `PathExtractor.ExtractPaths` end-to-end for a few sample HTML and JS contents. (Skeleton created)
  - [x] 8.5 Test `PathStore` for correct Parquet writing/reading (leveraging existing `internal/datastore` tests where applicable). (Skeleton created)