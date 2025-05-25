# Package `logger`

Package `logger` cung cấp một interface ghi log cơ bản và một triển khai mặc định sử dụng package `log` tiêu chuẩn của Go.

## Chức năng

-   **`Logger` Interface**: Định nghĩa một tập hợp các phương thức ghi log phổ biến:
    -   `Debugf(format string, args ...interface{})`
    -   `Infof(format string, args ...interface{})`
    -   `Warnf(format string, args ...interface{})`
    -   `Errorf(format string, args ...interface{})`
    -   `Fatalf(format string, args ...interface{})`
    -   `Debug(args ...interface{})`
    -   `Info(args ...interface{})`
    -   `Warn(args ...interface{})`
    -   `Error(args ...interface{})`
    -   `Fatal(args ...interface{})`

-   **`stdLogger` Struct**: Một triển khai của `Logger` interface, sử dụng `log.Logger` tiêu chuẩn của Go để thực hiện việc ghi log. Các phương thức `Debug` và `Debugf` trong `stdLogger` hiện tại sẽ không output gì cả, vì `log.Logger` không có level `DEBUG` mặc định. Để có logging theo level, cần một thư viện logging mạnh mẽ hơn.

-   **`NewStdLogger() Logger`**: Hàm khởi tạo, trả về một instance của `stdLogger` được cấu hình để ghi log ra `os.Stdout` với prefix `[MonsterInc] ` và cờ `log.LstdFlags`.

## Mục đích

Việc sử dụng một interface cho logger cho phép dễ dàng thay thế triển khai logger trong tương lai nếu cần một giải pháp logging mạnh mẽ hơn (ví dụ: `logrus`, `zap`) mà không cần thay đổi code ở các package sử dụng logger.

## Hạn chế của `stdLogger`

-   Không hỗ trợ các cấp độ log một cách tường minh (ví dụ: DEBUG, INFO, WARN, ERROR được xử lý như nhau bởi `log.Printf` hoặc `log.Fatalf` cho Fatal).
-   Đặc biệt, các lời gọi `Debug` và `Debugf` tới `stdLogger` sẽ không tạo ra output nào.
-   Không có các tính năng nâng cao như structured logging, output ra nhiều đích, xoay vòng file log (log rotation) tự động (mặc dù `LogConfig` có các trường cho việc này, `stdLogger` chưa tận dụng).

## Cách sử dụng

```go
import "monsterinc/internal/logger"

func main() {
    log := logger.NewStdLogger()
    log.Info("Chương trình bắt đầu")
    log.Warnf("Một cảnh báo: %s", "điều gì đó")
    // log.Debug("Thông tin debug này sẽ không hiển thị với stdLogger")
}
```

Trong tương lai, khi có một triển khai logger mới (ví dụ: `NewAdvancedLogger()`), chỉ cần thay đổi ở nơi khởi tạo logger mà không ảnh hưởng đến các lời gọi logging trong toàn bộ ứng dụng. 