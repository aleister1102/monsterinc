# Datastore Package (`internal/datastore`)

This package is responsible for handling the storage and retrieval of scan data, primarily using Parquet files.

## Core Components

1.  **`parquet_writer.go`**:
    *   Defines `ParquetWriter` for writing probe results to Parquet files.
    *   **Key Change**: Instead of creating a new Parquet file for each `scanSessionID`, the `Write` method now manages a *single `data.parquet` file per root target*.
    *   The directory structure for a target like "example.com" would be `database/example.com/data.parquet`.
    *   The `Write` operation involves:
        1.  Reading existing probe results for the target using `ParquetReader.FindAllProbeResultsForTarget`.
        2.  Merging the current scan's probe results with the historical data. This involves updating existing records (preserving `FirstSeenTimestamp`, updating `LastSeenTimestamp`, and other fields) and adding new records (setting both timestamps to the current scan time).
        3.  Overwriting the single `data.parquet` file with the merged results.
    *   Handles data transformation from `models.ProbeResult` to `models.ParquetProbeResult`, including timestamp management for `ScanTimestamp`, `FirstSeenTimestamp`, and `LastSeenTimestamp`.
    *   Supports various compression codecs (defaulting to Zstd).

2.  **`parquet_reader.go`**:
    *   Defines `ParquetReader` for reading probe results from Parquet files.
    *   `FindAllProbeResultsForTarget(rootTargetURL string) ([]models.ProbeResult, time.Time, error)`: This is the primary method for retrieving data. It reads the consolidated `data.parquet` file for a given `rootTargetURL` and returns all `models.ProbeResult` records found within, along with the file's last modification time.
    *   The method `FindMostRecentScanURLs` has been removed as it's no longer applicable with the single-file-per-target model.
    *   `readProbeResultsFromSpecificFile` is a helper function used internally.

## Key Features & Logic

-   **Single Parquet File per Target**: Each unique root target (e.g., "example.com", "sub.example.org") has its own directory under `storage_config.parquet_base_path` (e.g., `database/example.com/`). Inside this directory, a single file named `data.parquet` stores all historical and current probe data for that target.
-   **Data Merging**: When new scan data is written, it's merged with existing data in `data.parquet`.
    -   **New URLs**: Added with `FirstSeenTimestamp` and `LastSeenTimestamp` set to the current scan time.
    -   **Existing URLs**: Fields are updated from the current scan, `LastSeenTimestamp` is updated to the current scan time, and `FirstSeenTimestamp` is preserved from the historical record.
    -   **Old URLs** (present in historical data but not in the current scan): These are currently kept in the merged results by default, preserving their historical data and timestamps. The `URLStatus` field (set by the `differ` package) helps identify them.
-   **Timestamp Handling**:
    -   `ScanTimestamp` (in Parquet): Unix milliseconds of the scan session that wrote/updated the record.
    -   `FirstSeenTimestamp` (in Parquet): Unix milliseconds of when the URL was first ever recorded.
    -   `LastSeenTimestamp` (in Parquet): Unix milliseconds of when the URL was last seen/updated in a scan.
-   Parquet files are compressed (default: Zstd) for efficiency.

## Usage Flow

1.  **Initialization**:
    *   `NewParquetReader` is initialized with `StorageConfig` and a logger.
    *   `NewParquetWriter` is initialized with `StorageConfig`, a logger, and crucially, an instance of `ParquetReader`. The reader is necessary for the writer's merge logic.
2.  **Writing Data** (e.g., in `ScanOrchestrator` after a scan):
    *   `ParquetWriter.Write(currentProbeResults, scanSessionID, rootTarget)` is called.
    *   Internally, `Write` uses its `parquetReader` instance to call `FindAllProbeResultsForTarget(rootTarget)` to fetch historical data.
    *   Current and historical data are merged.
    *   The merged data is written to `database/<sanitized_rootTarget>/data.parquet`, overwriting the file.
3.  **Reading Data** (e.g., in `UrlDiffer` before comparison, or for reporting):
    *   `ParquetReader.FindAllProbeResultsForTarget(rootTarget)` is called to get all data for a specific target.

## Configuration

Relies on `StorageConfig` from the global configuration:
-   `ParquetBasePath`: The root directory where target-specific subdirectories and their `data.parquet` files will be stored.
-   `CompressionCodec`: The compression algorithm for Parquet files (e.g., "zstd", "snappy", "gzip").

## Example (Conceptual)

```go
// Initialization
storageCfg := config.GetStorageConfig() // Assume this gets the config
appLogger := logger.New(...) // Assume logger is initialized
pqReader := datastore.NewParquetReader(storageCfg, appLogger)
pqWriter, err := datastore.NewParquetWriter(storageCfg, appLogger, pqReader)
if err != nil {
    // Handle error
}

// --- During a scan for "example.com" ---
currentScanResults := []models.ProbeResult{ ... } // Results from current scan
scanSessionID := "20230101-120000"
rootTarget := "example.com"

// Write/Update data for example.com
err = pqWriter.Write(currentScanResults, scanSessionID, rootTarget)
if err != nil {
    // Handle error
}

// --- Later, for diffing or reporting "example.com" ---
allHistoricalAndCurrentData, modTime, err := pqReader.FindAllProbeResultsForTarget("example.com")
if err != nil {
    // Handle error
}
// Now 'allHistoricalAndCurrentData' contains all known probe results for "example.com"
```

## Data Models

- Utilizes `models.ProbeResult` for current scan data and `models.ParquetProbeResult` for the schema written to/read from Parquet files.

## Logging

- Both `ParquetReader` and `ParquetWriter` accept a `log.Logger` instance for logging their operations, including detailed steps, errors encountered, and summaries of data processed.

## Ghi dữ liệu Parquet (sử dụng `parquet-go/parquet-go`)

Thành phần chính là `ParquetWriter`, sử dụng thư viện `github.com/parquet-go/parquet-go` để ghi dữ liệu từ `models.ProbeResult` vào định dạng Parquet.

### Schema

-   Schema cho file Parquet được định nghĩa bởi struct `models.ParquetProbeResult` trong file `internal/models/parquet_schema.go`.
-   Struct này sử dụng các field tags theo quy ước của `parquet-go/parquet-go` (ví dụ: ``parquet:"column_name,optional"``).
-   `parquet-go/parquet-go` tự suy luận schema từ struct và các tags của nó.

### Cách sử dụng

1.  **Khởi tạo `ParquetWriter`**:
    Gọi `NewParquetWriter(cfg *config.StorageConfig, appLogger *log.Logger)`.
    -   `cfg`: Một con trỏ đến `config.StorageConfig`.
    -   `appLogger`: Một logger.
    -   Hàm này sẽ tạo thư mục `ParquetBasePath` nếu nó chưa tồn tại.

2.  **Gọi phương thức `Write()`**:
    `Write(probeResults []models.ProbeResult, scanSessionID string, rootTarget string)`
    -   `probeResults`: Một slice các `models.ProbeResult` cần được ghi.
    -   `scanSessionID`: ID định danh cho phiên quét (chủ yếu dùng cho logging).
    -   `rootTarget`: URL gốc của mục tiêu quét, dùng để tạo tên file.

    Phương thức `Write()` sẽ thực hiện các bước sau:
    -   Nếu `probeResults` rỗng, sẽ ghi log và không tạo file.
    -   Tạo tên file Parquet bằng cách làm sạch `rootTarget` (ví dụ: `example_com.parquet`).
    -   File sẽ được lưu trực tiếp vào `ParquetBasePath`. **File hiện tại (nếu có) sẽ bị ghi đè.**
    -   Chuyển đổi từng `models.ProbeResult` thành `models.ParquetProbeResult`.
    -   Sử dụng `parquet.NewGenericWriter[models.ParquetProbeResult]` để ghi dữ liệu.
    -   Áp dụng thuật toán nén được chỉ định trong `config.StorageConfig.CompressionCodec`.
    -   Ghi log chi tiết.

### Xử lý trường tùy chọn (Optional Fields)

-   Trong `models.ParquetProbeResult`, các trường optional được định nghĩa là con trỏ.
-   `transformToParquetResult` đảm bảo các trường rỗng/nil được xử lý đúng cách.

### Tổ chức file

Các file Parquet sẽ được lưu theo cấu trúc:

```
<ParquetBasePath>/<sanitized_root_target_name>.parquet
```

Ví dụ: `database/example_com.parquet`

### Phụ thuộc

-   `github.com/parquet-go/parquet-go`

Đảm bảo rằng thư viện này được liệt kê chính xác trong file `go.mod` của dự án. 