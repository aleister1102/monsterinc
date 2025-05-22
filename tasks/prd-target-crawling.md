# PRD: Target Crawling (`target-crawling`)

## 1. Introduction/Overview

This document outlines the requirements for the "Target Crawling" feature of the MonsterInc tool.

The primary purpose is to systematically browse (crawl) the web application starting from a given set of normalized target URLs. This process aims to discover new endpoints, find linked JavaScript files, and gather all accessible internal links and resources within the defined scope of the target.

## 2. Goals

*   To discover a comprehensive set of URLs (pages, scripts, assets) associated with a given target web application.
*   To provide a configurable crawling experience, allowing users to define depth, concurrency, and scope.
*   To prepare a list of discovered URLs for subsequent analysis stages, such as HTTP probing.
*   To operate efficiently and respectfully, with options to manage request rates and identify the crawler.

## 3. User Stories

*   As a security auditor, I want to crawl a target website to a specified depth so that I can uncover all linked pages and resources for further security assessment.
*   As a reconnaissance specialist, I want to discover all JavaScript files and other assets linked from a target application so that I can analyze them for potential vulnerabilities or information leakage.
*   As a bug bounty hunter, I want the crawler to identify as many unique endpoints as possible within the target's scope so I can expand my attack surface.

## 4. Functional Requirements

1.  The system must accept a list of normalized URLs (from the `target-input-normalization` stage) as input for crawling.
2.  For each input URL, the system shall initiate a crawl.
3.  **Scope Control:**
    *   By default, the crawler must only visit URLs that are on the same hostname as the initial seed URL provided for the crawl.
    *   An option (`IncludeSubs`, default: `false`) must be available to allow crawling subdomains of the initial seed URL's hostname.
    *   The crawler must *not* follow links to external domains (e.g., if crawling `example.com`, it should not follow a link to `google.com`).
    *   An option (`RespectRobotsTxt`, default: `false`) must be available. If set to `false` (default), the crawler will ignore `robots.txt` directives. If `true`, it will attempt to honor them.
    *   An option (`CrawlInsidePath`, default: `false`) must be available. If `true`, and the seed URL has a path (e.g., `http://example.com/blog/`), the crawler will only visit URLs under that initial path.
4.  **Crawl Parameters (Configurable, with defaults):**
    *   `MaxDepth`: Maximum depth to crawl. (Default: `5`).
    *   `Threads`: Number of concurrent crawling threads. (Default: `10`).
    *   `UserAgent`: User-Agent string for crawler requests. (Default: "MonsterIncCrawler/1.0").
    *   `RequestTimeout`: Timeout for each HTTP request made by the crawler. (Default: 10 seconds).
    *   `Delay`: Delay between requests for a given thread/domain. (Default: 0 milliseconds).
    *   `ProxyURL`: Optional HTTP/S proxy URL to route crawl traffic through.
    *   `CustomHeaders`: Optional custom HTTP headers to include in every crawler request.
    *   `InsecureTLS`: Optional flag to disable TLS certificate verification (Default: `false`).
5.  **Asset Discovery:** The crawler must attempt to extract URLs from:
    *   `<a>` tag `href` attributes.
    *   `<script>` tag `src` attributes.
    *   `<link>` tag `href` attributes (e.g., for CSS files).
    *   `<img>` tag `src` attributes.
    *   `<form>` tag `action` attributes.
    *   *(Consideration for future: URLs within comments, inline JavaScript, CSS files)*
6.  **HTTP Error Handling:**
    *   If an HTTP request during crawling results in a 4xx (Client Error) or 5xx (Server Error) status code, the crawler must skip that URL, log the error (including the URL and status code), and not attempt to retry it.
7.  **Output:**
    *   The crawler must produce a de-duplicated list of all unique absolute URLs discovered within the defined scope and parameters.
    *   This list of discovered URLs should be passed in memory to the next processing module (e.g., `httpx-probing`).
    *   The discovered URLs should also be available for storage to enable comparison with future crawls (as part of `crawl-result-diffing`).
8.  The crawler should log its startup parameters (seed URL, depth, threads, etc.) and a summary upon completion (total URLs visited, total unique URLs discovered).

## 5. Non-Goals (Out of Scope)

*   The crawler will *not* submit any forms.
*   The crawler will *not* execute JavaScript found on pages to discover dynamically generated links in this phase. (This might be a separate, more advanced feature or handled by a different module like `jsluice`).
*   The crawler will *not* attempt to authenticate to websites.
*   The crawler will *not* perform any vulnerability scanning itself.

## 6. Design Considerations (Optional)

*   The implementation should use a robust crawling library like `gocolly`.
*   Efficient de-duplication of found URLs is important.
*   Clear logging of visited URLs, discovered assets, and any errors encountered.

## 7. Technical Considerations (Optional)

*   Based on the `gocolly` library.
*   Mechanism to manage and respect `MaxDepth` and `AllowedDomains` (and `IncludeSubs`) correctly.
*   Concurrency management using `gocolly`'s built-in mechanisms.

## 8. Success Metrics

*   The crawler successfully discovers over 90% of accessible static links (hrefs, srcs) on a test website within the specified `MaxDepth`.
*   The crawler adheres to scope limitations (hostname, subdomains if enabled, path restrictions if enabled).
*   All discovered URLs are correctly passed to the next stage.
*   The crawler correctly handles and logs HTTP 4xx/5xx errors without crashing.
*   Configurable parameters (depth, threads, user-agent, etc.) are correctly applied during crawling.

## 9. Open Questions

*   Should there be a configurable limit on the page content size to download to avoid memory issues with extremely large pages? (Default: No limit, but `RequestTimeout` provides some protection).
*   How to handle non-HTML content (e.g., PDF, DOCX files) linked from pages? Should their URLs be collected? (Current assumption: Yes, collect the URL. Probing will determine content type).
*   Should there be an option to only collect URLs of certain file extensions? (For now, collect all valid URLs found). 