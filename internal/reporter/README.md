# Package `reporter`

Package `reporter` chịu trách nhiệm tạo các báo cáo từ kết quả thăm dò (probe results). Hiện tại, nó tập trung vào việc tạo báo cáo HTML tương tác.

## `HtmlReporter`

`HtmlReporter` là thành phần chính để tạo báo cáo HTML từ dữ liệu `models.ProbeResult`.

### Chức năng chính

1.  **Khởi tạo**: `NewHtmlReporter(cfg *config.ReporterConfig, appLogger *log.Logger)`
    -   Nhận vào `config.ReporterConfig` để lấy các thiết lập như đường dẫn template tùy chỉnh, tiêu đề báo cáo, số mục trên mỗi trang, v.v.
    -   Load và parse template HTML. Mặc định, nó sử dụng template được nhúng từ `internal/reporter/templates/report.html.tmpl`. Nếu `cfg.TemplatePath` được chỉ định, nó sẽ load template từ đường dẫn đó.
    -   Đăng ký các `templateFunctions` để sử dụng trong template.

2.  **Tạo Báo cáo**: `GenerateReport(probeResults []models.ProbeResult, outputPath string)`
    -   Đây là phương thức chính để tạo báo cáo.
    -   Nếu `probeResults` rỗng và `GenerateEmptyReport` trong config là `false`, báo cáo sẽ không được tạo.
    -   Gọi `prepareReportData` để chuyển đổi và chuẩn bị dữ liệu hiển thị.
    -   Gọi `embedCustomAssets` để nhúng nội dung của các file CSS và JavaScript tùy chỉnh.
    -   Gọi `executeAndWriteReport` để render template HTML với dữ liệu đã chuẩn bị và ghi ra `outputPath`.

3.  **Chuẩn bị Dữ liệu Báo cáo**: `prepareReportData(probeResults []models.ProbeResult) (models.ReportPageData, error)`
    -   Chuyển đổi `[]models.ProbeResult` thành `[]models.ProbeResultDisplay` (struct trong `internal/models/report_data.go` được tối ưu cho việc hiển thị trong template).
    -   Tính toán các thông tin tổng hợp như tổng số kết quả, số kết quả thành công/thất bại.
    -   Trích xuất các giá trị duy nhất từ kết quả để điền vào các bộ lọc (ví dụ: status codes, content types, technologies, root target URLs).
    -   Serialize `[]models.ProbeResultDisplay` thành một chuỗi JSON và gán vào `ReportPageData.ProbeResultsJSON`. Chuỗi JSON này sẽ được JavaScript phía client (`report.js`) sử dụng để render bảng và thực hiện các tương tác (tìm kiếm, lọc, sắp xếp, phân trang) mà không cần tải lại trang.

4.  **Nhúng Assets**: `embedCustomAssets(pageData *models.ReportPageData)`
    -   Đọc nội dung của các file CSS (`assets/css/styles.css`) và JavaScript (`assets/js/report.js`) được nhúng bằng `//go:embed`.
    -   Gán nội dung CSS và JS này vào các trường tương ứng trong `ReportPageData` (`CustomCSS`, `ReportJs`) để chúng có thể được chèn trực tiếp vào template HTML, tạo ra một file báo cáo HTML hoàn toàn độc lập.

5.  **Thực thi và Ghi Báo cáo**: `executeAndWriteReport(pageData models.ReportPageData, outputPath string)`
    -   Render template HTML đã được parse với `pageData`.
    -   Tạo thư mục output nếu chưa tồn tại.
    -   Ghi nội dung HTML đã render ra file tại `outputPath`.

### Template và Assets

-   **Template HTML**: `internal/reporter/templates/report.html.tmpl` là file template chính, sử dụng cú pháp của `html/template` trong Go.
-   **CSS Tùy chỉnh**: `internal/reporter/assets/css/styles.css` chứa các style tùy chỉnh cho báo cáo.
-   **JavaScript Tùy chỉnh**: `internal/reporter/assets/js/report.js` xử lý các tương tác phía client như tìm kiếm, lọc, sắp xếp, phân trang, và hiển thị chi tiết trong modal. Nó sử dụng dữ liệu JSON được nhúng trong `ReportPageData.ProbeResultsJSON`.
-   Các thư viện bên ngoài như Bootstrap và DataTables (tùy chọn) được load qua CDN trong template HTML.

### Các hàm tiện ích trong Template (`templateFunctions`)

Một số hàm tiện ích được cung cấp để sử dụng trong template, ví dụ:
-   `joinStrings`: Nối một slice các chuỗi.
-   `toLower`: Chuyển chuỗi sang chữ thường.
-   `formatTime`: Định dạng thời gian.
-   `safeHTML`: Đánh dấu một chuỗi là HTML an toàn.

## Cách sử dụng

1.  Tạo `config.ReporterConfig`.
2.  Gọi `NewHtmlReporter()` với config đó.
3.  Thu thập `[]models.ProbeResult` từ các module khác (ví dụ: `httpxrunner`).
4.  Gọi `htmlReporter.GenerateReport(results, "path/to/report.html")`.

## Features

-   **HTML Report Generation**: Creates a single, self-contained (or CDN-linked for common libraries) HTML file.
-   **Interactive UI**: The HTML report includes features such as:
    -   Global search across multiple fields.
    -   Filtering by Status Code, Content Type, and Technologies.
    -   Sorting by various columns (Input URL, Final URL, Status Code, Title, etc.).
    -   Pagination to handle large datasets.
    -   Multi-target navigation via a sidebar if multiple root targets were scanned.
    -   Modal view for detailed information of each probe result, including headers and body snippets.
-   **Customizable**: Report title and items per page can be configured.
-   **Asset Embedding**: Custom CSS and JavaScript are embedded into the HTML report. Common libraries like Bootstrap and jQuery are linked via CDN by default.

## Core Components

-   `html_reporter.go`: Contains the main logic for `HtmlReporter`, including parsing Go HTML templates and populating them with data.
-   `templates/report.html.tmpl`: The Go HTML template file that defines the structure of the report.
-   `assets/`: Directory containing static assets:
    -   `css/styles.css`: Custom CSS for styling the HTML report.
    -   `js/report.js`: Custom JavaScript (using jQuery) for interactivity (search, sort, filter, pagination, modal views, multi-target navigation).

## Configuration

The reporter's behavior is configured via `ReporterConfig` (defined in `internal/config/config.go`), which includes options such as:

-   `OutputDir`: Directory where reports will be saved (though `DefaultOutputHTMLPath` specifies the full path for the main report).
-   `EmbedAssets`: Whether to embed custom assets (currently always true for custom CSS/JS).
-   `TemplatePath`: Custom path to an HTML template file (if not using the embedded one).
-   `GenerateEmptyReport`: Whether to generate a report if there are no results.
-   `ReportTitle`: Custom title for the HTML report.
-   `DefaultItemsPerPage`: Default number of items to show per page in the report table.
-   `EnableDataTables`: Controls whether DataTables.js CDN links are included (though current interactivity is custom JS).
-   `DefaultOutputHTMLPath`: The default full path (including filename) where the HTML report will be saved.

## Usage

An `HtmlReporter` instance is created using `reporter.NewHtmlReporter()`, passing the `ReporterConfig` and a logger.
The report is generated by calling the `GenerateReport()` method with the probe results and the desired output file path.

Example integration in `cmd/monsterinc/main.go`:

```go
// Assuming gCfg is your loaded GlobalConfig and probeResults is []models.ProbeResult

reporterCfg := &gCfg.ReporterConfig
htmlReporter, reporterErr := reporter.NewHtmlReporter(reporterCfg, log.Default())
if reporterErr != nil {
    log.Printf("[ERROR] Main: Failed to initialize HTML reporter: %v", reporterErr)
} else {
    outputFile := reporterCfg.DefaultOutputHTMLPath
    if outputFile == "" {
        outputFile = "monsterinc_report.html" 
        log.Printf("[WARN] Main: ReporterConfig.DefaultOutputHTMLPath is not set. Using default: %s", outputFile)
    }
    err := htmlReporter.GenerateReport(probeResults, outputFile)
    if err != nil {
        log.Printf("[ERROR] Main: Failed to generate HTML report: %v", err)
    } else {
        log.Printf("[INFO] Main: HTML report generated successfully: %s", outputFile)
    }
}
```

## Future Considerations

-   Allow full offline asset embedding (Bootstrap, jQuery) via configuration.
-   More advanced theming options (e.g., dark mode improvements).
-   Unit tests for JavaScript functionality. 