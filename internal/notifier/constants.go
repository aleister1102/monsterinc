package notifier

// Discord formatting constants
const (
	DiscordUsername         = "MonsterInc Security Scanner"
	DiscordAvatarURL        = "http://insomnia1102.online:1337/favicon.ico"
	DefaultEmbedColor       = 0x2B2D31 // Discord dark theme color
	SuccessEmbedColor       = 0x5CB85C // Bootstrap success green
	ErrorEmbedColor         = 0xD9534F // Bootstrap danger red
	WarningEmbedColor       = 0xF0AD4E // Bootstrap warning orange
	InfoEmbedColor          = 0x5BC0DE // Bootstrap info blue
	MonitorEmbedColor       = 0x6F42C1 // Purple for monitoring
	InterruptEmbedColor     = 0xFD7E14 // Orange for interruptions
	CriticalErrorEmbedColor = 0xDC3545 // Red for critical errors
)

// Error formatting constants
const (
	MaxErrorTextLength         = 800 // Giảm từ 1000 xuống 800
	MaxCriticalErrorTextLength = 600 // Giảm từ 1500 xuống 600
	MaxSingleErrorLength       = 150 // Giới hạn cho mỗi error riêng lẻ
	MaxErrorSampleCount        = 3   // Giảm từ 5 xuống 3
)
