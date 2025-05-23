# Product Requirements Document: HTML/JS Secret Detection

## 1. Introduction/Overview

This document outlines the requirements for the HTML/JS Secret Detection feature within Project MonsterInc. The primary objective of this feature is to identify and report sensitive information (secrets) such as API keys, tokens, passwords, and other credentials inadvertently embedded in the content of monitored HTML and JavaScript files. This is achieved by leveraging the `trufflehog` tool and custom regular expressions inspired by `mantra`.

The goal is to provide an automated mechanism to scan HTML/JS content for known secret patterns, store these findings, and integrate them into existing reporting structures, enabling users to promptly address and remediate potential data leaks.

## 2. Goals

*   Automatically detect secrets within the content of monitored HTML and JavaScript files.
*   Utilize `trufflehog` (potentially its core libraries) for secret detection.
*   Incorporate custom regex patterns (inspired by `mantra`) via `trufflehog`'s custom detector capabilities.
*   Store detected secret information, including source file, location, and relevant code snippets, in a Parquet-based format.
*   Integrate secret detection findings into the project's existing reporting mechanisms.
*   Enable users to identify and take action (e.g., revoke credentials) based on the reported secrets.

## 3. User Stories

*   **As a Security Engineer, I want the tool to automatically scan all monitored JavaScript and HTML files for hardcoded API keys, private tokens, and other credentials using `trufflehog` and `mantra`-based regexes, so I can quickly identify and report them for revocation.**
*   **As a Developer, I want to be informed if any secrets are detected in the HTML/JS files I am working on before they are deployed, so I can remove them and prevent accidental exposure.**
*   **As an Auditor, I want a clear record of all secrets found, including where they were found (file and line number) and the snippet of code containing the secret, so I can track remediation efforts.**

## 4. Functional Requirements

1.  The system **must** use the content of HTML and JavaScript files stored in the Parquet data store (from the `html-js-file-monitoring` feature) as input for secret detection.
2.  The system **must** integrate `trufflehog` to perform secret scanning. Preference is to use `trufflehog` as a Go library if its core functionalities are exposed and suitable for integration; otherwise, it can be invoked as a command-line tool. (Refer to [trufflesecurity/trufflehog on GitHub](https://github.com/trufflesecurity/trufflehog)).
3.  The system **must** allow the definition and use of custom regular expressions (inspired by or directly from `mantra`) via `trufflehog`'s custom detector mechanism. (Trufflehog documentation on custom detectors should be consulted for implementation, e.g., `pkg/custom_detectors/CUSTOM_DETECTORS.md` within its repository).
4.  Detected secrets **must** be stored in a dedicated Parquet table/schema.
5.  Each detected secret record **must** include at least:
    *   A reference/link to the original HTML/JS file from which it was extracted.
    *   The line number(s) where the secret was found.
    *   The actual secret string or a redacted version if appropriate (TBD, `trufflehog` might provide this).
    *   The specific rule or detector (e.g., `trufflehog` built-in, custom regex name) that triggered the finding.
    *   A snippet of the code/text surrounding the secret for context.
    *   Severity or confidence level, if provided by `trufflehog`.
6.  The system **must not** attempt to validate the functionality of detected secrets (e.g., by trying to use an API key).
7.  Findings from the secret detection process **must** be integrated into the existing reporting framework of Project MonsterInc. This might involve adding a section to a general scan report or flagging files with detected secrets.
8.  The system **should** be designed to handle potentially large HTML/JS files as input, assuming the underlying tools (`trufflehog`) are performant for such cases.
9.  Consideration **should** be given to mechanisms for users to mark specific findings as false positives, though the full implementation of a false positive management system might be a separate effort.

## 5. Non-Goals (Out of Scope)

*   Real-time secret detection during code editing (focus is on scanned, monitored files).
*   Automated remediation or revocation of secrets.
*   Complex workflow for managing the lifecycle of a detected secret (e.g., assignment, tracking status) beyond initial detection and reporting.
*   Scanning non-HTML/JS files unless explicitly added to the scope of `html-js-file-monitoring` and deemed suitable for `trufflehog` text-based analysis.

## 6. Design Considerations (Optional)

*   The Parquet schema for detected secrets should be optimized for queries by source file, rule triggered, and severity.
*   The presentation of secrets in reports should be clear, highlighting the sensitive nature of the findings.
*   If `trufflehog` offers different levels of scan intensity or rule sets, these could be configurable.

## 7. Technical Considerations (Optional)

*   Thoroughly evaluate `trufflehog`'s Go library options vs. CLI invocation for robustness and flexibility.
*   Develop a clear process for defining, managing, and updating the custom regex patterns for `trufflehog`.
*   Ensure secure handling of any intermediate data or logs that might contain detected secrets before they are stored in Parquet (e.g., avoid plaintext logging of secrets where possible).
*   Performance testing with large files and a large number of regex rules will be important.

## 8. Success Metrics

*   Successful integration of `trufflehog` and custom regex scanning.
*   Accurate detection and storage of secrets from HTML/JS files.
*   Secret detection findings are clearly and effectively presented within existing reports.
*   Users can use the reported information to take action on exposed secrets.
*   Low rate of false positives from well-tuned regexes and `trufflehog` configurations.

## 9. Open Questions

*   What specific format does `trufflehog` use for its custom detectors, and how will `mantra` regexes be translated into this format?
*   Does `trufflehog` provide confidence scores or severity levels for its findings? How should these be used?
*   How will updates to `trufflehog` or the `mantra` regex set be managed within the project?
*   What is the strategy for minimizing false positives? Will there be an initial tuning phase?
*   Should high-severity secret findings trigger immediate notifications (e.g., via the `discord-notifications` feature) in addition to being in reports? 