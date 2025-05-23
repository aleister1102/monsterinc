# Product Requirements Document: HTML/JS Content Diff Reporting

## 1. Introduction/Overview

This document specifies the requirements for the HTML/JS Content Diff Reporting feature of Project MonsterInc. Building upon the `html-js-file-monitoring` feature, this functionality will compare different versions of monitored HTML and JavaScript files and generate user-friendly HTML reports visualizing these differences. This allows users to quickly identify and analyze changes between scans, aiding in security reviews, change tracking, and incident response.

The goal is to provide an automated way to generate and display intuitive "diff views" of content changes in monitored HTML/JS files, highlighting additions, deletions, and modifications between the current and previous scan.

## 2. Goals

*   Compare the content of a monitored HTML/JS file from the current scan with its version from the immediately preceding scan.
*   Generate an HTML report displaying a side-by-side diff view of the changes.
*   Allow users to easily navigate and understand the reported differences.
*   Ignore insignificant changes like whitespace and comments to focus on substantive alterations.
*   Effectively handle new, deleted, and very large files.

## 3. User Stories

*   **As a Security Analyst, I want to view a side-by-side diff report for a JavaScript file that has changed, with whitespace and comment changes ignored, so I can quickly identify actual code modifications and assess their security implications.**
*   **As a Web Developer, I want to easily navigate between detected changes in a large HTML file's diff report and collapse sections with no changes, so I can efficiently review modifications without being overwhelmed by unchanged content.**
*   **As an Auditor, when a file is reported as new, I want the diff report to clearly show its full content as the "current" version, so I can perform an initial review.**
*   **As an Incident Responder, if a monitored file is detected as deleted, I want the report to clearly indicate this, so I understand that the asset is no longer present.**

## 4. Functional Requirements

1.  The system **must** use the Parquet data store (created by the `html-js-file-monitoring` feature) as the source for file content.
2.  For each monitored HTML/JS file that has changed, the system **must** compare the content from the current scan with the content from the immediately previous scan.
3.  The comparison **must** ignore differences in whitespace (e.g., indentation, trailing spaces, blank lines) and comments (e.g., `// comment`, `/* comment */` for JS; `<!-- comment -->` for HTML).
4.  The system **must** generate an HTML report for each changed file, displaying a side-by-side diff view.
5.  The diff view **must** clearly distinguish between added, deleted, and modified lines/content.
6.  The HTML report **must** provide interactive features:
    *   Ability to collapse/expand sections of the diff that have no changes.
    *   Controls to navigate between different change hunks (e.g., "next change," "previous change").
7.  **Handling New Files:** If a file is present in the current scan but not in the previous one (a new file):
    *   The report **must** indicate that it's a new file.
    *   The side-by-side view **should** display the content of the new file on the "current" side, with the "previous" side being empty or explicitly marked as non-existent.
8.  **Handling Deleted Files:** If a file was present in the previous scan but not in the current one (a deleted file):
    *   The report **must** indicate that the file has been deleted.
    *   A diff view is not applicable; instead, the report **may** show the content of the old file with a clear "deleted" status.
9.  **Handling Large Files:**
    *   Before performing the diff, the system **should** beautify/prettify the HTML and JavaScript content to ensure a consistent format, which can improve the accuracy and readability of the diff.
    *   If a beautified file's content is very large (e.g., exceeds 20MB and a configurable number of lines), the system **must** employ strategies for efficient diff generation and presentation. This may include:
        *   Generating the diff in chunks or sections if a full in-browser rendering is too resource-intensive.
        *   Utilizing server-side diff generation if client-side processing is too slow.
        *   Potentially notifying the user that the file is very large and the diff might be simplified or truncated if full rendering is not feasible.
        *   The concept of "splitting the file into smaller files for parallel comparison" implies that the diffing process itself, or the presentation of the diff, might be broken down into manageable segments processed or displayed independently for performance reasons.
10. The generated HTML diff report **should** be self-contained or link to easily deployable assets (CSS, JS for interactivity).
11. This feature is specifically for HTML and JS files. Other file types are out of scope for diffing.

## 5. Non-Goals (Out of Scope)

*   Comparing arbitrary versions of files (only current vs. previous).
*   Three-way diffs or merge functionalities.
*   Saving or versioning the diff reports themselves beyond their generation upon request or as part of a scan summary.
*   Semantic diffing (understanding the code structure beyond textual changes) – the diff is primarily text-based after normalization (beautifying, ignoring comments/whitespace).
*   Generating diffs for non-textual changes within HTML (e.g., image binary changes, if images were embedded, which is not the focus).

## 6. Design Considerations (Optional)

*   The HTML diff report should be clean, readable, and use intuitive color-coding for additions, deletions, and modifications (e.g., green for additions, red for deletions).
*   Consider using established JavaScript diffing libraries (e.g., Diff2HTML, Monaco Editor's diff algorithm) for generating the visual diff.
*   The report should be responsive and usable on different screen sizes.
*   Line numbers should be displayed for both sides of the diff.

## 7. Technical Considerations (Optional)

*   Investigate Golang libraries for HTML/JS beautification.
*   Research efficient text diffing algorithms and libraries in Go or JavaScript (if rendering client-side).
*   The process of ignoring comments and whitespace needs to be robust for both HTML and JS syntax.
*   For very large files, server-side generation of the diff data (to be rendered by client-side JS) might be more performant than sending two large files to the client for diffing in the browser.

## 8. Success Metrics

*   Generated diff reports accurately reflect changes between file versions, correctly ignoring whitespace and comments.
*   Users find the diff reports easy to understand and navigate.
*   The system handles new, deleted, and very large files gracefully as per requirements.
*   Performance of diff generation and display is acceptable, even for large files.

## 9. Open Questions

*   What are the exact thresholds (file size, number of lines) for a file to be considered "very large" and trigger special handling for diffing?
*   Should there be an option to view the raw diff (e.g., unified diff format) in addition to the visual side-by-side HTML report?
*   How should character encoding differences between file versions be handled, if at all, before diffing?
*   For beautification of HTML/JS, are there preferred libraries or style guidelines to adhere to for consistency?
*   Regarding "splitting files for parallel comparison" for very large files: what is the expected output for the user? A single report with chunked diffs, or multiple separate diff reports? (FR9 attempts to clarify this as handling within a single report context). 