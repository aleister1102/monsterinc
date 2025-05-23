## Relevant Files

- `internal/normalizer/normalizer.go` - Core URL normalization logic.
- `internal/normalizer/file.go` - File input handling logic.
- `internal/config/normalizer_config.go` - Normalizer configuration.
- `internal/datastore/parquet.go` - Parquet storage for normalized URLs.
- `internal/config/config.go` - Configuration for normalizer settings.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [ ] 1.0 Implement URL Normalization
  - [ ] 1.1 Implement URL parsing and normalization in `internal/normalizer/normalizer.go`.
  - [ ] 1.2 Implement scheme and hostname normalization in `internal/normalizer/normalizer.go`.
  - [ ] 1.3 Implement URL fragment removal in `internal/normalizer/normalizer.go`.
- [ ] 2.0 Implement File Input Handling
  - [ ] 2.1 Implement file reading logic in `internal/normalizer/file.go`.
  - [ ] 2.2 Implement error handling for file operations in `internal/normalizer/file.go`.
  - [ ] 2.3 Implement logging for file processing in `internal/normalizer/file.go`.
- [ ] 3.0 Implement Configuration Management
  - [ ] 3.1 Define normalizer settings in configuration structure in `internal/config/normalizer_config.go`.
  - [ ] 3.2 Implement configuration loading for normalizer settings in `internal/config/normalizer_config.go`.
- [ ] 4.0 Implement Error Handling and Logging
  - [ ] 4.1 Implement error handling for URL normalization in `internal/normalizer/normalizer.go`.
  - [ ] 4.2 Implement error handling for file input in `internal/normalizer/file.go`.
  - [ ] 4.3 Implement logging for normalization operations in all relevant files. 