# Product Requirements Document: HTML/JS Path Extraction

## 1. Introduction/Overview

This document details the requirements for the HTML/JS Path Extraction feature in Project MonsterInc. This feature leverages the `jsluice` tool to analyze the content of monitored HTML and JavaScript files (obtained via the `html-js-file-monitoring` feature) and extract URLs, relative paths, and other potentially interesting navigation or resource links. The primary aim is to help users discover new endpoints, identify new attack surfaces, and understand how different parts of a web application are interconnected.

The goal is to integrate `jsluice` as a Go library to parse HTML/JS content and store the extracted path-like information in a structured and queryable format.

## 2. Goals

*   Automatically extract URLs and relative paths from the content of monitored HTML and JavaScript files using `jsluice`.
*   Store the extracted path information efficiently, linking it back to the source file.
*   Normalize and process extracted paths for better usability.
*   Provide a mechanism to handle errors during the extraction process.
*   Focus on discovering new potential endpoints and attack surfaces.

## 3. User Stories

*   **As a Security Analyst, I want the tool to automatically analyze all monitored JavaScript files with `jsluice` and provide me with a list of extracted URLs and paths, so I can identify potential new API endpoints or hidden application areas.**
*   **As a Penetration Tester, I want to see all discovered relative paths from HTML pages, resolved against their base URLs, so I can map out the application structure and find resources that might not be directly linked from the main navigation.**
*   **As an Auditor, I want the system to log any errors encountered by `jsluice` during file processing, so I am aware of any gaps in the path extraction coverage.**

## 4. Functional Requirements

1.  The system **must** use the content of HTML and JavaScript files stored in the Parquet data store (from the `html-js-file-monitoring` feature) as input for `jsluice`.
2.  The system **must** integrate `jsluice` as a Go library/package to perform the extraction. (Refer to [BishopFox/jsluice on GitHub](https://github.com/BishopFox/jsluice) for library capabilities).
3.  For each processed file, the system **must** extract URLs and other path-like strings identified by `jsluice`.
4.  Extracted path information **must** be stored in a dedicated Parquet table/schema.
5.  Each extracted path record **must** include:
    *   A reference/link to the original HTML/JS file from which it was extracted.
    *   The extracted path/URL string itself.
    *   The type of path (e.g., absolute URL, relative path, as determined by `jsluice` or post-processing).
    *   Contextual information if provided by `jsluice` (e.g., the surrounding code snippet or type of finding).
6.  The system **must** perform normalization on extracted paths:
    *   Relative paths **must** be resolved to absolute URLs based on the URL of the source file.
    *   Duplicates (identical absolute URLs/paths from the same source file) **should** be handled (e.g., stored once or flagged).
7.  The system **must** support a configurable depth limit for path discovery if applicable through `jsluice` or post-processing logic (User specified: depth 5). This typically applies to recursive analysis if `jsluice` were to fetch and analyze linked resources, which is not the primary assumption here; rather, it might refer to the complexity of path reconstruction from code.
8.  The system **must not** attempt to verify the validity or accessibility of the extracted URLs/paths.
9.  **Error Handling & Reporting:**
    *   If `jsluice` encounters an error while processing a file, the error **must** be logged.
    *   The error status **should** be indicated in any relevant reporting or status dashboards.
    *   If `jsluice` finds no paths in a file, no path records **should** be created for that file, and this **should not** be treated as an error.
10. The system **should** be designed to handle potentially large HTML/JS files as input to `jsluice`, assuming `jsluice` itself is performant for such cases (as per user expectation).

## 5. Non-Goals (Out of Scope)

*   Executing JavaScript or actively crawling extracted URLs.
*   Deep recursive analysis beyond the configured depth or what `jsluice` inherently provides on a single file's content.
*   Guessing or fuzzing parameters for extracted URLs/paths.
*   Storing the full AST (Abstract Syntax Tree) from `jsluice` unless specific parts are used for context.

## 6. Design Considerations (Optional)

*   The Parquet schema for extracted paths should be optimized for queries based on source file URL, extracted path, and path type.
*   Consider how to present extracted paths to the user (e.g., in a searchable table, integrated into other reports).

## 7. Technical Considerations (Optional)

*   Ensure proper Go library usage of `jsluice`, including any necessary configuration options it provides.
*   Develop robust logic for resolving relative paths to absolute URLs, considering various base URL scenarios.
*   Logging for the `jsluice` integration should be detailed enough for troubleshooting.

## 8. Success Metrics

*   `jsluice` is successfully integrated and processes content from monitored HTML/JS files.
*   Extracted URLs and paths are accurately stored in the Parquet store with correct metadata.
*   Path normalization (especially relative to absolute resolution) functions correctly.
*   Errors from `jsluice` are logged and reported appropriately.
*   Users find the extracted path information useful for discovering new endpoints and attack surfaces.

## 9. Open Questions

*   Does `jsluice` provide mechanisms to control or infer "depth" in the context of static analysis of a single file, or is the depth limit of 5 intended for a potential future feature involving crawling/recursive analysis?
*   What specific contextual information does `jsluice` output for its findings, and how should this map to the "context" field in the Parquet store?
*   How should the system handle JavaScript files that are heavily obfuscated or minified? Will `jsluice` still be effective?
*   Are there specific `jsluice` configurations or matchers (beyond defaults) that should be enabled or disabled for this use case? 