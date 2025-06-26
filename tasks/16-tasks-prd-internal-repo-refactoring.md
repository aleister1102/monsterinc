## Nguyên tắc Refactor
Tất cả các mã được viết trong quá trình refactor này phải tuân thủ các nguyên tắc sau để đảm bảo tính rõ ràng, đơn giản và dễ bảo trì.

- **Tên có ý nghĩa**: Tên biến, hàm, struct phải rõ ràng, súc tích. Tránh các tên viết tắt hoặc tên chung chung.
- **Hàm ngắn gọn**: Mỗi hàm chỉ nên thực hiện một nhiệm vụ duy nhất. Giữ số lượng tham số ở mức tối thiểu.
- **Bình luận (Comment)**: Chỉ bình luận để giải thích "tại sao" (why) một quyết định thiết kế được đưa ra, không phải "cái gì" (what) code đang làm. Viết Godoc cho các API public.
- **Định dạng**: Luôn sử dụng `gofmt` hoặc `goimports` để định dạng code.
- **Xử lý lỗi**: Xử lý lỗi một cách rõ ràng, không bỏ qua. Cung cấp ngữ cảnh cho lỗi để dễ gỡ lỗi.
- **Cấu trúc**: Tổ chức code vào các package có trách nhiệm rõ ràng, tránh lặp lại code (DRY) và tránh import vòng.
- **Ưu tiên sự đơn giản**: Tránh sử dụng các Design Pattern phức tạp nếu có giải pháp đơn giản và dễ hiểu hơn.

## Ghi chú về chiến lược Refactor

Theo yêu cầu mới nhất, chúng ta sẽ theo đuổi chiến lược tách biệt tối đa. Sẽ **không có bất kỳ thư viện dùng chung nào**.

- **Mỗi thư viện phải tự túc**: Nếu một thư viện cần một cấu trúc dữ liệu (model) nào đó, file định nghĩa model đó sẽ được di chuyển vào chính thư viện đó. Ví dụ: `ProbeResult` sẽ thuộc về thư viện `httpx`.
- **Không có phụ thuộc chéo giữa các thư viện mới**: Các thư viện được tách ra sẽ không `import` lẫn nhau.
- **Giữ lại code không thể tách**: Các thành phần quá đặc thù hoặc có nhiều phụ thuộc chéo sẽ được giữ lại trong `internal/` của project chính. Dựa trên phân tích này, các package sau sẽ **KHÔNG** được tách ra: `datastore`, `secretscanner`, `reporter`, `notifier`.

Cách tiếp cận này đảm bảo các thư viện mới hoàn toàn độc lập và có thể được sử dụng trong các project khác mà không cần kéo theo bất kỳ phụ thuộc nào từ hệ sinh thái `monsterinc`.

## Relevant Files

- `libs/` - Thư mục mới để chứa tất cả các thư viện sẽ được tách ra.
- `go.mod` - File go.mod của project chính, sẽ cần cập nhật nhiều với các `replace` directive.
- `internal/` - Nhiều file và thư mục trong này sẽ được di chuyển hoặc xóa sau khi hoàn tất.
- `cmd/monsterinc/main.go` - Entrypoint chính của ứng dụng.

### Notes

- Mỗi thư viện mới sẽ có file `go.mod` riêng và không phụ thuộc vào các thư viện mới khác.
- Chúng ta sẽ sử dụng `replace` trong `go.mod` chính để trỏ đến các thư viện local trong quá trình phát triển.

## Tasks

- [x] 1.0: Chuẩn bị cấu trúc dự án
  - [x] 1.1: Tạo thư mục `libs` ở thư mục gốc của project.
  - [x] 1.2: Cập nhật file `.gitignore` để thêm `libs/**/vendor/` và `libs/**/*.test.out`.

- [x] 2.0: Tách thư viện `logger`
  - [x] 2.1: Tạo thư mục `libs/logger`.
  - [x] 2.2: Di chuyển các file:
    - Toàn bộ thư mục `internal/logger/`
    - `internal/config/log_config.go`
  - [x] 2.3: Các **struct** chính sẽ được chuyển: `Logger`, `LoggerBuilder`, `LoggerConfig`, `LogFormat`, `ConfigConverter`, `LogLevelParser`, `LogFormatParser`, `WriterFactory`, `WriterStrategy` và các implementation của nó.
  - [x] 2.4: `cd libs/logger` và chạy `go mod init github.com/monsterinc/logger`.
  - [x] 2.5: Refactor code để loại bỏ các phụ thuộc vào `internal/config` khác và đảm bảo tính độc lập.
  - [x] 2.6: Chạy `go mod tidy` và kiểm tra build/test.

- [x] 3.0: Tách thư viện `httpclient`
  - [x] 3.1: Tạo thư mục `libs/httpclient`.
  - [x] 3.2: Di chuyển các file từ `internal/common/`:
    - `http_client.go`, `http_client_config.go`, `http_client_builder.go`, `http_client_factory.go`, `http_request_response.go`, `fetcher.go`, `retry.go`.
  - [x] 3.3: Các **struct** chính sẽ được chuyển: `HTTPClient`, `HTTPClientConfig`, `HTTPRequest`, `HTTPResponse`, `HTTPClientBuilder`, `HTTPClientFactory`, `Fetcher`, `FetchFileContentInput`, `FetchFileContentResult`, `RetryHandler`, `RetryHandlerConfig`.
  - [x] 3.4: `cd libs/httpclient` và chạy `go mod init github.com/monsterinc/httpclient`.
  - [x] 3.5: Refactor code: Thư viện này phải tự định nghĩa các kiểu lỗi (error types) của riêng mình để không phụ thuộc vào `internal/common/errors.go`.
  - [x] 3.6: Chạy `go mod tidy` và kiểm tra build/test.

- [x] 4.0: Tách thư viện `limiter`
  - [x] 4.1: Tạo thư mục `libs/limiter`.
  - [x] 4.2: Di chuyển các file từ `internal/common/`: `resource_limiter.go`, `resource_limiter_config.go`, `resource_usage.go`, `resource_stats_monitor.go`.
  - [x] 4.3: Các **struct** chính sẽ được chuyển: `ResourceLimiter`, `ResourceLimiterConfig`, `ResourceUsage`, `ResourceStatsMonitor`, `ResourceStatsMonitorConfig`.
  - [x] 4.4: `cd libs/limiter` và chạy `go mod init github.com/monsterinc/limiter`.
  - [x] 4.5: Chạy `go mod tidy` và kiểm tra build/test.

- [x] 5.0: Tách thư viện `progress`
  - [x] 5.1: Tạo thư mục `libs/progress`.
  - [x] 5.2: Di chuyển các file từ `internal/common/`: `progress_display_core.go`, `progress_display_loop.go`, `progress_display_updater.go`, `progress_info.go`, `progress_types.go`.
  - [x] 5.3: Các **struct/type** chính sẽ được chuyển: `ProgressDisplayConfig`, `ProgressDisplayManager`, `ProgressInfo`, `BatchProgressInfo`, `MonitorProgressInfo`, `ProgressType`, `ProgressStatus`.
  - [x] 5.4: `cd libs/progress` và chạy `go mod init github.com/monsterinc/progress`.
  - [x] 5.5: Chạy `go mod tidy` và kiểm tra build/test.

- [x] 6.0: Tách thư viện `httpx`
  - [x] 6.1: Tạo thư mục `libs/httpx`.
  - [x] 6.2: Di chuyển các file:
    - Toàn bộ thư mục `internal/httpxrunner/`
    - `internal/models/probe_result.go`
  - [x] 6.3: Các **struct** chính sẽ được chuyển: `Runner`, `RunnerBuilder`, `Config` (từ httpxrunner), `HTTPXOptionsConfigurator`, `ProbeResultMapper`, `ResultCollector`, `ProbeResult`, `Technology`.
  - [x] 6.4: `cd libs/httpx` và chạy `go mod init github.com/monsterinc/httpx`.
  - [x] 6.5: Refactor code để loại bỏ các phụ thuộc (ví dụ: `common.ValidationError`).
  - [x] 6.6: Chạy `go mod tidy` và kiểm tra build/test.

- [x] 7.0: Tích hợp và dọn dẹp
  - [x] 7.1: Cập nhật `go.mod` của project chính, thêm các `replace` directive cho tất cả thư viện mới.
  - [x] 7.2: Refactor toàn bộ project `monsterinc`, thay thế các đường dẫn import cũ (`internal/...`) bằng đường dẫn import mới (`github.com/monsterinc/...`).
  - [x] 7.3: Chạy `go mod tidy` ở thư mục gốc.
  - [x] 7.4: Biên dịch và chạy toàn bộ test của project (`go build ./... && go test ./...`).
  - [x] 7.5: Xóa các thư mục và file đã được di chuyển hoàn toàn ra khỏi `internal`.
  - [x] 7.6: Viết tài liệu `README.md` cho các thư viện mới. 