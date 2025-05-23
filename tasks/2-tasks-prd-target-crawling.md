## Relevant Files

- `internal/crawler/crawler.go` - Core crawling logic.
- `internal/crawler/scope.go` - Scope control logic.
- `internal/crawler/asset.go` - Asset discovery logic.
- `internal/config/crawler_config.go` - Crawler configuration.
- `internal/datastore/parquet.go` - Parquet storage for crawl results.
- `internal/config/config.go` - Configuration for crawler settings.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [x] 1.0 Implement Crawler Core
  - [x] 1.1 Implement crawler initialization in `internal/crawler/crawler.go`.
  - [x] 1.2 Implement URL de-duplication in `internal/crawler/crawler.go`.
  - [x] 1.3 Implement logging for crawl operations in `internal/crawler/crawler.go`.
- [x] 2.0 Implement Scope Control
  - [x] 2.1 Implement hostname and subdomain control in `internal/crawler/scope.go`.
  - [x] 2.2 Implement path restriction logic in `internal/crawler/scope.go`.
  - [x] 2.3 Implement robots.txt handling in `internal/crawler/scope.go`.
- [x] 3.0 Implement Asset Discovery
  - [x] 3.1 Implement URL extraction from HTML tags in `internal/crawler/asset.go`.
  - [x] 3.2 Implement asset URL collection in `internal/crawler/asset.go`.
  - [x] 3.3 Implement asset URL validation in `internal/crawler/asset.go`.
- [x] 4.0 Implement Configuration Management
  - [x] 4.1 Define crawler settings in configuration structure in `internal/config/crawler_config.go`.
  - [x] 4.2 Implement configuration loading for crawler settings in `internal/config/crawler_config.go`.
- [x] 5.0 Implement Error Handling and Logging
  - [x] 5.1 Implement error handling for HTTP requests in `internal/crawler/crawler.go`.
  - [x] 5.2 Implement error handling for asset discovery in `internal/crawler/asset.go`.
  - [x] 5.3 Implement logging for crawl operations in all relevant files. 