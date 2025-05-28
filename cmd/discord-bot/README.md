# MonsterInc Discord Bot

Discord bot để quản lý MonsterInc service và target files thông qua slash commands.

## Features

- **Target File Management**: Add, list, remove, update URLs trong `targets/urls.txt` và `targets/js_html.txt`
- **Service Monitoring**: Check status và trigger one-time scans của MonsterInc service
- **Rate Limiting**: Bảo vệ khỏi command spam
- **Cross-platform**: Hỗ trợ Windows và Unix systems
- **Logging**: Tích hợp với MonsterInc logging system

## Setup

### 1. Build Discord Bot

```bash
cd cmd/discord-bot
go mod tidy
go build -o discord-bot
```

### 2. Configuration

Tạo Discord bot token và guild ID:

1. Tạo Discord application tại https://discord.com/developers/applications
2. Tạo bot và copy token
3. Invite bot vào server với permissions: `applications.commands`, `Send Messages`
4. Copy Guild ID từ Discord server

Cập nhật `configs/discord-bot-config.yaml`:

```yaml
discord:
  token: ""  # Set via DISCORD_BOT_TOKEN environment variable
  guild_id: "YOUR_GUILD_ID"
  webhook_url: ""  # Optional: for notifications

paths:
  targets_dir: "../../targets"
  urls_file: "urls.txt"
  js_html_file: "js_html.txt"
  monsterinc_bin: "../../monsterinc"
```

### 3. Environment Variables

```bash
export DISCORD_BOT_TOKEN="your_bot_token_here"
export DISCORD_WEBHOOK_URL="your_webhook_url_here"  # Optional
```

### 4. Run Discord Bot

```bash
./discord-bot
```

## Slash Commands

### Target Management

- `/add-url <url>` - Add URL to urls.txt
- `/add-js-html <url>` - Add URL to js_html.txt
- `/list-urls [page]` - List URLs from urls.txt (paginated)
- `/list-js-html [page]` - List URLs from js_html.txt (paginated)
- `/remove-url <line>` - Remove URL by line number from urls.txt
- `/remove-js-html <line>` - Remove URL by line number from js_html.txt
- `/update-url <line> <url>` - Update URL at line number in urls.txt
- `/update-js-html <line> <url>` - Update URL at line number in js_html.txt

### Service Management

- `/status` - Check MonsterInc service status
- `/scan-onetime` - Trigger one-time scan using urls.txt

## Watchdog Script

### Setup Watchdog

```bash
# Make script executable
chmod +x scripts/monsterinc-watchdog.sh

# Test watchdog
./scripts/monsterinc-watchdog.sh status
```

### Environment Variables for Watchdog

```bash
export MONSTERINC_BIN="/path/to/monsterinc"
export MONSTERINC_ARGS="-mode scheduler"
export CHECK_INTERVAL="3600"  # 60 minutes
export LOG_DIR="$HOME/logs"
export DISCORD_WEBHOOK_URL="your_webhook_url"
export MAX_RESTART_ATTEMPTS="3"
export RESTART_DELAY="30"
```

### Watchdog Commands

```bash
# Start MonsterInc service
./scripts/monsterinc-watchdog.sh start

# Stop MonsterInc service
./scripts/monsterinc-watchdog.sh stop

# Restart MonsterInc service
./scripts/monsterinc-watchdog.sh restart

# Check service status
./scripts/monsterinc-watchdog.sh status

# Start monitoring (default)
./scripts/monsterinc-watchdog.sh monitor
```

### Deploy as Systemd Service

```bash
# Copy service file
sudo cp scripts/monsterinc-watchdog.service /etc/systemd/system/

# Edit service file with correct paths
sudo nano /etc/systemd/system/monsterinc-watchdog.service

# Create user and directories
sudo useradd -r -s /bin/false monsterinc
sudo mkdir -p /opt/monsterinc /var/log/monsterinc
sudo chown monsterinc:monsterinc /var/log/monsterinc

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable monsterinc-watchdog
sudo systemctl start monsterinc-watchdog

# Check status
sudo systemctl status monsterinc-watchdog
```

## File Structure

```
cmd/discord-bot/
├── main.go              # Application entry point
├── bot.go               # Core bot implementation
├── config.go            # Configuration management
├── file_manager.go      # Target file operations
├── service_monitor.go   # Service monitoring
├── go.mod              # Go module
└── README.md           # This file

configs/
└── discord-bot-config.yaml  # Bot configuration

scripts/
├── monsterinc-watchdog.sh      # Watchdog script
└── monsterinc-watchdog.service # Systemd service file
```

## Security Notes

- Bot token phải được bảo vệ và không commit vào git
- Sử dụng environment variables cho sensitive data
- Rate limiting được implement để tránh abuse
- File operations sử dụng proper locking mechanisms

## Troubleshooting

### Bot không start

1. Check Discord token và guild ID
2. Verify bot permissions trong Discord server
3. Check logs cho error messages

### Commands không hoạt động

1. Verify bot có permission `applications.commands`
2. Check rate limiting
3. Verify target files và MonsterInc binary paths

### Watchdog không restart service

1. Check process permissions
2. Verify MonsterInc binary path
3. Check logs tại `$LOG_DIR/monsterinc-watchdog.log`

## Development

### Adding New Commands

1. Add command definition trong `commands` slice
2. Add handler case trong `handleCommands`
3. Implement handler method
4. Test với Discord

### Extending Service Monitoring

1. Add methods trong `ServiceMonitor` struct
2. Implement platform-specific logic
3. Add Discord command handlers
4. Update documentation 