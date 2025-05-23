## Relevant Files

- `internal/htmljs/trufflehog.go` - Integration with TruffleHog library or CLI.
- `internal/htmljs/custom_regex.go` - Custom regex pattern definitions and loading.
- `internal/htmljs/detector.go` - Core secret detection logic.
- `internal/datastore/parquet.go` - Parquet storage logic for detected secrets.
- `internal/reporter/report.go` - Reporting logic for secret detection findings.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [ ] 1.0 Integrate TruffleHog for Secret Detection
  - [ ] 1.1 Evaluate TruffleHog Go library vs. CLI invocation in `internal/htmljs/trufflehog.go`.
  - [ ] 1.2 Implement TruffleHog integration (library or CLI) in `internal/htmljs/trufflehog.go`.
- [ ] 2.0 Implement Custom Regex Pattern Integration
  - [ ] 2.1 Define custom regex patterns (inspired by `mantra`) in `internal/htmljs/custom_regex.go`.
  - [ ] 2.2 Implement logic to load custom regex patterns into TruffleHog in `internal/htmljs/custom_regex.go`.
- [ ] 3.0 Implement Secret Detection Logic
  - [ ] 3.1 Implement logic to read HTML/JS content from Parquet store in `internal/htmljs/detector.go`.
  - [ ] 3.2 Implement logic to run TruffleHog (with custom regexes) on the content in `internal/htmljs/detector.go`.
  - [ ] 3.3 Implement logic to process and format TruffleHog findings in `internal/htmljs/detector.go`.
- [ ] 4.0 Implement Secret Storage in Parquet Format
  - [ ] 4.1 Define Parquet schema for detected secrets in `internal/datastore/parquet.go`.
  - [ ] 4.2 Implement logic to write detected secrets to Parquet in `internal/datastore/parquet.go`.
- [ ] 5.0 Integrate Secret Detection Findings into Reporting
  - [ ] 5.1 Implement logic to read detected secrets from Parquet in `internal/reporter/report.go`.
  - [ ] 5.2 Implement logic to format secret findings for reports in `internal/reporter/report.go`.
  - [ ] 5.3 Integrate secret findings into existing reporting mechanisms in `internal/reporter/report.go`.
- [ ] 6.0 Implement Performance Optimization and Error Handling
  - [ ] 6.1 Implement performance optimizations for large files in `internal/htmljs/detector.go`.
  - [ ] 6.2 Implement error handling for TruffleHog execution in `internal/htmljs/trufflehog.go`.
  - [ ] 6.3 Implement error handling for Parquet storage in `internal/datastore/parquet.go`. 