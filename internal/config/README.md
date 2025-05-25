# Package config

Package `config` quản lý tất cả các cấu hình cho ứng dụng MonsterInc. Nó định nghĩa các struct cho từng module cấu hình, cung cấp các giá trị mặc định, và cho phép load cấu hình từ file JSON.

## Tổng quan

Cấu hình chính được quản lý bởi struct `GlobalConfig`, bao gồm các cấu hình con cho từng module của ứng dụng.

### `GlobalConfig`

Struct `GlobalConfig` là điểm truy cập trung tâm cho tất cả các thiết lập cấu hình. Nó bao gồm:

-   `InputConfig`: Cấu hình cho việc nhập URL ban đầu.
-   `HttpxRunnerConfig`: Cấu hình cho module httpx probing.
-   `CrawlerConfig`: Cấu hình cho module crawling web.
-   `ReporterConfig`: Cấu hình cho module tạo báo cáo HTML.
-   `StorageConfig`: Cấu hình cho việc lưu trữ dữ liệu (ví dụ: Parquet).
-   `NotificationConfig`: Cấu hình cho việc gửi thông báo (ví dụ: Discord).
-   `LogConfig`: Cấu hình cho logging.
-   `NormalizerConfig`: Cấu hình cho việc chuẩn hóa URL (hiện tại chưa có nhiều thiết lập).
-   `Mode`: Chế độ hoạt động của ứng dụng (ví dụ: "onetime", "automated").

### Các Struct Cấu hình Chi tiết

1.  **`InputConfig`**
    -   `InputURLs []string`: Danh sách các URL đầu vào được cung cấp trực tiếp trong config.

2.  **`HttpxRunnerConfig`**
    -   Quản lý các thiết lập cho việc chạy httpx, bao gồm phương thức HTTP, số luồng, timeout, retries, proxy, các cờ trích xuất dữ liệu (title, status code, headers, v.v.).

3.  **`CrawlerConfig`**
    -   `SeedURLs []string`: Các URL gốc để bắt đầu crawl.
    -   `UserAgent string`: User agent sử dụng khi crawl.
    -   `RequestTimeoutSecs int`: Thời gian chờ cho mỗi request.
    -   `MaxConcurrentRequests int`: Số request đồng thời tối đa.
    -   `MaxDepth int`: Độ sâu tối đa khi crawl.
    -   `RespectRobotsTxt bool`: Có tôn trọng file robots.txt hay không.
    -   `IncludeSubdomains bool`: Có bao gồm các subdomain không.
    -   `Scope CrawlerScopeConfig`: Cấu hình phạm vi crawl.
        -   **`CrawlerScopeConfig`**: Bao gồm `AllowedHostnames`, `DisallowedHostnames`, `AllowedPathRegexes`, `DisallowedPathRegexes` để kiểm soát những gì được crawl.
    -   `MaxContentLengthMB int`: Kích thước nội dung tối đa cho phép tải về.

4.  **`ReporterConfig`**
    -   `OutputDir string`: Thư mục lưu trữ báo cáo.
    -   `EmbedAssets bool`: Có nhúng assets (CSS, JS) vào file HTML không.
    -   `DefaultItemsPerPage int`: Số mục hiển thị trên mỗi trang của báo cáo.
    -   `DefaultOutputHTMLPath string`: Đường dẫn file HTML output mặc định.

5.  **`StorageConfig`**
    -   `ParquetBasePath string`: Đường dẫn thư mục gốc để lưu trữ file Parquet.
    -   `CompressionCodec string`: Codec nén sử dụng cho file Parquet (ví dụ: "zstd", "snappy", "gzip").

6.  **`NotificationConfig`**
    -   Cấu hình cho việc gửi thông báo, ví dụ qua Discord webhook, bao gồm URL webhook và các tùy chọn thông báo cho các sự kiện khác nhau (thành công, thất bại, bắt đầu scan).

7.  **`LogConfig`**
    -   Cấu hình cho logger, bao gồm `LogLevel`, `LogFormat`, `LogFile`, và các tùy chọn quản lý file log (kích thước tối đa, số backup, nén log cũ).

8.  **`NormalizerConfig`**
    -   Hiện tại chưa có nhiều thiết lập, có thể mở rộng trong tương lai (ví dụ: `DefaultScheme`).

## Sử dụng

### Load Cấu hình từ File

Sử dụng hàm `LoadGlobalConfig(filePath string)` để load cấu hình từ một file JSON.

```go
gCfg, err := config.LoadGlobalConfig("config.json")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}
// Sử dụng gCfg...
```

### Cấu hình Mặc định

Package `config` cung cấp các hàm `NewDefault...()` cho mỗi struct cấu hình để lấy các giá trị mặc định. Ví dụ:

-   `NewDefaultGlobalConfig() *GlobalConfig`
-   `NewDefaultHTTPXRunnerConfig() HttpxRunnerConfig`
-   `NewDefaultCrawlerConfig() CrawlerConfig`
-   v.v.

Các giá trị mặc định này cũng được sử dụng bởi `LoadGlobalConfig` nếu một file config rỗng được cung cấp hoặc các trường bị thiếu.

## File `config.json`

Một file `config.json` mẫu được cung cấp ở thư mục gốc của dự án, chứa tất cả các tùy chọn cấu hình có thể có. Người dùng nên sao chép và chỉnh sửa file này cho phù hợp với nhu cầu của mình. 