# PRD: Target Input and Normalization (`target-input-normalization`)

## 1. Introduction/Overview

This document outlines the requirements for the "Target Input and Normalization" feature of the MonsterInc tool.

The primary purpose is to ingest a list of target URLs from a user-provided text file, normalize these URLs into a consistent and valid format, and filter out any invalid entries at the earliest stage. This ensures data integrity and reliability for subsequent processing modules (e.g., crawling, HTTP probing).

## 2. Goals

*   To ensure all URLs processed by downstream modules are in a consistent, standardized format.
*   To filter out and log invalid or un-parsable URLs immediately upon input.
*   To provide clear error reporting for issues encountered during file processing (e.g., file not found, empty file, permission issues) and URL normalization.
*   To seamlessly pass the list of valid, normalized URLs to the next processing module in memory.

## 3. User Stories

*   As a security professional, I want to provide a list of target URLs from a simple text file so that I can easily define the scope for a new scan.
*   As a system administrator, I want the tool to automatically normalize various URL formats (e.g., with or without scheme, mixed case domains) into a standard format so that I don't have to manually clean up the input list, ensuring consistency for scanning.
*   As a penetration tester, I want invalid URLs or file access issues to be clearly logged so that I can quickly identify and rectify problems with my input target list.

## 4. Functional Requirements

1.  The system must accept a file path as input, pointing to a text file containing target URLs.
2.  The input text file must contain one URL per line.
3.  The system must skip processing for any empty lines found in the input file.
4.  The system shall *not* support or interpret comment lines (e.g., lines starting with `#`); any such line will be treated as a regular URL input.
5.  For each non-empty line read from the input file, the system must attempt to normalize the URL according to the following rules:
    *   If no scheme (e.g., `http://`, `https://`) is present, prepend `http://` by default.
    *   Convert the scheme and the hostname components of the URL to lowercase.
    *   Remove any URL fragment (the part of the URL after a `#` symbol).
6.  If a URL from the input file cannot be parsed or fails normalization (e.g., it's fundamentally malformed), the system must:
    *   Skip this URL and not include it in the output list of normalized targets.
    *   Log an error message indicating the original problematic URL and the reason for the failure.
7.  The system must handle the following file-level error conditions by logging an appropriate error message and terminating the target loading process for the given file:
    *   If the specified input file is empty.
    *   If the specified input file does not exist at the given path.
    *   If the system lacks the necessary permissions to read the specified input file.
8.  The list of successfully parsed and normalized URLs shall be passed directly in memory to the next module responsible for processing these targets.
9.  The system should log a summary after processing an input file, including: total lines read, number of URLs successfully normalized, and number of URLs skipped due to errors.

## 5. Non-Goals (Out of Scope)

*   The system will *not* check if the URLs are live or reachable (i.e., no network requests will be made during this normalization stage).
*   The system will *not* perform de-duplication of URLs. Duplicates in the input file will result in duplicate entries in the normalized output.
*   The system will *not* attempt to guess or correct typographical errors in URLs beyond the defined normalization rules.
*   The system will *not* support any input formats other than a plain text file with one URL per line.

## 6. Design Considerations (Optional)

*   Error messages logged for invalid URLs or file issues should be clear and informative, aiding the user in troubleshooting their input.

## 7. Technical Considerations (Optional)

*   Utilize Go's standard `net/url` package for URL parsing and manipulation.
*   Employ standard file I/O operations for reading the input file.

## 8. Success Metrics

*   100% of syntactically valid URLs provided in an input file are correctly normalized according to the specified rules.
*   All malformed or un-parsable URLs are correctly identified, skipped, and logged with an informative error message.
*   File-level errors (empty file, file not found, permission denied) are correctly detected, logged, and result in appropriate termination of the loading process for that file.
*   The feature can efficiently process input files containing a large number of URLs (e.g., 10,000 URLs) within an acceptable time frame (e.g., under 5 seconds on standard hardware).

## 9. Open Questions

*   Should the default scheme be configurable (e.g., allow users to specify `https://` as the default)? (For now, `http://` is the hardcoded default).
*   How should the system behave if a line in the input file contains multiple space-separated strings that could be interpreted as multiple URLs? (Current assumption: The entire line is treated as a single URL string for parsing; if it fails, it's logged as one failed URL). 