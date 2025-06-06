# MonsterInc

MonsterInc là một công cụ CLI (Command-Line Interface) được viết bằng Go, chuyên dụng cho việc thu thập thông tin từ các website, thực hiện HTTP/HTTPS probing, giám sát thay đổi nội dung, phát hiện secrets và tạo báo cáo chi tiết.

## Tính năng chính

### 🕷️ Web Crawling
- Thu thập URLs từ websites bắt đầu từ một hoặc nhiều seed URLs
- Kiểm soát phạm vi crawl (hostnames được phép/không được phép, subdomains, path regexes)
- Tùy chỉnh User-Agent, timeout, độ sâu, số luồng
- Có thể tuân thủ hoặc bỏ qua `robots.txt`
- Kiểm tra `Content-Length` trước khi crawl để tránh tải file lớn

### 🔍 HTTP/HTTPS Probing
- Sử dụng thư viện `httpx` của ProjectDiscovery
- Trích xuất thông tin đa dạng: status code, content type, content length, title, web server, headers, IPs, CNAMEs, ASN, thông tin TLS, công nghệ sử dụng
- Tùy chỉnh HTTP method, request URIs, headers, proxy, timeout, retries

### 📊 HTML Reporting
- Tạo báo cáo HTML tương tác từ kết quả probing
- Hiển thị kết quả dạng bảng với khả năng tìm kiếm, lọc và sắp xếp
- Nhúng CSS/JS tùy chỉnh cho giao diện người dùng tốt
- Sử dụng Bootstrap và DataTables cho styling và tương tác

### 💾 Parquet Storage
- Ghi kết quả probing vào file Parquet để phân tích dữ liệu sau này
- Hỗ trợ các codec nén: ZSTD (mặc định), SNAPPY, GZIP, UNCOMPRESSED
- Lưu file theo cấu trúc thư mục được tổ chức theo ngày và target

### ⚙️ Flexible Configuration
- Quản lý cấu hình qua file YAML (`config.yaml` ưu tiên) hoặc JSON (`config.json`)
- Hỗ trợ tham số command-line
- Hot-reload configuration với file watching

### 🔄 Periodic Scanning (Automated Mode)
- Cho phép lập lịch quét định kỳ với khoảng thời gian có thể cấu hình
- Tải lại danh sách target ở đầu mỗi chu kỳ quét
- Duy trì lịch sử quét trong cơ sở dữ liệu SQLite
- Gửi thông báo (ví dụ: qua Discord) khi bắt đầu quét, thành công và thất bại
- Bao gồm logic retry cho các lần quét thất bại

### 📁 File Monitoring
- Giám sát thay đổi file JS/HTML trong thời gian thực
- Phát hiện thay đổi nội dung và tạo báo cáo diff
- Hỗ trợ thông báo tổng hợp
- Sử dụng ETag và Last-Modified headers cho conditional requests

### 🔐 Secret Detection
- Tích hợp TruffleHog cho phát hiện secrets
- Hỗ trợ custom regex patterns từ Mantra project
- Thông báo tự động cho secrets độ nghiêm trọng cao
- Lưu trữ findings trong Parquet format

### 🔗 Path Extraction
- Trích xuất paths/URLs từ nội dung JS/HTML
- Sử dụng thư viện jsluice cho phân tích JS
- Hỗ trợ custom regex patterns
- Phát hiện API endpoints và sensitive paths

### 📈 Diff Analysis
- So sánh kết quả quét hiện tại với dữ liệu lịch sử
- Phân loại URLs: New, Existing, Old
- Tạo báo cáo diff chi tiết cho thay đổi nội dung
- Hỗ trợ beautification cho HTML/JS trong diff reports

## Cài đặt

### Yêu cầu hệ thống
- Go version 1.23.1 hoặc mới hơn

### Cài đặt từ Source

1. Clone repository:
```bash
git clone https://github.com/aleister1102/monsterinc.git
cd monsterinc
```

2. Build ứng dụng:
```bash
# Windows
go build -o monsterinc.exe ./cmd/monsterinc

# Linux/macOS
go build -o monsterinc ./cmd/monsterinc
```

### Cài đặt từ GitHub Releases

1. Download appropriate binary from [GitHub Releases](https://github.com/aleister1102/monsterinc/releases)
2. Extract and place in system PATH

### Cài đặt via Go install

```bash
go install github.com/aleister1102/monsterinc/cmd/monsterinc@latest
```

## Sử dụng

### Cú pháp cơ bản

```bash
./monsterinc --mode <onetime|automated> [options]
```

### Tham số Command-Line chính

#### Tham số bắt buộc
- `--mode <onetime|automated>`: (Bắt buộc) Chế độ thực thi
  - `onetime`: Chạy một lần và thoát
  - `automated`: Chạy liên tục theo lịch trình

#### Tham số tùy chọn
- `--scan-targets, -st <path>`: Đường dẫn đến file chứa danh sách seed URLs
- `--monitor-targets, -mt <path>`: File chứa URLs để giám sát (chỉ cho automated mode)
- `--globalconfig, -gc <path>`: Đường dẫn đến file cấu hình

### Ví dụ sử dụng

```bash
# Chạy một lần với danh sách URLs từ file
./monsterinc --mode onetime --scan-targets targets.txt

# Chạy tự động với giám sát
./monsterinc --mode automated --monitor-targets monitor_targets.txt

# Sử dụng file cấu hình tùy chỉnh
./monsterinc --mode onetime --globalconfig custom_config.yaml --scan-targets targets.txt

# Chạy automated mode với cả scan và monitor
./monsterinc --mode automated --scan-targets scan_targets.txt --monitor-targets monitor_targets.txt
```

## Cấu hình

### File cấu hình

Ứng dụng tìm kiếm file cấu hình theo thứ tự:
1. `config.yaml` (ưu tiên)
2. `config.json` (dự phòng)

Copy `config.example.yaml` thành `config.yaml` và chỉnh sửa theo nhu cầu:

```bash
cp config.example.yaml config.yaml
```

### Các section cấu hình chính


- **httpx_runner_config**: Cài đặt cho httpx probing
- **crawler_config**: Cấu hình web crawling
- **reporter_config**: Cài đặt tạo báo cáo HTML
- **storage_config**: Cấu hình lưu trữ Parquet
- **notification_config**: Cài đặt thông báo Discord
- **monitor_config**: Cấu hình giám sát file
- **secrets_config**: Cài đặt phát hiện secret
- **scheduler_config**: Cấu hình automated mode
- **extractor_config**: Cài đặt trích xuất path
- **diff_config**: Cấu hình so sánh dữ liệu
- **log_config**: Cấu hình logging

## Cấu trúc thư mục

```
monsterinc/
├── cmd/
│   └── monsterinc/             # Điểm vào ứng dụng
├── internal/                   # Logic ứng dụng nội bộ
│   ├── common/                # Utilities và patterns chung
│   ├── config/                # Quản lý cấu hình
│   ├── crawler/               # Module web crawling
│   ├── datastore/             # Module lưu trữ dữ liệu (Parquet)
│   ├── differ/                # Module so sánh thay đổi
│   ├── extractor/             # Module trích xuất path
│   ├── httpxrunner/           # Wrapper httpx
│   ├── logger/                # Module logging
│   ├── models/                # Định nghĩa cấu trúc dữ liệu
│   ├── monitor/               # Module giám sát file
│   ├── notifier/              # Module thông báo
│   ├── orchestrator/          # Điều phối workflow
│   ├── reporter/              # Tạo báo cáo HTML
│   ├── scheduler/             # Lập lịch quét tự động
│   ├── secrets/               # Phát hiện secret
│   └── urlhandler/            # Xử lý và chuẩn hóa URL
├── reports/                   # Thư mục báo cáo HTML
│   ├── scan/                  # Báo cáo scan
│   └── diff/                  # Báo cáo diff
├── database/                  # Database và file Parquet
│   ├── scan/                  # Dữ liệu scan
│   ├── monitor/               # Dữ liệu monitor
│   ├── scheduler/             # SQLite database cho scheduler
│   └── secrets/               # Secret findings
├── target/                    # File target lists
├── tasks/                     # File PRD và task lists
├── config.example.yaml        # File cấu hình mẫu
└── README.md                  # File này
```

## Workflow hoạt động

### Onetime Mode
1. **Khởi tạo**: Load cấu hình, khởi tạo logger và notification
2. **Thu thập Target**: Xác định seed URLs từ file hoặc config
3. **Crawling**: Thu thập URLs từ seed URLs
4. **Probing**: Thực hiện HTTP/HTTPS probing với httpx
5. **Diffing**: So sánh với dữ liệu lịch sử từ Parquet
6. **Secret Detection**: Quét nội dung tìm secrets (nếu được bật)
7. **Path Extraction**: Trích xuất paths từ nội dung JS/HTML
8. **Storage**: Lưu kết quả vào file Parquet
9. **Reporting**: Tạo báo cáo HTML
10. **Notification**: Gửi thông báo hoàn thành

### Automated Mode
1. **Scheduler**: Tính toán thời gian quét tiếp theo dựa trên cấu hình
2. **Target Reloading**: Tải lại targets cho mỗi chu kỳ
3. **Scan Execution**: Thực thi workflow như onetime mode
4. **History Management**: Lưu lịch sử quét vào SQLite
5. **Retry Logic**: Retry nếu quét thất bại
6. **File Monitoring**: Giám sát thay đổi file JS/HTML (nếu được bật)

## Database Schema

### Parquet Files
- **scan data**: `database/scan/<hostname>/data.parquet`
- **file history**: `database/monitor/<hostname>/file_history.parquet`
- **secrets**: `database/secrets/findings.parquet`

### SQLite Database
- **scan_history**: Lưu trữ lịch sử quét trong automated mode
- Columns: scan_session_id, target_source, num_targets, scan_start_time, scan_end_time, status, report_file_path, diff_new, diff_old, diff_existing

## Logging và Thông báo

- Sử dụng `zerolog` cho structured logging
- Hỗ trợ thông báo Discord cho:
  - Sự kiện lifecycle quét
  - Thông báo thay đổi file
  - Lỗi nghiêm trọng
  - Secrets độ nghiêm trọng cao
  - Báo cáo diff tổng hợp

## Dependencies chính

- [colly](https://github.com/gocolly/colly) - Web crawling
- [httpx](https://github.com/projectdiscovery/httpx) - HTTP probing
- [parquet-go](https://github.com/parquet-go/parquet-go) - Xử lý file Parquet
- [zerolog](https://github.com/rs/zerolog) - Structured logging
- [jsluice](https://github.com/BishopFox/jsluice) - Phân tích JavaScript
- [sqlite](https://modernc.org/sqlite) - SQLite database (CGO-free)
- [trufflehog](https://github.com/trufflesecurity/trufflehog) - Secret detection
- [fsnotify](https://github.com/fsnotify/fsnotify) - File system watching

## Đóng góp

1. Fork repository
2. Tạo feature branch (`git checkout -b feature/amazing-feature`)
3. Commit thay đổi (`git commit -m 'feat: add amazing feature'`)
4. Push lên branch (`git push origin feature/amazing-feature`)
5. Tạo Pull Request

## License

Project này được phân phối dưới MIT License. Xem file `LICENSE` để biết thêm chi tiết.

## Hỗ trợ

- Create [GitHub Issue](https://github.com/aleister1102/monsterinc/issues) to report bugs or suggest features
- See [Wiki](./WIKI.md) for more details about project structure and operation 
