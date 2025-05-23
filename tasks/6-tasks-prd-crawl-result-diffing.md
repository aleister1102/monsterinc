## Relevant Files

- `internal/differ/differ.go` - Core diffing logic.
- `internal/datastore/parquet.go` - Parquet data retrieval logic.
- `internal/config/differ_config.go` - Differ configuration.
- `internal/datastore/parquet.go` - Parquet storage for diff results.
- `internal/config/config.go` - Configuration for differ settings.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [ ] 1.0 Implement Diffing Logic
  - [ ] 1.1 Implement URL comparison logic in `internal/differ/differ.go`.
  - [ ] 1.2 Implement status assignment for URLs in `internal/differ/differ.go`.
  - [ ] 1.3 Implement logging for diffing operations in `internal/differ/differ.go`.
- [ ] 2.0 Implement Parquet Data Retrieval
  - [ ] 2.1 Implement logic to find the latest Parquet file in `internal/datastore/parquet.go`.
  - [ ] 2.2 Implement URL list retrieval from Parquet in `internal/datastore/parquet.go`.
  - [ ] 2.3 Implement error handling for Parquet operations in `internal/datastore/parquet.go`.
- [ ] 3.0 Implement Configuration Management
  - [ ] 3.1 Define differ settings in configuration structure in `internal/config/differ_config.go`.
  - [ ] 3.2 Implement configuration loading for differ settings in `internal/config/differ_config.go`.
- [ ] 4.0 Implement Error Handling and Logging
  - [ ] 4.1 Implement error handling for diffing operations in `internal/differ/differ.go`.
  - [ ] 4.2 Implement error handling for Parquet operations in `internal/datastore/parquet.go`.
  - [ ] 4.3 Implement logging for diffing operations in all relevant files. 