# Product Requirements Document: Real-time Secret Detection during Crawling

## 1. Introduction/Overview

This document outlines the requirements for the Real-time Secret Detection feature within Project MonsterInc. The primary objective is to identify and report sensitive information (secrets) such as API keys, tokens, and credentials that are inadvertently embedded in the content of HTTP responses during the crawling phase. This is achieved by scanning the body of relevant responses using custom regular expressions inspired by `mantra`.

The goal is to provide an automated, real-time mechanism to scan HTML/JS content for known secret patterns, store these findings, and immediately notify users of critical discoveries.

## 2. Goals

*   Automatically detect secrets in real-time within the body of HTTP responses from the crawler.
*   Incorporate custom regex patterns from `mantra` to ensure comprehensive detection coverage.
*   Store detected secret information, including the source URL, secret type, the secret itself, and contextual snippets.
*   Compile all findings into the final HTML scan report.
*   Send immediate notifications via Discord when secrets are discovered.
*   Enable users to take prompt action on discovered secrets based on timely alerts.

## 3. User Stories

*   **As a Security Engineer, I want the tool to automatically scan every HTML and JavaScript response during a crawl for hardcoded API keys and credentials, so I can identify and report them for revocation immediately.**
*   **As a Penetration Tester, I want to receive instant Discord alerts with details about any discovered secrets the moment the crawler finds them, allowing me to investigate these potential vulnerabilities while the scan is still in progress.**
*   **As an Auditor, I want a clear, consolidated section in the final scan report detailing all secrets found during the crawl, including where they were found (URL and line number) and the snippet of code containing the secret, so I can track remediation efforts.**

## 4. Functional Requirements

1.  The system **must** use the body of an HTTP response, received by the crawler, as input for secret detection.
2.  The system **must** check the `Content-Type` header of the HTTP response and only perform secret detection on the following types: `text/html`, `application/javascript`, `application/x-javascript`.
3.  The system **must** use a configurable set of regular expressions, primarily sourced from `mantra`, to perform the scan.
4.  Detected secrets **must** be stored in a dedicated Parquet table/schema for later analysis and inclusion in reports.
5.  Each detected secret record **must** include:
    *   The source URL where the secret was found.
    *   The line number(s) where the secret was found.
    *   The actual secret string (or a redacted version).
    *   The name/ID of the specific regex rule that triggered the finding.
    *   A snippet of the code/text surrounding the secret for context.
    *   A severity or confidence level associated with the rule.
6.  The system **must not** attempt to validate the functionality of detected secrets (e.g., by trying to use an API key).
7.  The system **must** be designed to handle potentially large response bodies without significantly impacting crawler performance.
8.  **Reporting and Notifications:**
    *   All findings from the secret detection process **must** be compiled and included in a dedicated section of the final HTML scan report.
    *   Upon detecting a secret, the system **must** send an immediate notification to the configured Discord webhook. This notification should contain the details outlined in FR5.

## 5. Non-Goals (Out of Scope)

*   This feature is not a post-processing step; detection happens in real-time during the crawl.
*   Automated remediation or revocation of secrets.
*   Complex workflow for managing the lifecycle of a detected secret beyond initial detection and reporting.
*   Scanning content types other than those specified in FR2.

## 6. Design Considerations (Optional)

*   The Parquet schema for detected secrets should be optimized for queries by source URL, rule triggered, and severity.
*   The presentation of secrets in the HTML report and Discord notifications should be clear and highlight the sensitive nature of the findings.
*   The secret scanning process should run in a separate goroutine from the main crawler response handling to minimize impact on crawling speed.

## 7. Technical Considerations (Optional)

*   Develop a clear process for defining, managing, and updating the custom regex patterns.
*   Ensure secure handling of any intermediate data or logs that might contain detected secrets (e.g., avoid plaintext logging of secrets where possible).
*   Performance testing with large response bodies and a large number of regex rules will be important.

## 8. Success Metrics

*   Successful integration of real-time regex scanning within the crawler's response handling flow.
*   Accurate detection and storage of secrets from in-scope HTTP responses.
*   Secret detection findings are clearly and effectively presented in the final HTML report.
*   Discord notifications are sent promptly upon secret discovery.
*   Minimal performance degradation of the crawling process.

## 9. Open Questions

*   How should the system handle heavily minified or obfuscated JavaScript? Will regex scanning still be effective?
*   Should there be a global on/off switch for this real-time scanning feature within the configuration?
*   How will updates to the `mantra` regex set be managed within the project?
*   To avoid notification fatigue, should we send one summary notification at the end of the scan in addition to (or instead of) instant notifications? For now, the requirement is for instant notifications. 