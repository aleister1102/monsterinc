# MonsterInc

MonsterInc là một công cụ dòng lệnh (CLI) được viết bằng Go, dùng để thu thập thông tin từ các trang web, thực hiện probing HTTP/HTTPS, và tạo báo cáo.

## Tính năng chính

-   **Crawling Web**: Thu thập các URL từ các trang web bắt đầu từ một hoặc nhiều URL gốc (seed URLs).
    -   Kiểm soát phạm vi crawl (allowed/disallowed hostnames, subdomains, path regexes).
    -   Tùy chỉnh User Agent, timeout, độ sâu, số luồng.
    -   Có thể tôn trọng hoặc bỏ qua `robots.txt`.
    -   Kiểm tra `Content-Length` trước khi crawl để tránh tải các file lớn.
-   **HTTP/HTTPS Probing**: Gửi request đến các URL đã thu thập để lấy thông tin chi tiết.
    -   Sử dụng thư viện `httpx` của ProjectDiscovery.
    -   Trích xuất đa dạng thông tin: status code, content type, content length, title, web server, headers, IPs, CNAMEs, ASN, thông tin TLS, công nghệ sử dụng.
    -   Tùy chỉnh phương thức HTTP, request URIs, headers, proxy, timeout, retries.
-   **Báo cáo HTML**: Tạo báo cáo HTML tương tác từ kết quả probing.
    -   Hiển thị kết quả dưới dạng bảng, có thể tìm kiếm, lọc, sắp xếp.
    -   Nhúng CSS/JS tùy chỉnh để có giao diện và trải nghiệm người dùng tốt.
    -   Sử dụng Bootstrap (qua CDN) cho styling cơ bản.
-   **Lưu trữ Parquet**: Ghi kết quả probing vào file Parquet để phân tích dữ liệu sau này.
    -   Hỗ trợ các codec nén: ZSTD (mặc định), SNAPPY, GZIP, UNCOMPRESSED.
    -   Lưu file theo cấu trúc thư mục `ParquetBasePath/YYYYMMDD/scan_results_*.parquet`.
-   **Cấu hình Linh hoạt**: Quản lý cấu hình qua file JSON (`config.json`) và các tham số dòng lệnh.

## Cài đặt và Build

### Yêu cầu

-   Go version 1.23.0 hoặc mới hơn.

### Build

1.  Clone repository (nếu có):
    ```bash
    git clone <your-repository-url>
    cd monsterinc
    ```
2.  Build ứng dụng:
    ```bash
    go build -o monsterinc.exe ./cmd/monsterinc
    ```
    (Hoặc `go build -o monsterinc ./cmd/monsterinc` cho Linux/macOS)

## Cách sử dụng

Chạy ứng dụng từ dòng lệnh:

```bash
./monsterinc.exe --mode <onetime|automated> [tùy chọn khác]
```

### Tham số dòng lệnh chính

-   `--mode <onetime|automated>`: (Bắt buộc) Chế độ chạy.
    -   `onetime`: Chạy một lần, tên file báo cáo sẽ có timestamp chi tiết (ví dụ: `reports/2023-10-27-15-30-00.html`).
    -   `automated`: Chạy tự động (ví dụ: theo lịch), tên file báo cáo sẽ theo ngày (ví dụ: `reports/2023-10-27.html`).
-   `-u <path/to/urls.txt>` hoặc `--urlfile <path/to/urls.txt>`: (Tùy chọn) Đường dẫn đến file text chứa danh sách các URL gốc để crawl, mỗi URL một dòng. Nếu không cung cấp, sẽ sử dụng `input_config.input_urls` từ `config.json`.
-   `--globalconfig <path/to/config.json>`: (Tùy chọn) Đường dẫn đến file cấu hình JSON. Mặc định là `config.json` ở cùng thư mục với file thực thi.

### Ví dụ

```bash
# Chạy một lần với danh sách URL từ file targets.txt
./monsterinc.exe --mode onetime -u targets.txt

# Chạy tự động, sử dụng URL từ config.json (nếu có)
./monsterinc.exe --mode automated

# Chạy với file cấu hình tùy chỉnh
./monsterinc.exe --mode onetime --globalconfig custom_config.json -u targets.txt
```

### File Cấu hình (`config.json`)

File `config.json` cho phép tùy chỉnh chi tiết hành vi của các module. Xem `internal/config/README.md` và file `config.json` mẫu để biết thêm chi tiết về các tùy chọn cấu hình.

## Cấu trúc thư mục dự án

```
monsterinc/
├── cmd/monsterinc/         # Entrypoint của ứng dụng (main.go)
│   └── README.md
├── internal/                 # Code logic nội bộ của ứng dụng
│   ├── config/             # Quản lý cấu hình (config.go, README.md)
│   ├── core/               # Logic nghiệp vụ cốt lõi (target_manager.go, README.md)
│   ├── crawler/            # Module crawling web (crawler.go, scope.go, asset.go, README.md)
│   ├── datastore/          # Module lưu trữ dữ liệu (parquet_writer.go, README.md)
│   ├── differ/             # (Chưa sử dụng) Module so sánh sự khác biệt
│   ├── httpxrunner/        # Wrapper cho thư viện httpx (runner.go, result.go, README.md)
│   ├── logger/             # Module logging (logger.go, README.md)
│   ├── models/             # Định nghĩa các struct dữ liệu (probe_result.go, parquet_schema.go, report_data.go, etc., README.md)
│   ├── notifier/           # (Chưa sử dụng) Module gửi thông báo
│   ├── reporter/           # Module tạo báo cáo HTML
│   │   ├── assets/         # CSS, JS cho báo cáo HTML
│   │   ├── templates/      # Template HTML (report.html.tmpl)
│   │   └── html_reporter.go, README.md
│   ├── urlhandler/         # Xử lý và chuẩn hóa URL (urlhandler.go, file.go, README.md)
│   └── utils/              # (Chưa sử dụng) Các tiện ích chung
├── reports/                  # Thư mục mặc định chứa các báo cáo HTML đã tạo (được .gitignore)
├── database/                 # Thư mục mặc định chứa các file Parquet (theo config.json, được .gitignore)
├── tasks/                    # Chứa các file PRD và danh sách task (ví dụ: *.md)
├── tests/                    # Chứa các file test (ví dụ: _test.go, được .gitignore)
├── .gitignore                # Các file và thư mục bị bỏ qua bởi Git
├── config.json               # File cấu hình mẫu
├── go.mod                    # Khai báo module Go và các dependency
├── go.sum                    # Checksum của các dependency
├── monsterinc.exe            # (Sau khi build) File thực thi cho Windows
├── monsterinc                # (Sau khi build) File thực thi cho Linux/macOS
├── PLAN.md                   # Kế hoạch phát triển (nếu có)
└── README.md                 # README này
```

## Các module chính và chức năng

-   **`config`**: Đọc và quản lý cấu hình từ file JSON.
-   **`urlhandler`**: Chuẩn hóa URL, đọc URL từ file.
-   **`crawler`**: Thu thập URL từ các trang web.
-   **`httpxrunner`**: Thực hiện thăm dò HTTP/HTTPS đến các URL bằng `httpx`.
-   **`models`**: Định nghĩa các struct dữ liệu chung.
-   **`datastore`**: Lưu trữ kết quả vào file Parquet.
-   **`reporter`**: Tạo báo cáo HTML từ kết quả.
-   **`logger`**: Cung cấp interface và triển khai cơ bản cho logging.
-   **`core`**: (Đang phát triển) Chứa logic điều phối chính.

## Đóng góp

(Thông tin về cách đóng góp vào dự án - nếu có) 