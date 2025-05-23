## Relevant Files

- `internal/datastore/parquet.go` - Core Parquet schema definition and writing logic.
- `internal/datastore/file_manager.go` - Logic for file naming and directory structure.
- `internal/httpxrunner/probe.go` - Integration point with `httpx-probing` module.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [ ] 1.0 Define Parquet Schema for HTTP/S Probe Results
  - [ ] 1.1 Define Go structs to represent the Parquet schema in `internal/datastore/parquet.go`.
  - [ ] 1.2 Define Parquet schema using a Go Parquet library in `internal/datastore/parquet.go`.
- [ ] 2.0 Implement Parquet File Writing Logic
  - [ ] 2.1 Implement logic to write data to Parquet files in `internal/datastore/parquet.go`.
  - [ ] 2.2 Implement logic to handle empty input lists in `internal/datastore/parquet.go`.
- [ ] 3.0 Implement File Naming and Directory Structure Logic
  - [ ] 3.1 Implement logic to create directory structure in `internal/datastore/file_manager.go`.
  - [ ] 3.2 Implement logic to generate unique filenames in `internal/datastore/file_manager.go`.
- [ ] 4.0 Implement Compression Logic
  - [ ] 4.1 Implement Zstandard compression for Parquet files in `internal/datastore/parquet.go`.
- [ ] 5.0 Implement Error Handling and Logging
  - [ ] 5.1 Implement error handling for Parquet file writing in `internal/datastore/parquet.go`.
  - [ ] 5.2 Implement error handling for file management in `internal/datastore/file_manager.go`.
  - [ ] 5.3 Implement logging for errors and events in `internal/datastore/parquet.go` and `internal/datastore/file_manager.go`.
- [ ] 6.0 Implement Integration with `httpx-probing` Module
  - [ ] 6.1 Implement logic to receive data from `httpx-probing` in `internal/httpxrunner/probe.go`.
  - [ ] 6.2 Implement logic to pass data to Parquet storage in `internal/httpxrunner/probe.go`. 