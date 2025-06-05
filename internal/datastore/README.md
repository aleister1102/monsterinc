# Datastore Package

Package `datastore` cung cấp các interface và implementation để lưu trữ và truy xuất dữ liệu trong MonsterInc.

## Architecture

Package này được tổ chức theo kiến trúc modular với Separation of Concerns:

### Core Interfaces và Models
- `models.FileHistoryStore` - Interface chính cho file history operations
- `models.FileHistoryRecord` - Model cho file history records

### File Structure

#### Core Files
- **`parquet_file_history_store.go`** - Core struct, builder pattern, URL mutex management
- **`file_history_config.go`** - Constants và configuration structures
- **`url_hash_generator.go`** - URL hash generation functionality  
- **`url_mutex_manager.go`** - Thread-safe mutex management cho URLs
- **`file_path_generator.go`** - File path generation logic

#### Operations Files
- **`file_history_operations.go`** - Core CRUD operations:
  - `StoreFileRecord()` - Lưu trữ file history record
  - `GetLastKnownRecord()` - Lấy record mới nhất cho URL
  - `GetRecordsForURL()` - Lấy nhiều records cho URL
  - `GetLastKnownHash()` - Lấy hash mới nhất

#### I/O Files  
- **`file_history_readers.go`** - Reading operations:
  - `readFileHistoryRecords()` - Đọc records từ Parquet file
  - `getAndSortRecordsForURL()` - Đọc và sort records cho URL
  - `readRecordsFromFile()` - Helper để đọc từ file

- **`file_history_writers.go`** - Writing operations:
  - `createParquetFile()` - Tạo Parquet file với compression
  - `writeParquetData()` - Ghi data vào Parquet file
  - `loadExistingRecords()` - Load existing records từ file

#### Diff Operations
- **`file_history_diff_operations.go`** - Diff-related operations:
  - `GetAllRecordsWithDiff()` - Lấy tất cả records có diff data
  - `GetAllLatestDiffResultsForURLs()` - Lấy latest diff results cho URLs
  - `getAndSortRecordsForHost()` - Helper cho host-based diff operations
  - `processHostRecordsForDiffs()` - Xử lý diff cho host records

#### Helper Functions
- **`file_history_helpers.go`** - Utility functions:
  - `scanHistoryFile()` - Scan file để tìm diff data
  - `walkDirectoryForDiffs()` - Walk directory để tìm diffs
  - `groupURLsByHost()` - Group URLs theo hostname
  - `GetHostnamesWithHistory()` - Lấy danh sách hostnames có history

#### Other Components
- **`parquet_reader.go`** - ParquetReader cho probe result queries
- **`parquet_writer.go`** - ParquetWriter cho probe result storage

## Key Features

### Thread Safety
- URL-specific mutexes để đảm bảo concurrent access safety
- Configurable mutex management qua `ParquetFileHistoryStoreConfig`

### Compression Support
- Hỗ trợ multiple compression codecs: Snappy, Gzip, Zstd, Uncompressed
- Configurable qua `CompressionCodec` setting

### Organized Storage
- File organization theo hostname/port structure
- Separate files cho mỗi URL's history: `{urlHash}_history.parquet`

### Builder Pattern
- `ParquetFileHistoryStoreBuilder` cho fluent configuration
- `ParquetReaderBuilder` và `ParquetWriterBuilder` cho other components

## Usage

```go
// Tạo store instance
store, err := NewParquetFileHistoryStoreBuilder(logger).
    WithStorageConfig(storageConfig).
    WithConfig(DefaultParquetFileHistoryStoreConfig()).
    Build()

// Store a record
record := models.FileHistoryRecord{
    URL: "https://example.com/file.js",
    Timestamp: time.Now().UnixMilli(),
    Hash: "sha256hash",
    Content: fileContent,
    // ... other fields
}
err = store.StoreFileRecord(record)

// Get latest record
latestRecord, err := store.GetLastKnownRecord("https://example.com/file.js")
```

## Design Principles

1. **Separation of Concerns** - Mỗi file có responsibility rõ ràng
2. **Thread Safety** - Thread-safe operations với URL-specific locking
3. **Performance** - Optimized cho concurrent access và large datasets
4. **Modularity** - Loosely coupled components với clear interfaces
5. **Testability** - Builder pattern và dependency injection để dễ test

## Configuration

Sử dụng `ParquetFileHistoryStoreConfig` để configure:

```go
config := ParquetFileHistoryStoreConfig{
    MaxFileSize:       10 * 1024 * 1024, // 10MB
    EnableCompression: true,
    CompressionCodec:  "zstd",
    EnableURLMutexes:  true,
    CleanupInterval:   3600, // 1 hour
}
``` 