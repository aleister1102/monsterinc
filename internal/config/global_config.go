package config

import (
	"encoding/json" // Thay đổi import từ yaml sang json
	"fmt"
	"os"
)

// GlobalConfig là struct chứa tất cả cấu hình global của ứng dụng.
// Nó có thể bao gồm cấu hình cho nhiều module khác nhau.
type GlobalConfig struct {
	HTTPXSettings   HTTPXConfig   `json:"httpx_settings"`   // Thay đổi tag từ yaml sang json
	CrawlerSettings CrawlerConfig `json:"crawler_settings"` // Thay đổi tag từ yaml sang json
	// Thêm các mục cấu hình global khác ở đây nếu cần
}

// NewDefaultGlobalConfig tạo một GlobalConfig với các giá trị mặc định
// bằng cách gọi các hàm tạo mặc định của từng module con.
func NewDefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		HTTPXSettings:   *NewHTTPXConfig(),
		CrawlerSettings: *NewDefaultCrawlerConfig(),
	}
}

// LoadGlobalConfig tải cấu hình global từ một file JSON.
// Nó bắt đầu với các giá trị mặc định và sau đó ghi đè bằng các giá trị từ file.
func LoadGlobalConfig(filePath string) (*GlobalConfig, error) {
	// Bắt đầu với cấu hình mặc định
	gCfg := NewDefaultGlobalConfig()

	// Đọc nội dung file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read global config file '%s': %w", filePath, err)
	}

	// Giải mã JSON vào struct GlobalConfig
	if err := json.Unmarshal(data, gCfg); err != nil { // Thay đổi từ yaml.Unmarshal sang json.Unmarshal
		return nil, fmt.Errorf("failed to unmarshal global config data from '%s': %w", filePath, err)
	}

	// (Tùy chọn) Thực hiện validation cho từng phần của config sau khi load
	// HTTPXSettings.Validate() có thể cần được cập nhật nếu nó không xử lý việc targets rỗng khi không có file config
	// if len(gCfg.HTTPXSettings.Targets) > 0 { // Chỉ validate nếu có targets, tránh lỗi khi dùng default empty
	// 	if err := gCfg.HTTPXSettings.Validate(); err != nil {
	// 		return nil, fmt.Errorf("validation failed for httpx_settings in '%s': %w", filePath, err)
	// 	}
	// }

	// Thêm validation cho CrawlerSettings nếu có hàm Validate() tương ứng
	// Ví dụ:
	// if err := gCfg.CrawlerSettings.Validate(); err != nil {
	//   return nil, fmt.Errorf("validation failed for crawler_settings in '%s': %w", filePath, err)
	// }

	return gCfg, nil
}
