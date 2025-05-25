# Package models

This package defines various data structures used throughout the MonsterInc application, particularly for representing scan results, configuration, and reporting data.

## Core Data Structures

### `ProbeResult` (`probe_result.go`)
Represents the detailed findings for a single probed URL. It includes:
- Basic info: `InputURL`, `Method`, `Timestamp`, `Duration`, `Error`, `RootTargetURL`.
- HTTP response: `StatusCode`, `ContentLength`, `ContentType`, `Headers`, `Body` (snippet), `Title`, `WebServer`.
- Redirect info: `FinalURL`.
- DNS info: `IPs`, `CNAMEs`, `ASN`, `ASNOrg`.
- Technology detection: `Technologies` (slice of `Technology` struct).
- TLS info: `TLSVersion`, `TLSCipher`, `TLSCertIssuer`, `TLSCertExpiry`.

### `ParquetProbeResult` (`parquet_schema.go`)
Defines the schema for data stored in Parquet files. Fields are mostly optional pointers (`*string`, `*int32`, etc.) to handle missing data gracefully in Parquet. It includes fields like `OriginalURL`, `FinalURL`, `StatusCode`, `ScanTimestamp`, `RootTargetURL`, etc. Arrays are marked for list representation in Parquet.

### `ReportPageData` (`report_data.go`)
This struct holds all data necessary for rendering the HTML report. Key fields include:
- `ReportTitle`, `GeneratedAt`.
- `ProbeResults`: A slice of `ProbeResultDisplay` for the main results table.
- `TotalResults`, `SuccessResults`, `FailedResults`.
- `UniqueStatusCodes`, `UniqueContentTypes`, `UniqueTechnologies`, `UniqueRootTargets`: For populating filter dropdowns.
- `CustomCSS`, `ReportJs`: For embedded assets.
- `ProbeResultsJSON`: Probe results marshaled to JSON for client-side JavaScript.
- `URLDiffs`: A map of `string` (RootTargetURL) to `URLDiffResult`, holding the diff data to be displayed in the report (e.g., for the "Old/Missing URLs" section).
- Configuration for display: `ItemsPerPage`, `EnableDataTables`.

### `ProbeResultDisplay` (`report_data.go`)
A struct tailored for displaying probe results in the HTML report. It often reformats or adds helper fields based on `ProbeResult`. Includes `URLStatus` (string) to show diff status (new, old, existing).

### `URLDiffResult`, `DiffedURL`, `URLStatus` (`url_diff.go`)
These structures are central to the URL diffing feature:
- **`URLStatus`**: An enum-like string (`new`, `old`, `existing`).
- **`DiffedURL`**: Contains a `NormalizedURL`, its `Status` (`URLStatus`), and `LastSeenData` (a `ProbeResult` struct, primarily for providing context to old/existing URLs).
- **`URLDiffResult`**: Holds the `RootTargetURL` for the diff and a slice of `DiffedURL` results.

### `Target` (`target.go`)
Represents a target URL, including its original and normalized forms.

### `ExtractedAsset` (`asset.go`)
Represents an asset (like a JS file, CSS file, image) extracted from a crawled page, including its `AbsoluteURL` and source tag/attribute.

### `URLValidationError` (`error.go`)
A custom error type for URL validation issues.

## Helper Functions

- `ToProbeResultDisplay(pr ProbeResult) ProbeResultDisplay`: Converts a `ProbeResult` to a `ProbeResultDisplay` for easier rendering in templates.
- `GetDefaultReportPageData() ReportPageData`: Returns a `ReportPageData` struct with some default values initialized.

## Purpose

Consolidating these models in one package promotes consistency and reusability across different modules of the application (crawler, httpxrunner, datastore, differ, reporter). 