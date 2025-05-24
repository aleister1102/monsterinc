# `urlhandler` Package Documentation

## Overview

The `urlhandler` package is responsible for processing, validating, normalizing, and reading URLs from files. It ensures that URLs conform to expected standards before being used by other parts of the MonsterInc application.

Key functionalities include:
- Validating URL strings.
- Normalizing URLs (e.g., adding default schemes, lowercasing hostnames, removing fragments).
- Reading lists of URLs from text files.
- Resolving relative URLs against a base URL.
- Checking domain/subdomain relationships.

## Core Functions

### URL Validation and Normalization

1.  **`NormalizeURL(rawURL string) (string, error)`**
    *   **Purpose:** Takes a raw URL string and applies a series of normalization rules.
    *   **Operation:**
        1.  Trims leading/trailing whitespace from the input string.
        2.  Returns an error (`*models.URLValidationError`) if the input is empty.
        3.  Parses the URL. If parsing fails, returns an error.
        4.  If no scheme (e.g., `http://`, `https://`) is present, it prepends `http://` by default and re-parses the URL.
        5.  Converts the scheme and hostname components to lowercase.
        6.  Removes any URL fragment (the part after a `#` symbol).
        7.  Returns an error if normalization results in an empty or scheme-only URL (e.g., `http://`).
    *   **Returns:** The normalized URL string and an error (of type `*models.URLValidationError`) if any step fails.
    *   **Usage:**
        ```go
        import "monsterinc/internal/urlhandler"
        // ...
        normalized, err := urlhandler.NormalizeURL(" EXAmple.com/path?query#fragment ")
        if err != nil {
            // Handle error (err will be *models.URLValidationError)
        }
        // normalized will be "http://example.com/path?query"
        ```

2.  **`ValidateURL(rawURL string) error`**
    *   **Purpose:** Validates a single URL string by attempting to normalize it.
    *   **Operation:** Calls `NormalizeURL`. If `NormalizeURL` returns an error, this function returns that error.
    *   **Returns:** An error (`*models.URLValidationError`) if the URL is invalid, `nil` otherwise.
    *   **Usage:**
        ```go
        err := urlhandler.ValidateURL("ftp://example.com") // Assuming NormalizeURL handles various schemes or this is valid by its rules
        if err != nil {
            // URL is invalid
        }
        ```

3.  **`IsValidURL(rawURL string) bool`**
    *   **Purpose:** A convenience function to quickly check if a URL is valid.
    *   **Operation:** Calls `ValidateURL` and returns `true` if no error occurs, `false` otherwise.
    *   **Returns:** `true` if the URL is valid, `false` otherwise.
    *   **Usage:**
        ```go
        if urlhandler.IsValidURL("example.com") {
            // Proceed
        }
        ```

4.  **`NormalizeURLs(urls []string) (map[string]string, map[string]error)`**
    *   **Purpose:** Normalizes a slice of URL strings.
    *   **Operation:** Iterates through the input slice, calling `NormalizeURL` for each.
    *   **Returns:**
        *   A map where keys are original URLs and values are their normalized forms (for successfully normalized URLs).
        *   A map where keys are original URLs and values are the errors encountered during their normalization.
    *   **Usage:**
        ```go
        urlsToProcess := []string{"url1", " InValid URL ", "url2.com"}
        normalizedMap, errorsMap := urlhandler.NormalizeURLs(urlsToProcess)
        // Process normalizedMap and errorsMap
        ```

5.  **`ValidateURLs(urls []string) map[string]error`**
    *   **Purpose:** Validates a slice of URL strings.
    *   **Operation:** Iterates through the input slice, calling `ValidateURL` for each.
    *   **Returns:** A map where keys are the invalid original URLs and values are the errors encountered.
    *   **Usage:**
        ```go
        invalidUrls := urlhandler.ValidateURLs([]string{"http://valid.com", "not a url"})
        // Check invalidUrls
        ```

### File Operations

1.  **`ReadURLsFromFile(filePath string) ([]string, error)`**
    *   **Purpose:** Reads URLs from a specified file, one URL per line.
    *   **Operation:**
        1.  Checks if the file exists and is not a directory.
        2.  Opens the file. Handles permission errors.
        3.  Checks if the file is empty (0 bytes).
        4.  Reads the file line by line using a scanner.
        5.  For each line:
            *   Trims whitespace.
            *   Skips empty lines.
            *   Attempts to normalize the URL using `NormalizeURL`.
            *   If normalization fails, logs an error and skips the URL.
            *   Otherwise, adds the normalized URL to a result slice.
        6.  Logs summary statistics (lines read, URLs normalized, URLs skipped).
        7.  Returns an error if the file contained lines but no valid URLs were found.
    *   **Error Types:** Can return `ErrFileNotFound`, `ErrFilePermission`, `ErrFileEmpty`, `ErrReadingFile` (wrapped with more context).
    *   **Returns:** A slice of successfully normalized URL strings and an error if file operations or processing failed significantly.
    *   **Usage:**
        ```go
        normalizedUrls, err := urlhandler.ReadURLsFromFile("path/to/urls.txt")
        if err != nil {
            // Handle file reading or processing error
        }
        // Use normalizedUrls
        ```

### Utility Functions

1.  **`GetBaseURL(rawURL string) (string, error)`**
    *   **Purpose:** Extracts the base URL (scheme + host) from a given URL string.
    *   **Operation:** Parses the URL and constructs a string in the format `scheme://host`.
    *   **Returns:** The base URL string and an error if parsing fails.
    *   **Usage:**
        ```go
        base, err := urlhandler.GetBaseURL("https://example.com/some/path?query=1")
        // base will be "https://example.com"
        ```

2.  **`IsDomainOrSubdomain(domain, baseDomain string) bool`**
    *   **Purpose:** Checks if a `domain` is the same as `baseDomain` or is a subdomain of `baseDomain`.
    *   **Operation:** Compares `domain` and `baseDomain` for equality. If not equal, checks if `domain` ends with `"." + baseDomain`. Assumes inputs are already normalized (e.g., lowercase).
    *   **Returns:** `true` if it's a domain or subdomain, `false` otherwise.
    *   **Usage:**
        ```go
        isSub := urlhandler.IsDomainOrSubdomain("sub.example.com", "example.com") // true
        isSame := urlhandler.IsDomainOrSubdomain("example.com", "example.com")   // true
        isNot := urlhandler.IsDomainOrSubdomain("another.com", "example.com") // false
        ```

3.  **`ResolveURL(href string, base *url.URL) (string, error)`**
    *   **Purpose:** Resolves a URL reference (`href`), which may be relative, against a `base` URL.
    *   **Operation:**
        *   If `base` is `nil`, the `href` must be an absolute URL. If not, an error is returned.
        *   If `base` is provided, `base.Parse(href)` is used to resolve the reference.
    *   **Returns:** The absolute URL string after resolution and an error if parsing or resolution fails.
    *   **Usage:**
        ```go
        baseUrl, _ := url.Parse("https://example.com/path/")
        absoluteLink, err := urlhandler.ResolveURL("../another?q=1", baseUrl)
        // absoluteLink might be "https://example.com/another?q=1"

        absoluteFromNilBase, err := urlhandler.ResolveURL("https://absolute.com/resource", nil)
        // absoluteFromNilBase will be "https://absolute.com/resource"
        ```

## Error Handling

The package defines several custom error variables for file operations in `file.go`:
- `ErrFileNotFound`
- `ErrFilePermission`
- `ErrFileEmpty`
- `ErrReadingFile`

For URL validation and normalization errors, `*models.URLValidationError` is used, which includes the problematic URL and a message.

## Logging

Functions like `ReadURLsFromFile` include logging (using the standard `log` package for now) for:
- Start and end of file processing.
- Skipping empty lines or URLs that fail normalization.
- Errors during file scanning.
- Summary statistics.

## Dependencies
- Go standard library: `fmt`, `net/url`, `strings`, `bufio`, `os`, `errors`, `log`.
- Internal: `monsterinc/internal/models` (for `URLValidationError`).

This documentation provides a guide to understanding and using the `urlhandler` package.
It's recommended to handle errors returned by these functions appropriately in the calling code. 