# Package `models`

Package `models` định nghĩa các cấu trúc dữ liệu (structs) cốt lõi được sử dụng trong toàn bộ ứng dụng MonsterInc. Các model này đại diện cho các thực thể như kết quả thăm dò, cấu hình báo cáo, schema dữ liệu Parquet, v.v.

## Các Models Chính

1.  **`ProbeResult` (`probe_result.go`)**
    -   Đây là struct trung tâm lưu trữ kết quả chi tiết của một lần thăm dò (probe) HTTP/HTTPS tới một URL.
    -   Bao gồm các thông tin cơ bản (URL đầu vào, phương thức, timestamp, thời gian phản hồi, lỗi nếu có, URL gốc của mục tiêu).
    -   Thông tin phản hồi HTTP (mã trạng thái, content length, content type, headers, body, title, web server).
    -   Thông tin chuyển hướng (URL cuối cùng).
    -   Thông tin DNS (IPs, CNAMEs, ASN, ASN Organization).
    -   Thông tin phát hiện công nghệ (`[]Technology`).
    -   Thông tin TLS (phiên bản, cipher, nhà cung cấp chứng chỉ, ngày hết hạn chứng chỉ).
    -   Có một hàm helper `HasTechnologies()`.

2.  **`Technology` (`probe_result.go`)**
    -   Một struct con của `ProbeResult`, lưu trữ thông tin về một công nghệ được phát hiện (tên, phiên bản, danh mục).

3.  **`ParquetProbeResult` (`parquet_schema.go`)**
    -   Định nghĩa schema để lưu trữ `ProbeResult` vào file Parquet, sử dụng thư viện `parquet-go/parquet-go`.
    -   Các trường được đánh dấu bằng struct tags của `parquet-go` (ví dụ: `parquet:"column_name,optional"`).
    -   Các trường tùy chọn (optional) thường là kiểu con trỏ (ví dụ: `*string`, `*int64`) để thể hiện giá trị `nil` trong Parquet.
    -   Slice được dùng cho kiểu `LIST`, và map (ví dụ: headers) được marshal thành chuỗi JSON trước khi lưu.

4.  **`ReportPageData` (`report_data.go`)**
    -   Struct chính chứa tất cả dữ liệu cần thiết để render một trang báo cáo HTML.
    -   Bao gồm tiêu đề báo cáo, thời gian tạo, danh sách các `ProbeResultDisplay`, các thông số thống kê (tổng số, thành công, thất bại), cấu hình reporter, danh sách các giá trị duy nhất cho bộ lọc (status codes, content types, technologies, root targets).
    -   Chứa CSS và JS tùy chỉnh đã nhúng (`CustomCSS`, `ReportJs`).
    -   Chứa `ProbeResultsJSON` là một chuỗi JSON của `[]ProbeResultDisplay` để JavaScript phía client sử dụng.

5.  **`ProbeResultDisplay` (`report_data.go`)**
    -   Một phiên bản của `ProbeResult` được điều chỉnh cho việc hiển thị trong báo cáo HTML.
    -   Có thể bỏ bớt hoặc định dạng lại một số trường từ `ProbeResult` (ví dụ: `TLSCertExpiry` và `Timestamp` được định dạng thành chuỗi).
    -   Bao gồm các cờ boolean tiện ích cho template (ví dụ: `IsSuccess`, `HasTechnologies`, `HasTLS`).
    -   Hàm `ToProbeResultDisplay(pr ProbeResult)` được sử dụng để chuyển đổi từ `ProbeResult` sang `ProbeResultDisplay`.

6.  **`ReporterConfigForTemplate` (`report_data.go`)**
    -   Một tập hợp con các cấu hình từ `config.ReporterConfig` liên quan trực tiếp đến việc render template (ví dụ: `ItemsPerPage`).

7.  **`Target` (`target.go`)**
    -   Đại diện cho một mục tiêu đầu vào, bao gồm `OriginalURL` và `NormalizedURL` sau khi qua xử lý.

8.  **`ExtractedAsset` (`asset.go`)**
    -   Lưu trữ thông tin về một asset (ví dụ: link, script, image) được trích xuất từ một trang HTML.
    -   Bao gồm `AbsoluteURL`, `SourceTag` (ví dụ: `a`, `img`), `SourceAttr` (ví dụ: `href`, `src`).

9.  **`URLSource` (`asset.go`)**
    - Struct đơn giản chứa thông tin Tag và Attribute của một URL được trích xuất.

10. **`URLValidationError` (`error.go`)**
    -   Kiểu lỗi tùy chỉnh được sử dụng bởi package `urlhandler` khi có vấn đề trong việc xác thực hoặc chuẩn hóa URL.
    -   Chứa `URL` gốc và `Message` lỗi.

## Các hàm tiện ích khác

-   `report_data.go` cũng chứa các hàm helper như `formatTime()` và `GetDefaultReportPageData()`.

Package `models` không chứa logic nghiệp vụ phức tạp mà chủ yếu tập trung vào việc định nghĩa cấu trúc dữ liệu và các tiện ích nhỏ liên quan đến các cấu trúc đó. 