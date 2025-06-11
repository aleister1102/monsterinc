# Ngôn ngữ & Giao tiếp
- Giao tiếp bằng tiếng Việt, code bằng tiếng Anh.
- Hạn chế giải thích dài dòng.

# Thực thi & Workflow

- Tự động thực hiện tác vụ cho đến khi hoàn thành, gặp lỗi hoặc hết token mà không cần hỏi.
- Luôn xem xét toàn bộ dự án để tái sử dụng code, giảm thiểu code base.
- Sau mỗi tác vụ lớn, kiểm tra linter toàn bộ dự án và xóa hết các log debug.
- Không tự động commit.

# Môi trường & Công cụ

- Sử dụng các câu lệnh của Windows.
- Không import các package built-in.
- Không tự động build project.
- Sử dụng parquet-go/parquet-go, không dùng xitongsys/parquet-go.
- Sử dụng memory-cache MCP server khi đọc file.
- Sử dụng context7 MCP server khi tìm tài liệu.