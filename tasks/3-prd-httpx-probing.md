# PRD: HTTP/S Probing (`httpx-probing`)

## 1. Introduction/Overview

This document outlines the requirements for the "HTTP/S Probing" feature of the MonsterInc tool.

The primary purpose of this feature is to take a list of URLs (typically discovered by the `target-crawling` module), send HTTP/S requests to each, and extract key information about the responses. This mimics the functionality of tools like `httpx` with a specific set of data extraction options.

## 2. Goals

*   To determine the status and gather essential metadata for a large list of URLs efficiently.
*   To extract specific data points from HTTP responses, including status code, content length, content type, page title, server headers, IP address, CNAME records, and detected technologies.
*   To provide configurable options for the probing process, such as concurrency and request timeouts.
*   To produce structured data for each probed URL, suitable for reporting and storage.

## 3. User Stories

*   As a security analyst, I want to quickly probe all URLs discovered by the crawler to get their status codes, server types, and page titles, so I can prioritize further investigation.
*   As a reconnaissance specialist, I want to identify the technologies running on discovered web servers so I can tailor my subsequent testing methodologies.
*   As a system administrator, I want to verify the reachability and CNAME records for a list of internal web assets after a migration.

## 4. Functional Requirements

1.  The system must accept a list of unique URLs (output from the `target-crawling` module) as input.
2.  For each input URL, the system must perform an HTTP/S request.
3.  **Data Extraction (corresponding to `httpx` flags):** The system must attempt to extract the following information for each successfully probed URL:
    *   `StatusCode` (from `-sc`): The HTTP status code of the response.
    *   `ContentLength` (from `-cl`): The value of the `Content-Length` header.
    *   `ContentType` (from `-ct`): The value of the `Content-Type` header.
    *   `Title` (from `-title`): The content of the `<title>` tag from HTML responses.
    *   `ServerHeader` (from `-server`): The value of the `Server` HTTP header.
    *   `Technologies` (from `-td`): A list of web technologies detected on the page/server. This will be based on integrating or emulating a Wappalyzer-like engine.
    *   `IPAddress` (from `-ip`): The resolved IP address(es) of the hostname.
    *   `CNAMERecord` (from `-cname`): The CNAME record(s) for the hostname, if any.
    *   `FinalURL` (related to `-fr`): If redirects are followed, this is the URL of the final response.
    *   `RedirectChain`: (related to `-fr`): A list of URLs in the redirect chain, if any.
4.  **Control Options (Configurable, with defaults):**
    *   `Threads` (from `-t`): Number of concurrent probing threads. (Default: `40`, configurable).
    *   `RequestTimeout`: Timeout for each individual HTTP/S request. (Default: 10 seconds, configurable).
    *   `FollowRedirects` (from `-fr`): Boolean, whether to follow HTTP redirects. (Default: `true`). If true, data should be extracted from the final response after all redirects.
    *   `MaxRedirects`: Maximum number of redirects to follow for a single URL. (Default: 10, configurable).
    *   `UserAgent`: User-Agent string for probing requests. (Default: "MonsterIncProber/1.0", configurable).
    *   `ProxyURL`: Optional HTTP/S proxy URL to route probing traffic through. (Configurable).
    *   `CustomHeaders`: Optional custom HTTP headers to include in every probing request. (Configurable).
    *   `InsecureTLS`: Optional flag to disable TLS certificate verification. (Default: `false`, configurable).
5.  **Output Structure:**
    *   For each probed URL, the system must produce a structured data object (e.g., a Go struct) containing all the extracted fields mentioned above. If a piece of information cannot be retrieved (e.g., no title tag, CNAME not found), the corresponding field in the struct should be empty or indicate absence appropriately.
    *   This list of structured data objects must be passed in memory to subsequent modules (`httpx-html-reporting` and `httpx-parquet-storage`).
6.  **Error Handling:**
    *   If a URL probe results in a timeout, it should be logged, and the corresponding output struct should indicate the timeout (e.g., a specific error field or a special status code).
    *   If a URL probe results in a connection error (e.g., connection refused, DNS lookup failed), it should be logged, and the output struct should reflect this error.
    *   The system should continue processing other URLs even if some individual probes fail.

## 5. Non-Goals (Out of Scope)

*   This module will not perform any form of active vulnerability scanning or fuzzing.
*   This module will not attempt to bypass Web Application Firewalls (WAFs).
*   While it detects technologies, it will not assess the vulnerability status of those technologies.
*   The `-nc` (no-color) option is not directly applicable as the primary output is structured data, not direct console text.

## 6. Design Considerations (Optional)

*   The HTTP client used should be robust and highly configurable (e.g., Go's standard `net/http` with appropriate settings).
*   The technology detection engine (`-td`) should be modular to allow for updates to its rules/signatures. Integration with or inspiration from `httpx`'s use of `wappalyzer` technologies is key.

## 7. Technical Considerations (Optional)

*   Efficient concurrency management for a large number of URLs.
*   Careful handling of HTTP client settings, especially timeouts and redirect policies.
*   For CNAME and IP lookups, standard Go DNS resolution functions can be used.

## 8. Success Metrics

*   The system correctly extracts all specified data fields (status code, title, server, IP, CNAME, tech) for over 95% of successfully responding URLs in a test set.
*   Configurable options (threads, timeout, follow-redirects) are correctly applied and impact the probing behavior as expected.
*   The system efficiently processes a large list of URLs (e.g., 10,000 URLs) within a reasonable timeframe.
*   Failures (timeouts, connection errors) for individual URLs are logged appropriately without halting the processing of other URLs.

## 9. Open Questions

*   For `-td` (Tech Detected), what is the expected format of the output? A list of strings?
*   When `FollowRedirects` is true, should we store information about intermediate redirect responses, or only the final one? (Current PRD suggests `FinalURL` and `RedirectChain`).
*   Are there specific HTTP methods to use for probing (e.g., GET, HEAD)? (Current assumption: GET by default, HEAD as a configurable option perhaps for quicker checks). 