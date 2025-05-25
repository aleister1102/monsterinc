# Package `urlhandler`

Package `urlhandler` cung cấp các tiện ích để xử lý, chuẩn hóa, và xác thực URL, cũng như đọc URL từ file.

## Chức năng chính

### Xử lý và Chuẩn hóa URL (`urlhandler.go`)

1.  **`NormalizeURL(rawURL string) (string, error)`**
    -   Đây là hàm cốt lõi để chuẩn hóa một URL đầu vào.
    -   **Các bước chuẩn hóa:**
        1.  Loại bỏ khoảng trắng thừa ở đầu và cuối (`strings.TrimSpace`).
        2.  Nếu URL không có scheme (ví dụ: `example.com/path`), nó sẽ thử thêm `http://` vào đầu.
        3.  Parse URL bằng `net/url.Parse()`.
        4.  Chuyển đổi scheme (ví dụ: `HTTP` -> `http`) và host (ví dụ: `Example.COM` -> `example.com`) sang chữ thường.
        5.  Loại bỏ phần fragment của URL (phần sau dấu `#`).
    -   **Xử lý lỗi:** Trả về lỗi kiểu `*models.URLValidationError` nếu URL không hợp lệ (ví dụ: rỗng, parsing thất bại, không có host).

2.  **`ValidateURL(rawURL string) error`**
    -   Sử dụng `NormalizeURL` để kiểm tra xem một URL có hợp lệ hay không. Trả về `nil` nếu hợp lệ, ngược lại trả về lỗi từ `NormalizeURL`.

3.  **`IsValidURL(rawURL string) bool`**
    -   Một hàm tiện ích trả về `true` nếu `ValidateURL` không trả về lỗi, `false` nếu ngược lại.

4.  **`NormalizeURLs(urls []string) (map[string]string, map[string]error)`**
    -   Nhận một slice các URL, chuẩn hóa từng URL bằng `NormalizeURL`.
    -   Trả về hai map: một map các URL gốc tới các URL đã chuẩn hóa thành công, và một map các URLs gốc tới lỗi nếu có.

5.  **`ValidateURLs(urls []string) map[string]error`**
    -   Tương tự `NormalizeURLs` nhưng chỉ trả về map các URLs lỗi.

6.  **`GetBaseURL(rawURL string) (string, error)`**
    -   Parse URL và trả về phần base của nó (ví dụ: `http://example.com`).

7.  **`IsDomainOrSubdomain(domain, baseDomain string) bool`**
    -   Kiểm tra xem `domain` có phải là `baseDomain` hoặc là một subdomain của `baseDomain` hay không (ví dụ: `blog.example.com` là subdomain của `example.com`).

8.  **`ResolveURL(href string, base *url.URL) (string, error)`**
    -   Giải quyết một chuỗi `href` (có thể là URL tương đối hoặc tuyệt đối) dựa trên một `base *url.URL`.
    -   Nếu `base` là `nil`, `href` phải là một URL tuyệt đối.
    -   Sử dụng `base.Parse(href)` để thực hiện việc giải quyết.

### Đọc URL từ File (`file.go`)

1.  **`ReadURLsFromFile(filePath string) ([]string, error)`**
    -   Đọc một file văn bản tại `filePath`, mỗi dòng được coi là một URL.
    -   **Xử lý file:**
        -   Kiểm tra file có tồn tại, có phải là file (không phải thư mục), và có quyền đọc không.
        -   Kiểm tra file có rỗng (0 byte) không.
    -   **Xử lý từng dòng:**
        -   Bỏ qua các dòng trống.
        -   Gọi `NormalizeURL` cho mỗi dòng không trống.
        -   Nếu `NormalizeURL` trả về lỗi, ghi log lỗi và bỏ qua URL đó.
        -   Các URLs được chuẩn hóa thành công sẽ được thêm vào một slice.
    -   **Logging**: Ghi log về quá trình xử lý file, bao gồm số dòng đã đọc, số URLs chuẩn hóa thành công, và số URLs bị bỏ qua do lỗi.
    -   **Trả về**: Một slice các chuỗi URLs đã được chuẩn hóa và hợp lệ, hoặc một lỗi nếu có vấn đề nghiêm trọng khi đọc file hoặc không tìm thấy URLs hợp lệ nào.
    -   **Các lỗi tùy chỉnh**: `ErrFileNotFound`, `ErrFilePermission`, `ErrFileEmpty`, `ErrReadingFile`.

## Kiểu lỗi

-   `models.URLValidationError`: Được trả về bởi `NormalizeURL` khi có lỗi trong quá trình chuẩn hóa hoặc xác thực URLs. Nó chứa URLs gốc và thông báo lỗi.

## Cách sử dụng ví dụ

```go
// Chuẩn hóa một URL
normalized, err := urlhandler.NormalizeURL("Http://Example.Com/Path?Query#Frag")
if err != nil {
    log.Printf("Lỗi chuẩn hóa: %v", err)
} else {
    fmt.Printf("URL đã chuẩn hóa: %s", normalized) // Output: http://example.com/path?Query
}

// Đọc URLs từ file
urls, err := urlhandler.ReadURLsFromFile("targets.txt")
if err != nil {
    log.Fatalf("Không thể đọc URLs từ file: %v", err)
}
for _, u := range urls {
    fmt.Println(u)
}
```

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