## Relevant Files

- `internal/datastore/parquet_writer.go` - Core logic for writing probe results to Parquet files.
- `internal/datastore/parquet_writer_test.go` - Unit tests for `parquet_writer.go`.
- `internal/models/parquet_schema.go` - Defines the Go struct matching the Parquet schema for probe results (e.g., `ParquetProbeResult`).
- `internal/config/storage_config.go` - Configuration related to data storage, including Parquet base path and compression settings.
- `cmd/monsterinc/main.go` - To integrate the call to the Parquet writer after `httpx` probing.
- `go.mod` - To add the chosen Parquet library (e.g., `github.com/xitongsys/parquet-go`).

### Notes

- The chosen Parquet library (e.g., `github.com/xitongsys/parquet-go` or `github.com/segmentio/parquet-go`) will need to be added to `go.mod` via `go get`.
- Ensure the Parquet schema is well-defined, handles nullable fields correctly, and maps Go types appropriately (e.g., `[]string` to `LIST` of `UTF8`).
- Unit tests should cover schema correctness, data type mapping, file output, directory creation, and compression.
- Use `go test ./...` to run tests.

## Tasks

- [X] 1.0 Setup Parquet Writing Core
  - [X] 1.1 Choose and add a Go Parquet library to `go.mod` (e.g., `github.com/parquet-go/parquet-go`).
  - [X] 1.2 Define the `ParquetProbeResult` struct in `internal/models/parquet_schema.go` based on FR3.
  - [X] 1.3 Create `ParquetWriter` struct in `internal/datastore/parquet_writer.go`.
  - [X] 1.4 Implement `NewParquetWriter(cfg *config.StorageConfig)` in `parquet_writer.go`.
  - [X] 1.5 Implement `Write(probeResults []models.ProbeResult, scanSessionID string, rootTarget string)` method in `ParquetWriter` (signature adjusted).
  - [X] 1.6 Handle the case where `probeResults` is empty (log and do not generate file - FR2).

- [X] 2.0 Implement Data Transformation and Schema Mapping
  - [X] 2.1 In `Write` method, transform `[]models.ProbeResult` to `[]models.ParquetProbeResult` (input type is `models.ProbeResult`).
  - [X] 2.2 Ensure correct mapping of all fields, including handling of slices and potential null/empty values.
  - [X] 2.3 Define the Parquet schema via struct tags in `models.ParquetProbeResult` (no separate definition in writer needed for `parquet-go/parquet-go`).
  - [X] 2.4 Add `ScanTimestamp` and `RootTargetURL` to each `ParquetProbeResult`.

- [X] 3.0 File Naming, Directory Structure, and Compression
  - [X] 3.1 Implement logic in `ParquetWriter.Write` to create the base directory (via `NewParquetWriter`).
  - [X] 3.2 Implement logic to create dated subdirectories (`YYYYMMDD`).
  - [X] 3.3 Generate unique Parquet filenames, e.g., `scan_results_<scanSessionID_or_timestamp>.parquet` (FR5.4).
  - [X] 3.4 Implement Zstandard (and other configurable) compression when writing the Parquet file (FR6).

- [X] 4.0 Integration and Error Handling
  - [X] 4.1 Update `internal/config/config.go` to include `ParquetBasePath`.
  - [X] 4.2 Update `GlobalConfig` to include `StorageConfig`.
  - [X] 4.3 Update `cmd/monsterinc/main.go` to initialize `ParquetWriter` with the config.
  - [X] 4.4 In `main.go`, after `httpx` probing finishes and results are available, call `ParquetWriter.Write()`.
  - [X] 4.5 Implement robust error handling in `ParquetWriter.Write`.
  - [X] 4.6 Add detailed logging for successful writes and any errors encountered in `ParquetWriter`.
  - [X] 4.7 Write unit tests for `ParquetWriter`. 