# PRD: HTML Reporting for HTTP/S Probe Results (`httpx-html-reporting`)

## 1. Introduction/Overview

This document outlines the requirements for the "HTML Reporting" feature of the MonsterInc tool.

The primary purpose is to generate a user-friendly, interactive, and visually appealing static HTML report from the structured data collected by the `httpx-probing` module. This report will allow users to easily view, search, sort, and navigate the scan results.

## 2. Goals

*   To provide a clear, intuitive, and interactive HTML interface for reviewing HTTP/S probe results.
*   To enable powerful search and filtering capabilities across all collected data fields.
*   To allow sorting of results by key fields to facilitate analysis.
*   To support pagination for handling large result sets efficiently.
*   To generate a single, self-contained static HTML file per scan session for easy sharing and archiving.
*   To present data for multiple initial targets within a single scan session in a navigable way within the same report.
*   To achieve a modern and aesthetically pleasing user interface.

## 3. User Stories

*   As a security analyst, I want an interactive HTML report where I can quickly search for specific keywords across all fields, sort by status code, and paginate through results, so I can efficiently analyze the outcome of a large scan.
*   As a penetration tester, I want a visually clear report that uses client-side navigation to switch between results for different root targets within the same scan, so I can organize my findings easily.
*   As a team lead, I want a single static HTML file report that I can easily share with team members or archive, which includes all relevant data and is easy to understand.

## 4. Functional Requirements

1.  The system must accept a list of structured data objects (output from `httpx-probing`) as input. Each object represents a probed URL and its associated information.
2.  If the input list from `httpx-probing` is empty (no results to report), the system must *not* generate an HTML report and should log this event.
3.  The system must generate a single static HTML file as output for each scan session.
4.  **Data Display:** The HTML report must display all fields collected by `httpx-probing` for each URL in a tabular format. These fields include: Original URL (or an identifier if multiple root targets), FinalURL, StatusCode, ContentLength, ContentType, Title, ServerHeader, Technologies, IPAddress, CNAMERecord, and RedirectChain.
5.  **Search Functionality:**
    *   A global search input field must be provided that allows users to search for a string across all displayed data fields.
    *   For fields with a limited set of distinct values (low cardinality) observed in the current result set, such as `StatusCode` and `ContentType`, the report must provide dropdown menus for filtering. These dropdowns should only contain values actually present in the current data.
    *   For the `Technologies` field (which may contain a comma-separated list), the search should ideally allow filtering if any of the listed technologies match the search term.
6.  **Sorting Functionality:**
    *   The results table must be sortable by at least the following columns: `Title`, `StatusCode`.
    *   The default sort order for the results must be by the original input URL (or an equivalent identifier that maintains the order of initial targets if multiple were provided).
7.  **Pagination:**
    *   The report must implement client-side pagination for the results table.
    *   Each page must display a configurable number of results, defaulting to 10 items per page.
8.  **Multi-Target Navigation (within a single scan session):**
    *   If a scan session was initiated with multiple root target URLs, the single HTML report should provide a mechanism (e.g., a side menu, tabs) to navigate or filter the view to show results pertaining to each specific root target. This will likely involve client-side routing/path changes (e.g., using URL fragments `#target1`, `#target2`).
9.  **User Interface (UI) / User Experience (UX):**
    *   The report must have a modern and professional look and feel.
    *   Utilize the Bootstrap CSS framework (or a similar modern framework) for layout and components.
    *   Incorporate design elements such as rounded corners for containers/buttons, subtle box shadows for depth, and use of the "Nunito" font (or a similar clean, modern sans-serif font).
    *   Employ subtle gradients or transparent elements where appropriate to enhance visual appeal.
    *   Interactive elements (buttons, sortable headers, pagination links) must have clear hover and click visual feedback/effects.
    *   The report should be responsive and usable on common desktop screen sizes.

## 5. Non-Goals (Out of Scope)

*   The HTML report will not allow users to edit the displayed data.
*   The report will not save user preferences (like sort order or search queries) across sessions or page reloads.
*   Server-side rendering or processing for the report is not required; it will be a fully client-side application within the static HTML.
*   Advanced charting or data visualization beyond the tabular display.

## 6. Design Considerations (Optional)

*   Consider using a JavaScript library like DataTables.js (or a similar one compatible with the chosen CSS framework) to provide the table functionalities (search, sort, pagination) if it simplifies development and offers a good user experience.
*   Ensure all necessary CSS and JavaScript are either embedded within the HTML file or linked as relative paths if a small number of auxiliary files are absolutely necessary (preference for a single HTML file if feasible).

## 7. Technical Considerations (Optional)

*   Go's `html/template` package can be used to generate the HTML structure.
*   JavaScript will be required for interactivity (search, sort, pagination, client-side routing for multi-target views).
*   Performance considerations for rendering and interactivity, especially with larger datasets (though pagination helps).

## 8. Success Metrics

*   The generated HTML report accurately displays all data from the `httpx-probing` results.
*   Search, sort, and pagination functionalities work correctly and efficiently for datasets up to at least 1,000-5,000 entries.
*   The UI adheres to the specified modern design guidelines.
*   Navigation between different root targets (if applicable) within the report is intuitive.
*   The report is rendered correctly in modern web browsers (e.g., latest Chrome, Firefox, Edge).
*   A single HTML file is produced per scan (or minimal auxiliary files if absolutely unavoidable).

## 9. Open Questions

*   For the `Technologies` field display and search, if a technology contains spaces (e.g., "Apache Tomcat"), how should this be handled in parsing and search? (Assumption: Treat as a single technology string).
*   How should very long strings (e.g., a very long Title or RedirectChain) be displayed in the table to avoid breaking the layout? (e.g., truncation with a tooltip, or a scrollable cell). 