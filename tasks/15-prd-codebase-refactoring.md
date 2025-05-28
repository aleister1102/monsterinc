# PRD: Codebase Refactoring

## Introduction/Overview

Dự án MonsterInc hiện tại có các vấn đề về code quality cần được giải quyết thông qua việc refactoring toàn bộ codebase. Các vấn đề chính bao gồm: code duplication, comments dài dòng, unused dependencies, excessive logging noise, và các patterns không consistent. Mục tiêu của việc refactor này là tạo ra một codebase dễ maintain, dễ extend features mới, và giảm thiểu technical debt.

## Goals

1. **Giảm Code Duplication**: Xác định và loại bỏ các patterns code trùng lặp trong toàn bộ dự án
2. **Clean Up Comments**: Rút ngọn các comments dài dòng và chỉ giữ lại những comments cần thiết
3. **Remove Unused Dependencies**: Xóa các dependencies không sử dụng trong go.mod
4. **Reduce Logging Noise**: Giảm thiểu các log Info/Debug không cần thiết
5. **Improve Maintainability**: Tạo codebase dễ maintain và extend features mới
6. **Maintain Backward Compatibility**: Đảm bảo functionality không thay đổi sau mỗi phase refactor

## User Stories

1. **Là một developer**, tôi muốn codebase có ít code duplication để dễ dàng maintain và add features mới mà không phải copy-paste logic.

2. **Là một developer**, tôi muốn comments ngắn gọn và to-the-point để dễ đọc code mà không bị phân tâm bởi comments dài dòng.

3. **Là một developer**, tôi muốn dependencies clean để build time nhanh hơn và dependency tree đơn giản hơn.

4. **Là một system operator**, tôi muốn logs ít noise hơn để dễ debug và monitor system.

5. **Là một developer**, tôi muốn consistent patterns trong toàn bộ codebase để dễ hiểu và contribute.

## Functional Requirements

### FR1: Code Duplication Elimination
1. **FR1.1**: Tạo shared utility functions cho các patterns error handling thường dùng
2. **FR1.2**: Consolidate các constructor patterns tương tự (tất cả New* functions đều nhận logger và config)
3. **FR1.3**: Tạo shared interfaces cho common behaviors (Validator, Initializer, etc.)
4. **FR1.4**: Extract common HTTP client configuration logic
5. **FR1.5**: Consolidate file I/O patterns và error handling

### FR2: Comment Cleanup
2. **FR2.1**: Loại bỏ các comments dài hơn 2 dòng mà chỉ giải thích code logic rõ ràng
3. **FR2.2**: Chỉ giữ lại comments cho complex business logic hoặc external dependencies
4. **FR2.3**: Xóa các TODO/FIXME comments đã outdated
5. **FR2.4**: Standardize comment format (// TODO: thay vì // Task X.X)

### FR3: Dependency Management
6. **FR3.1**: Audit tất cả dependencies trong go.mod
7. **FR3.2**: Xóa các dependencies không được import trong bất kỳ file Go nào
8. **FR3.3**: Kiểm tra indirect dependencies có thể remove được không
9. **FR3.4**: Update go.mod và go.sum sau khi cleanup

### FR4: Logging Optimization
10. **FR4.1**: Audit tất cả log.Info() và log.Debug() calls
11. **FR4.2**: Giữ lại chỉ essential Info logs (startup, shutdown, major operations)
12. **FR4.3**: Convert unnecessary Info logs thành Debug logs
13. **FR4.4**: Xóa redundant debug logs trong hot paths
14. **FR4.5**: Standardize log messages format

### FR5: Configuration Consistency
15. **FR5.1**: Đảm bảo tất cả New* functions có consistent parameter order (config, logger, dependencies)
16. **FR5.2**: Standardize error handling patterns
17. **FR5.3**: Maintain YAML config format nhưng có thể restructure nội dung

## Non-Goals (Out of Scope)

1. **Performance optimization**: Refactor này focus vào code quality, không phải performance
2. **API changes**: Không thay đổi public APIs hoặc command-line interfaces
3. **Feature additions**: Không thêm features mới trong quá trình refactor
4. **Architecture redesign**: Không thay đổi kiến trúc tổng thể của system
5. **Test additions**: Focus vào cleanup code hiện tại, không viết thêm tests

## Technical Considerations

- **Backward Compatibility**: Mỗi phase phải đảm bảo functionality không đổi
- **Phase-based Approach**: Refactor theo từng module để dễ review và test
- **Go Module Dependencies**: Sử dụng `go mod tidy` và `go mod why` để kiểm tra dependencies
- **Logging Framework**: Tiếp tục sử dụng zerolog nhưng optimize usage patterns
- **Configuration**: Maintain YAML format với có thể restructure schema

## Success Metrics

1. **Code Duplication**: Giảm ít nhất 30% số dòng code duplicate (measured by tools như gocyclo hoặc manual review)
2. **Comment Density**: Giảm comment-to-code ratio từ hiện tại xuống dưới 15%
3. **Dependencies**: Loại bỏ ít nhất 10-15 unused dependencies
4. **Log Volume**: Giảm 50% số log messages ở level Info trong normal operations
5. **Build Time**: Maintain hoặc improve build time sau khi remove dependencies
6. **Functionality**: 100% functional tests pass sau mỗi phase

## Open Questions

1. **Shared Packages**: Có nên tạo internal/common package cho shared utilities không?
2. **Error Handling**: Có nên standardize error wrapping patterns không?
3. **Interface Extraction**: Interfaces nào nên được extract để improve testability?
4. **Config Restructuring**: Config structure có nên được optimize không (vẫn giữ YAML format)?
5. **Testing Strategy**: Làm sao đảm bảo refactor không break functionality mà không có comprehensive tests? 