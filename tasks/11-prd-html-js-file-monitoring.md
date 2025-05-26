# Product Requirements Document: HTML/JS File Monitoring

## 1. Introduction/Overview

This document outlines the requirements for the HTML/JS File Monitoring feature within Project MonsterInc. The primary purpose of this feature is to track changes in specified HTML and JavaScript files over time. By regularly fetching and storing the content of these files, the tool will help users detect new URLs, new features, or new data embedded within them. This is crucial for identifying modifications, potential unauthorized changes, or new attack surfaces as web applications evolve.

The goal is to provide a configurable and scheduled mechanism to retrieve and store the content of designated HTML/JS files for subsequent analysis.

## 2. Goals

*   Enable users to specify a list of HTML/JS files (via URLs) to be monitored.
*   Allow users to configure the frequency of fetching for these files.
*   Reliably fetch the content of the specified HTML/JS files at scheduled intervals.
*   Store the fetched content efficiently, along with metadata such as timestamp, original URL, and a content hash.
*   Handle cases where specified files are inaccessible or very large.
*   Support dynamic updates to the list of monitored files.

## 3. User Stories

*   **As a Security Analyst, I want to provide a list of critical JavaScript files from our web applications and have the tool fetch their content daily, so I can later analyze them for changes that might introduce vulnerabilities or new endpoints.**
*   **As a Web Administrator, I want to monitor key HTML pages for unexpected modifications by scheduling regular content fetches, so that I can quickly detect defacements or unauthorized alterations.**
*   **As a Developer, I want the tool to read an updated list of files to monitor each time it runs its scheduled task, so I don't have to restart the tool to add or remove files from monitoring.**
*   **As an Operator, I want the system to log an error and send a Discord notification if a monitored file cannot be accessed, so I can investigate the issue.**

## 4. Functional Requirements

1.  The system **must** allow users to specify the HTML/JS files to be monitored by providing a list of URLs in a text file (similar to how targets are specified).
2.  The system **must** periodically re-read this text file at the time of a scheduled scan to identify any new URLs to monitor or URLs that might have been removed.
3.  The system **must** allow users to configure the monitoring frequency (e.g., cron expression or predefined intervals like hourly, daily, weekly).
4.  At each scheduled interval, for every specified URL, the system **must** attempt to fetch the content of the HTML/JS file.
5.  The fetched content **must** be stored in Parquet format.
6.  For each fetched file, the system **must** store the following metadata alongside the content:
    *   Timestamp of when the file was fetched.
    *   The original URL of the file.
    *   A hash of the file content (using a fast hashing algorithm suitable for potentially large files, e.g., XXH64, MurmurHash3).
7.  The system **must** only retrieve the content as plain text and **must not** attempt to parse, interpret, or execute the HTML/JS.
8.  There **should not** be a hard limit on the number of files that can be monitored or the size of individual files (within practical system limits).
9.  If a specified file URL is inaccessible (e.g., 404 error, DNS resolution failure, timeout), the system **must**:
    *   Log the error with relevant details (URL, timestamp, error type).
    *   Send a notification via Discord (leveraging the `discord-notifications` feature) detailing the failure.
10. If a file's content is very large (e.g., 1-20MB or more), the system **must** still attempt to fetch and store it. Consideration **should** be given to streaming the content to avoid excessive memory usage during fetch and hashing, rather than loading the entire file into memory at once.
11. The term "processing in parallel" for large files in this context refers to the ability of the system to monitor multiple files (some of which might be large) concurrently as part of its scheduled task, not necessarily breaking a single large file into chunks for fetching its *initial* content (unless this is a strategy for robust retrieval against unstable connections).

## 5. Non-Goals (Out of Scope)

*   Analyzing or interpreting the content of HTML/JS files within this feature (this is deferred to other features like content diffing, path extraction, or secret detection).
*   Executing JavaScript or rendering HTML.
*   Managing versions or history of the input text file containing the list of URLs (the tool simply reads the latest version at scan time).
*   Authenticated access to files (initially, all files are assumed to be publicly accessible via URL).

## 6. Design Considerations (Optional)

*   The file fetching mechanism should be resilient to network issues (e.g., configurable timeouts, retries for transient errors).
*   Storage in Parquet should be optimized for queries based on URL and timestamp.

## 7. Technical Considerations (Optional)

*   Use a robust HTTP client library for fetching file content.
*   Investigate efficient hashing libraries in Go for large data streams.
*   The scheduling mechanism should be reliable (e.g., using a well-tested Go scheduling library).
*   Ensure that concurrent fetching of multiple files is handled efficiently, respecting configurable concurrency limits to avoid overwhelming the network or the target servers.

## 8. Success Metrics

*   Files specified by the user are fetched according to the configured schedule.
*   Fetched content and associated metadata are accurately stored in Parquet format.
*   Errors during fetching are logged and reported via Discord as specified.
*   The system can handle updates to the list of monitored files without requiring a restart.
*   User feedback confirms that the feature helps in tracking changes to their HTML/JS assets.

## 9. Open Questions

*   What specific fast hashing algorithm is preferred (e.g., XXH3, BLAKE3 if performance on very large files is paramount and Go implementations are mature)?
*   Should there be a configurable user-agent for fetching these files?
*   How should the system behave if the input text file of URLs is missing or empty at scan time?
*   For the "chia nhỏ ra và xử lý đa luồng" (split and process in parallel) for very large files: does this imply fetching a large file in chunks, or simply that the broader system can fetch *multiple* large files in parallel if multiple are defined for monitoring? (Clarified in FR11 to mean the latter, but initial fetch is whole-file). 