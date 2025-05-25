# Package datastore

Package `datastore` chịu trách nhiệm xử lý việc lưu trữ dữ liệu đã được xử lý, hiện tại tập trung vào việc ghi kết quả probe (thăm dò) vào các file Parquet.

## Ghi dữ liệu Parquet (sử dụng `parquet-go/parquet-go`)

Thành phần chính là `ParquetWriter`, sử dụng thư viện `github.com/parquet-go/parquet-go` để ghi dữ liệu từ `models.ProbeResult` vào định dạng Parquet.

### Schema

-   Schema cho file Parquet được định nghĩa bởi struct `models.ParquetProbeResult` trong file `internal/models/parquet_schema.go`.
-   Struct này sử dụng các field tags theo quy ước của `parquet-go/parquet-go` (ví dụ: ``parquet:"column_name,optional"``).
-   **Không cần** chạy công cụ `parquetgen` như khi sử dụng thư viện `parsyl/parquet` trước đây. `parquet-go/parquet-go` tự suy luận schema từ struct và các tags của nó.

### Cách sử dụng

1.  **Khởi tạo `ParquetWriter`**:
    Gọi `NewParquetWriter(cfg *config.StorageConfig, appLogger *log.Logger)`.
    -   `cfg`: Một con trỏ đến `config.StorageConfig`, chứa các thông tin như `ParquetBasePath` (đường dẫn thư mục gốc lưu file Parquet) và `CompressionCodec` (mã codec nén).
    -   `appLogger`: Một logger để ghi lại các hoạt động.
    -   Hàm này sẽ tạo thư mục `ParquetBasePath` nếu nó chưa tồn tại.

2.  **Gọi phương thức `Write()`**:
    `Write(probeResults []models.ProbeResult, scanSessionID string, rootTarget string)`
    -   `probeResults`: Một slice các `models.ProbeResult` cần được ghi.
    -   `scanSessionID`: Một ID định danh cho phiên quét (ví dụ: timestamp hoặc một UUID).
    -   `rootTarget`: URL gốc của mục tiêu quét, dùng để tạo tên file và thư mục.

    Phương thức `Write()` sẽ thực hiện các bước sau:
    -   Nếu `probeResults` rỗng, sẽ ghi log và không tạo file.
    -   Tạo một thư mục con bên trong `ParquetBasePath` dựa trên ngày hiện tại (ví dụ: `YYYYMMDD`).
    -   Tạo tên file Parquet dựa trên `rootTarget` (sau khi đã được làm sạch để loại bỏ các ký tự không hợp lệ cho tên file) và `scanSessionID`. Ví dụ: `scan_results_<cleaned_root_target>_<scanSessionID>.parquet`.
    -   Chuyển đổi từng `models.ProbeResult` thành `models.ParquetProbeResult` bằng phương thức `transformToParquetResult`. Phương thức này xử lý việc ánh xạ các trường, chuyển đổi kiểu dữ liệu (ví dụ: `time.Time` sang `*int64` cho Unix milliseconds), và xử lý các giá trị rỗng/nil thành con trỏ nil cho các trường optional trong Parquet.
    -   Sử dụng `parquet.NewGenericWriter[models.ParquetProbeResult]` từ thư viện `parquet-go/parquet-go` để ghi dữ liệu.
    -   Áp dụng thuật toán nén được chỉ định trong `config.StorageConfig.CompressionCodec`. Các lựa chọn hỗ trợ bao gồm: "ZSTD", "SNAPPY", "GZIP", "UNCOMPRESSED". Nếu một codec không được hỗ trợ được chỉ định, nó sẽ mặc định là "UNCOMPRESSED".
    -   Ghi log chi tiết về quá trình ghi và các lỗi có thể xảy ra.

### Xử lý trường tùy chọn (Optional Fields)

-   Trong struct `models.ParquetProbeResult`, các trường có thể không có giá trị (optional) được định nghĩa là con trỏ (ví dụ: `*string`, `*int32`).
-   Phương thức `transformToParquetResult` đảm bảo rằng nếu một trường trong `models.ProbeResult` không có giá trị (ví dụ: chuỗi rỗng, số 0 cho các trường số khi có lỗi), trường tương ứng trong `models.ParquetProbeResult` sẽ là `nil`.

### Tổ chức file

Các file Parquet sẽ được lưu theo cấu trúc:

```
<ParquetBasePath>/<YYYYMMDD>/scan_results_<cleaned_root_target>_<scanSessionID>.parquet
```

### Phụ thuộc

-   `github.com/parquet-go/parquet-go`

Đảm bảo rằng thư viện này được liệt kê chính xác trong file `go.mod` của dự án. 