## Relevant Files

- `internal/httpxrunner/probe.go` - Core probing logic.
- `internal/httpxrunner/client.go` - HTTP client configuration.
- `internal/httpxrunner/techdetector.go` - Technology detection logic.
- `internal/httpxrunner/dns.go` - DNS resolution logic.
- `internal/datastore/parquet.go` - Parquet storage for probe results.
- `internal/config/config.go` - Configuration for probing settings.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [ ] 1.0 Implement HTTP Client Core
  - [ ] 1.1 Implement configurable HTTP client in `internal/httpxrunner/client.go`.
  - [ ] 1.2 Implement retry and timeout logic in `internal/httpxrunner/client.go`.
  - [ ] 1.3 Implement proxy and custom headers support in `internal/httpxrunner/client.go`.
- [ ] 2.0 Implement Probing Logic
  - [ ] 2.1 Implement URL probing with concurrency in `internal/httpxrunner/probe.go`.
  - [ ] 2.2 Implement redirect handling in `internal/httpxrunner/probe.go`.
  - [ ] 2.3 Implement response data extraction in `internal/httpxrunner/probe.go`.
- [ ] 3.0 Implement Technology Detection
  - [ ] 3.1 Implement Wappalyzer-like detection engine in `internal/httpxrunner/techdetector.go`.
  - [ ] 3.2 Implement technology signature matching in `internal/httpxrunner/techdetector.go`.
  - [ ] 3.3 Implement technology result formatting in `internal/httpxrunner/techdetector.go`.
- [ ] 4.0 Implement DNS Resolution
  - [ ] 4.1 Implement IP address resolution in `internal/httpxrunner/dns.go`.
  - [ ] 4.2 Implement CNAME record lookup in `internal/httpxrunner/dns.go`.
  - [ ] 4.3 Implement DNS caching in `internal/httpxrunner/dns.go`.
- [ ] 5.0 Implement Configuration Management
  - [ ] 5.1 Define probing settings in configuration structure in `internal/config/config.go`.
  - [ ] 5.2 Implement configuration loading for probing settings in `internal/config/config.go`.
- [ ] 6.0 Implement Error Handling and Logging
  - [ ] 6.1 Implement error handling for HTTP requests in `internal/httpxrunner/probe.go`.
  - [ ] 6.2 Implement error handling for DNS resolution in `internal/httpxrunner/dns.go`.
  - [ ] 6.3 Implement error handling for technology detection in `internal/httpxrunner/techdetector.go`.
  - [ ] 6.4 Implement logging for probing operations in all relevant files. 