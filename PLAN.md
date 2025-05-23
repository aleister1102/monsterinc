Tôi muốn bạn dựa vào create-prd.mdc để tạo ra một công cụ viết bằng golang.

Các tính năng:
1. Nhóm target
- Quản lý target: nhận vào danh sách target (là các URL) thông qua file text và chuẩn hóa các URL này.
- Thực hiện crawl các target: sử dụng @https://github.com/hakluke/hakrawler
- Với mỗi URL mà hakrawler có được, sử dụng @https://github.com/projectdiscovery/httpx  để gửi request.
- Tạo ra báo cáo dạng HTML và có khả năng tìm kiếm dựa trên các trường của httpx, có khả năng sort theo các trường của httpx, có khả năng phân trang các kết quả của httpx. Mỗi lần scan là sẽ có một báo cáo (báo cáo có thể có nhiều target).
- Các option httpx mà tôi sẽ sử dụng là: -sc -cl -ct -title -server -td -ip -cname -t 40 -fr -nc (sử dụng các httpx như là một thư viện và gọi các option trong type tương ứng)
- Sử dụng database là parquet để giảm kích thước lưu trữ
- Cho phép lưu trữ kết quả của mỗi lần crawl cho từng target. Khi có một kết quả crawl mới. So sánh danh sách các URL đã có với danh sách các URL mới. Đánh dấu các URL đã có là cũ nếu kết quả crawl có chúng và thêm mới các URL mới (đánh dấu là mới) nếu có.
- Cho phép đọc cấu hình từ file json
- Cho phép gửi request đến webhook của discord, có khả năng specify số luồng thực thi, delay cho mỗi request, list các user-agent, các extension mà httpx sẽ exclude
- Có cơ chế ghi log (không cần ghi vào file parquet)

2. Nhóm HTML/JS
- Giám sát HTML/JS: cho phép nhập vào các đường dẫn file HTML/JS. Lập lịch để chạy tại một thời điểm được chỉ định trong ngày nhằm lấy nội dung file.
- So sánh nội dung file của lần quét trước và lần này để tìm ra sự khác nhau. Xuất báo cáo HTML thể hiện sự khác nhau bằng cách hiển thị diff view
- Sử dụng @https://github.com/BishopFox/jsluice  để tìm các đường dẫn có trong nội dung file HTML/JS
- Sử dụng @https://github.com/trufflesecurity/trufflehog và các regex có trong @https://github.com/brosck/mantra  để tìm các secrets có trong file HTML/JS