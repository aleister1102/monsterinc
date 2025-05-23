# PRD: Parquet Storage for HTTP/S Probe Results (`httpx-parquet-storage`)

## 1. Introduction/Overview

This document outlines the requirements for the "Parquet Storage" feature of the MonsterInc tool.

The primary purpose is to efficiently store the structured data collected by the `httpx-probing` module in Apache Parquet format. This facilitates optimized storage, faster querying with external data analysis tools, and enables historical data analysis and comparison across different scan sessions.

## 2. Goals

*   To significantly reduce the storage footprint of scan results compared to formats like JSON or CSV.
*   To enable efficient data loading and querying by external data analysis tools (e.g., DuckDB, Apache Spark, Pandas).
*   To persist all relevant data fields collected during the probing phase for comprehensive future analysis.
*   To organize stored data logically, for instance, by scan date.

## 3. User Stories

*   As a data analyst, I want scan results to be stored in Parquet format so that I can easily ingest and perform complex queries on large datasets using tools like DuckDB or Spark for trend analysis and reporting.
*   As a security team manager, I want historical scan data to be stored compactly and efficiently so that we can maintain a long-term archive of findings without excessive storage costs.
*   As a developer integrating with MonsterInc, I want a well-defined Parquet schema for scan results so that I can build automated data processing pipelines.

## 4. Functional Requirements

1.  The system must accept a list of structured data objects (output from `httpx-probing`) as input.
2.  If the input list from `httpx-probing` is empty (no results to store), the system must *not* generate a Parquet file and should log this event.
3.  **Data to Store:** All data fields collected and structured by the `httpx-probing` module must be stored in the Parquet file. This includes:
    *   `OriginalURL` (string): The URL that was actually probed.
    *   `FinalURL` (string): The URL after all redirects (if `FollowRedirects` was enabled).
    *   `StatusCode` (int32): HTTP status code.
    *   `ContentLength` (int64): Content length from header.
    *   `ContentType` (string): Content type from header.
    *   `Title` (string): HTML page title.
    *   `ServerHeader` (string): Server HTTP header.
    *   `Technologies` (list of strings): Detected web technologies.
    *   `IPAddress` (list of strings): Resolved IP addresses.
    *   `CNAMERecord` (list of strings): CNAME records.
    *   `RedirectChain` (list of strings): List of URLs in the redirect chain.
    *   `ScanTimestamp` (timestamp): Timestamp indicating when the scan for this URL occurred or when the scan session started.
    *   `RootTargetURL` (string): The initial root target URL that led to the discovery of this `OriginalURL`.
    *   `ProbeError` (string): Any error message encountered during the probing of this specific `OriginalURL` (e.g., "timeout", "connection refused").
4.  **Parquet Schema:** The Parquet file schema must accurately reflect the fields and their appropriate data types (e.g., String, Int32, Int64, List<String>, Timestamp). Fields that might not always be present (e.g., `Title` for non-HTML content, `CNAMERecord`) must be nullable.
5.  **File Naming and Location:**
    *   Parquet files must be stored in a base directory named `data`.
    *   Within the `data` directory, subdirectories must be created based on the date of the scan in `YYYYMMDD` format (e.g., `data/20240115/`).
    *   Each scan session must generate a new, separate Parquet file.
    *   The filename for each Parquet file should be unique and indicative of the scan, e.g., `scan_results_<timestamp_of_scan_start>.parquet` (e.g., `scan_results_20240115_143000.parquet`).
6.  **Compression:** The Parquet files must be written using Zstandard compression.
7.  **Error Handling:** If an error occurs during the writing of a Parquet file (e.g., I/O error, disk full), the system must log a detailed error message. It should attempt to complete storing other data if possible, or manage the error gracefully without crashing.

## 5. Non-Goals (Out of Scope)

*   The MonsterInc tool itself will not provide an interface for directly querying or viewing data within the Parquet files.
*   The system will not support appending data to existing Parquet files; each scan session creates a new file.
*   The system will not manage Parquet file lifecycle (e.g., deletion of old files, archiving).

## 6. Design Considerations (Optional)

*   A robust Go library for writing Parquet files should be used (e.g., `github.com/xitongsys/parquet-go` or alternatives).
*   The schema definition should be centralized and easily maintainable.

## 7. Technical Considerations (Optional)

*   Ensure proper mapping of Go data types to Parquet data types.
*   Efficiently handle writing potentially large lists of results to the Parquet file.

## 8. Success Metrics

*   All specified data fields from `httpx-probing` are accurately written to the Parquet files with the correct schema and Zstandard compression.
*   Generated Parquet files are valid and can be successfully read and queried by standard Parquet-compatible tools like DuckDB and Apache Spark/Pandas.
*   The storage size of the Parquet files shows a significant reduction compared to equivalent uncompressed JSON or CSV data.
*   File naming and directory structure conventions are correctly implemented.

## 9. Open Questions

*   What specific timestamp should `ScanTimestamp` represent? The start of the entire scan session, or the specific moment an individual URL was probed? (Assumption: Start of the scan session for grouping records belonging to the same run).
*   If a scan session involves multiple `RootTargetURL`s, are all results from that session written to a single Parquet file, with `RootTargetURL` differentiating them? (Assumption: Yes, one Parquet file per scan session). 