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

- [ ] 1.0 Setup Parquet Writing Core
  - [ ] 1.1 Choose and add a Go Parquet library to `go.mod` (e.g., `github.com/xitongsys/parquet-go`).
  - [ ] 1.2 Define the `ParquetProbeResult` struct in `internal/models/parquet_schema.go` based on FR3 (OriginalURL, FinalURL, StatusCode, ContentLength, ContentType, Title, ServerHeader, Technologies ([]string), IPAddress ([]string), CNAMERecord ([]string - to be removed as per previous changes), RedirectChain ([]string), ScanTimestamp, RootTargetURL, ProbeError).
        *Note: Review if CNAMERecord and RedirectChain are still relevant after previous `httpxrunner` changes. Assume `ScanTimestamp` is `time.Time` and `RootTargetURL` is provided or derivable.*
  - [ ] 1.3 Create `ParquetWriter` struct in `internal/datastore/parquet_writer.go`.
  - [ ] 1.4 Implement `NewParquetWriter(cfg *config.StorageConfig)` in `parquet_writer.go`.
  - [ ] 1.5 Implement `Write(probeResults []httpxrunner.ProbeResult, scanSessionID string, rootTargets []string)` method in `ParquetWriter`.
        *Consider how `RootTargetURL` per `ProbeResult` will be determined if not directly available. It might need to be passed or inferred.*
  - [ ] 1.6 Handle the case where `probeResults` is empty (log and do not generate file - FR2).

- [ ] 2.0 Implement Data Transformation and Schema Mapping
  - [ ] 2.1 In `Write` method, transform `[]httpxrunner.ProbeResult` to `[]models.ParquetProbeResult`.
  - [ ] 2.2 Ensure correct mapping of all fields, including handling of slices (e.g., `Technologies`, `IPAddress`) and potential null/empty values for nullable Parquet fields.
  - [ ] 2.3 Define the Parquet schema within `parquet_writer.go` using the chosen library, matching `models.ParquetProbeResult`.
  - [ ] 2.4 Add `ScanTimestamp` (e.g., current time at the start of `Write`) and `RootTargetURL` to each `ParquetProbeResult`.
        *If multiple root targets, each original `ProbeResult` might need to be associated with its root, or this needs to be handled in input.*

- [ ] 3.0 File Naming, Directory Structure, and Compression
  - [ ] 3.1 Implement logic in `ParquetWriter.Write` to create the base `data` directory if it doesn't exist (FR5.1).
  - [ ] 3.2 Implement logic to create dated subdirectories (`YYYYMMDD`) within the base directory (FR5.2).
  - [ ] 3.3 Generate unique Parquet filenames, e.g., `scan_results_<scanSessionID_or_timestamp>.parquet` (FR5.4).
  - [ ] 3.4 Implement Zstandard compression when writing the Parquet file (FR6).
        *Verify library support and usage for Zstandard.*

- [ ] 4.0 Integration and Error Handling
  - [ ] 4.1 Update `internal/config/storage_config.go` to include `ParquetBasePath` (e.g., default "data").
  - [ ] 4.2 Update `GlobalConfig` to include `StorageConfig`.
  - [ ] 4.3 Update `cmd/monsterinc/main.go` to initialize `ParquetWriter` with the config.
  - [ ] 4.4 In `main.go`, after `httpx` probing finishes and results are available, call `ParquetWriter.Write()`.
        *Generate a `scanSessionID` (e.g., timestamp string) to pass to `Write`.*
  - [ ] 4.5 Implement robust error handling in `ParquetWriter.Write` for file I/O, directory creation, and Parquet writing errors (FR7).
  - [ ] 4.6 Add detailed logging for successful writes and any errors encountered in `ParquetWriter`.
  - [ ] 4.7 Write unit tests for `ParquetWriter`, covering data transformation, schema application, file/directory logic, and error conditions. 