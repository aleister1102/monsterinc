# Package `core`

Package `core` dự kiến chứa các logic nghiệp vụ cốt lõi và các thành phần quản lý trung tâm của ứng dụng MonsterInc.

## Hiện tại

Hiện tại, package này bao gồm:

-   **`TargetManager` (`target_manager.go`)**: 
    -   Struct `TargetManager` được định nghĩa nhưng chưa có nhiều chức năng.
    -   `NewTargetManager()`: Hàm khởi tạo một `TargetManager` mới.
    -   `LoadTargetsFromFile(filePath string) ([]models.Target, error)`: Placeholder cho chức năng load target từ file (hiện tại chưa triển khai logic đọc file thực tế bên trong nó, logic này đang ở `urlhandler`).

## Định hướng tương lai

Package này có thể được mở rộng để bao gồm:

-   Quản lý các phiên quét (scan sessions).
-   Điều phối hoạt động giữa các module khác nhau (crawler, httpxrunner, reporter, datastore).
-   Xử lý các trạng thái và vòng đời của ứng dụng. 