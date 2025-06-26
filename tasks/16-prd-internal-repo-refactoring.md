# PRD: Tách các package trong `internal` thành các thư viện độc lập

## 1. Giới thiệu/Tổng quan

Tài liệu này mô tả yêu cầu kỹ thuật cho việc tái cấu trúc (refactor) các package bên trong thư mục `internal` của dự án `monsterinc`. Mục tiêu là tách các thành phần có thể tái sử dụng thành các thư viện Go (Go modules) độc lập. Việc này sẽ giúp cải thiện cấu trúc dự án, tăng khả năng tái sử dụng code cho các dự án khác trong tương lai, và cho phép các thư viện này được phát triển và kiểm thử một cách độc lập.

## 2. Mục tiêu

*   Tách các package có chức năng chung, ít phụ thuộc vào logic lõi của `monsterinc` thành các thư viện độc lập.
*   Mỗi thư viện được tách ra phải có file `go.mod` riêng và có thể được `go get` và sử dụng trong một dự án khác.
*   Dự án `monsterinc` chính sẽ import các thư viện này thông qua `go.mod` của nó.
*   Giảm thiểu sự耦合 (coupling) giữa các thành phần trong codebase.

## 3. User Stories

*   **Là một nhà phát triển,** tôi muốn sử dụng lại logic ghi/đọc file Parquet từ `datastore` trong một project mới mà không cần phải copy-paste code hoặc import toàn bộ project `monsterinc`.
*   **Là một nhà phát triển,** tôi muốn sử dụng hệ thống `logger` đã được cấu hình sẵn trong nhiều project khác nhau để đảm bảo sự đồng nhất trong logging.
*   **Là một nhà phát triển,** tôi muốn các thư viện chung được quản lý phiên bản (versioning) một cách độc lập, cho phép tôi cập nhật chúng mà không ảnh hưởng đến các phần khác của ứng dụng chính.

## 4. Yêu cầu chức năng

### 4.1. Phân tích và đề xuất các thư viện cần tách

Chúng ta sẽ áp dụng nguyên tắc chỉ tách các package khi chúng cung cấp một chức năng hoàn chỉnh, có thể tái sử dụng và đủ lớn để justifications cho việc tồn tại như một thư viện độc lập. Sẽ không có thư viện `models` dùng chung.

1.  **`config`**:
    *   **Nguồn**: `internal/config/loader.go`, `internal/config/validator.go` và các file cấu hình chung không phụ thuộc vào logic nghiệp vụ.
    *   **Mô tả**: Logic để tải và xác thực cấu hình.

2.  **`logger`**:
    *   **Nguồn**: `internal/logger` và `internal/config/log_config.go`.
    *   **Mô tả**: Thư viện logging.

3.  **`httpclient`**:
    *   **Nguồn**: Các file liên quan trong `internal/common` như `http_client.go`, `fetcher.go` và `retry.go`.
    *   **Mô tả**: Cung cấp HTTP client, tích hợp sẵn logic retry.

4.  **`limiter`**:
    *   **Nguồn**: Các file liên quan đến resource limiter trong `internal/common`.
    *   **Mô tả**: Thư viện để giới hạn tài nguyên và rate limit.

5.  **`progress`**:
    *   **Nguồn**: Các file liên quan đến progress display trong `internal/common`.
    *   **Mô tả**: Cung cấp chức năng hiển thị thanh tiến trình cho các tác vụ dòng lệnh.

6.  **`datastore`**:
    *   **Nguồn**: `internal/datastore` và các model liên quan như `parquet_schema.go`, `file_history.go` từ `internal/models`.
    *   **Mô tả**: Logic để tương tác với file Parquet. Các model cần thiết sẽ được định nghĩa bên trong thư viện này.

7.  **`notifier`**:
    *   **Nguồn**: `internal/notifier` và `notification_models.go` từ `internal/models`.
    *   **Mô tả**: Gửi thông báo. Các model cần thiết sẽ được định nghĩa bên trong thư viện này.

8.  **`reporter`**:
    *   **Nguồn**: `internal/reporter` và `report_data.go` từ `internal/models`.
    *   **Mô tả**: Tạo báo cáo HTML. Nếu cần dữ liệu từ `datastore`, nó sẽ import thư viện `datastore` và sử dụng model của `datastore`.

9.  **`httpx`**:
    *   **Nguồn**: `internal/httpxrunner` và `probe_result.go` từ `internal/models`.
    *   **Mô tả**: Wrapper cho công cụ `httpx`. Model `ProbeResult` sẽ nằm trong thư viện này.

10. **`secretscanner`**:
    *   **Nguồn**: `internal/secretscanner` và `secret_finding.go` từ `internal/models`.
    *   **Mô tả**: Quét secrets. Model `SecretFinding` sẽ nằm trong thư viện này.

### 4.2. Các thành phần không tách

*   **Các tiện ích nhỏ**: Các file trong `internal/common` như `context_utils.go`, `errors.go`, `time_utils.go` sẽ được giữ lại trong project chính.
*   **Các model đặc thù của project**: Các model gắn liền với logic nghiệp vụ cốt lõi của `monsterinc` như `Asset`, `Target`, `ContentDiff`, `ScanSummary` sẽ được giữ lại trong `internal/models` của project chính.

### 4.3. Nguyên tắc phân tách Models

*   **Không có thư viện model chung**: Mỗi thư viện tự quản lý các model mà nó định nghĩa.
*   **Phụ thuộc tường minh**: Nếu thư viện `A` cần model từ thư viện `B`, `A` phải import `B`.
*   **Model của project được giữ lại**: Các model đặc thù cho logic của `monsterinc` vẫn nằm trong `internal/models`.

### 4.4. Quy trình thực hiện cho mỗi thư viện

1.  Tạo một thư mục mới cho thư viện (ví dụ: `libs/logger`).
2.  Di chuyển code từ các package tương ứng trong `internal` vào thư mục mới.
3.  Trong thư mục của thư viện mới, chạy `go mod init <module_path>` (ví dụ: `go mod init github.com/monsterinc/logger`).
4.  Refactor code để loại bỏ các phụ thuộc không cần thiết và cập nhật đường dẫn import để trỏ đến các thư viện phụ thuộc khác nếu có (ví dụ: `reporter` import `datastore`).
5.  Cập nhật file `go.mod` của project `monsterinc` để sử dụng thư viện mới qua `replace`.
    ```go
    replace github.com/monsterinc/logger => ../libs/logger
    ```
6.  Biên dịch và chạy test để đảm bảo mọi thứ hoạt động chính xác.

## 5. Non-Goals (Ngoài phạm vi)

*   **Không tách các package có logic nghiệp vụ phức tạp và gắn chặt với `monsterinc`:** Các package như `crawler`, `scanner`, `monitor`, `scheduler`, `differ`, `extractor`, `urlhandler` sẽ **không** được tách trong lần refactor này do chúng có nhiều phụ thuộc chéo và chứa logic lõi của ứng dụng. Việc cố gắng tách chúng ra ở giai đoạn này có thể gây ra nhiều rủi ro và tốn kém thời gian.
*   **Không public các thư viện lên Github:** Ở giai đoạn này, các thư viện sẽ được quản lý như các subproject trong cùng một monorepo. Việc public chúng sẽ được xem xét sau.
*   Không thay đổi logic hoạt động của các chức năng hiện có. Đây chỉ là một cuộc tái cấu trúc về mặt tổ chức code.

## 6. Tiêu chí thành công

*   Các thư viện được đề xuất (`logger`, `datastore`, v.v.) được tạo thành công dưới dạng các Go module độc lập.
*   Mỗi thư viện có thể được build (`go build ./...`) và test (`go test ./...`) một cách độc lập (với các thư viện phụ thuộc của nó).
*   Project `monsterinc` chính có thể import, build và hoạt động bình thường sau khi sử dụng các thư viện đã tách.
*   Codebase của `monsterinc` trở nên gọn gàng hơn, các thành phần được phân định rõ ràng hơn.

## 7. Câu hỏi còn mở

*   Cần xác định một convention đặt tên và cấu trúc thư mục cuối cùng cho các thư viện mới (ví dụ: `libs/` hay một thư mục khác cùng cấp với `monsterinc`). 