# Crawler Package Structure

Package `crawler` đã được refactor thành nhiều file để dễ bảo trì hơn:

## Files

### `crawler.go` (Core)
- Chứa struct `Crawler` chính
- Builder pattern (`CrawlerBuilder`) 
- Initialization logic
- GetDiscoveredURLs method

### `crawler_handlers.go`
- Các Colly callback handlers:
  - `handleError()` - xử lý lỗi
  - `handleRequest()` - xử lý request
  - `handleResponse()` - xử lý response
- Context checking methods
- Counter management methods

### `crawler_discovery.go`
- URL discovery và processing logic:
  - `DiscoverURL()` - entry point cho URL discovery
  - `processRawURL()` - resolve và validate URL
  - URL scope checking
  - Content length validation
  - Queue management

### `crawler_runner.go`
- Crawler execution logic:
  - `Start()` - khởi động crawling process
  - Seed URL processing
  - Wait for completion
  - Summary logging

### `crawler_helpers.go`
- Utility functions:
  - `getValueOrDefault()`
  - `getIntValueOrDefault()`

### `asset.go`
- Asset extraction từ HTML responses

### `scope.go`
- URL scope filtering logic

## Usage

Package API không thay đổi sau khi refactor. Sử dụng như trước:

```go
crawler, err := crawler.NewCrawler(config, logger)
if err != nil {
    return err
}

crawler.Start(ctx)
urls := crawler.GetDiscoveredURLs()
```

## Benefits

- **Separation of Concerns**: Mỗi file có trách nhiệm rõ ràng
- **Easier Maintenance**: Code dễ đọc và bảo trì hơn
- **Better Testing**: Có thể test từng component riêng biệt
- **Code Reusability**: Các helper functions có thể tái sử dụng 