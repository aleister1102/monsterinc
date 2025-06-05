# HTTPX Runner Package

## Mô tả

Package `httpxrunner` là một wrapper cho thư viện httpx của ProjectDiscovery, được thiết kế để tích hợp dễ dàng vào MonsterInc với các tính năng như configuration management, result mapping, và collection.

## Cấu trúc File

Package đã được refactor thành các file riêng biệt theo nguyên tắc Single Responsibility:

### `config.go`
- `Config` struct: Chứa tất cả configuration cho HTTPX runner
- `DefaultConfig()`: Trả về configuration mặc định

### `options_configurator.go`
- `HTTPXOptionsConfigurator`: Chuyển đổi từ MonsterInc config sang httpx options
- Chứa các method để áp dụng từng loại configuration riêng biệt

### `result_mapper.go`
- `ProbeResultMapper`: Chuyển đổi từ httpx result sang MonsterInc ProbeResult
- Xử lý mapping các field như headers, technologies, network info, ASN

### `result_collector.go`
- `ResultCollector`: Thu thập và quản lý probe results
- Thread-safe với mutex để hỗ trợ concurrent access

### `builder.go`
- `RunnerBuilder`: Builder pattern để tạo Runner instance
- Fluent interface để cấu hình runner

### `runner.go`
- `Runner`: Main struct wrap httpx runner
- Các method để execute và quản lý lifecycle

## Sử dụng

```go
// Tạo runner với builder pattern
runner, err := httpxrunner.NewRunnerBuilder(logger).
    WithConfig(config).
    WithRootTargetURL("https://example.com").
    Build()

if err != nil {
    return err
}

// Chạy runner
err = runner.Run(ctx)
if err != nil {
    return err
}

// Lấy results
results := runner.GetResults()
```

## Lợi ích của Refactor

1. **Single Responsibility**: Mỗi file có một nhiệm vụ rõ ràng
2. **Dễ bảo trì**: Code được tổ chức logic, dễ tìm và sửa
3. **Testability**: Có thể test từng component riêng biệt
4. **Reusability**: Các component có thể tái sử dụng trong context khác
5. **Clean Architecture**: Tuân thủ nguyên tắc clean code 