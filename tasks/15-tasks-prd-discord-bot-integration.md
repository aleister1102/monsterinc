## Relevant Files

- `cmd/discord-bot/main.go` - Main Discord bot application entry point
- `cmd/discord-bot/bot.go` - Core bot implementation with slash commands
- `cmd/discord-bot/config.go` - Bot configuration management
- `cmd/discord-bot/file_manager.go` - Target file management operations
- `cmd/discord-bot/service_monitor.go` - MonsterInc service health checking
- `cmd/discord-bot/go.mod` - Go module dependencies for the bot
- `cmd/discord-bot/go.sum` - Go module checksums
- `scripts/monsterinc-watchdog.sh` - Bash watchdog script for service monitoring
- `scripts/monsterinc-watchdog.service` - Systemd service file for watchdog
- `configs/discord-bot-config.yaml` - Bot configuration file
- `targets/urls.txt` - Target URLs file (created by bot if not exists)
- `targets/js_html.txt` - JS/HTML URLs file (created by bot if not exists)

### Notes

- Discord bot will be implemented in Go using discordgo library (github.com/bwmarrin/discordgo)
- File operations require proper locking mechanisms to prevent concurrent access issues
- Watchdog script should be deployed as a systemd service for automatic startup
- Bot token should be stored securely in environment variables or config file
- Use Go's built-in sync package for file locking and concurrency control
- Discord bot runs as a separate service independent from main MonsterInc application

## Tasks

- [x] 1.0 Setup Discord Bot Infrastructure
  - [x] 1.1 Create Go module for Discord bot in `cmd/discord-bot/`
  - [x] 1.2 Add discordgo dependency and other required packages
  - [x] 1.3 Create basic main.go with application entry point
  - [x] 1.4 Implement configuration structure and YAML loading
  - [x] 1.5 Setup logging system compatible with MonsterInc logging
  - [x] 1.6 Create bot initialization and connection handling
- [x] 2.0 Implement Target File Management System
  - [x] 2.1 Create file_manager.go with file locking mechanisms
  - [x] 2.2 Implement functions to create targets directory and files if not exist
  - [x] 2.3 Add functions to read URLs from targets/urls.txt and targets/js_html.txt
  - [x] 2.4 Implement functions to add URLs to target files with proper locking
  - [x] 2.5 Create functions to remove URLs by line number with validation
  - [x] 2.6 Add functions to update existing URLs by line number
  - [x] 2.7 Implement error handling and file operation logging
- [x] 3.0 Develop Discord Slash Commands
  - [x] 3.1 Implement slash command registration and handler setup
  - [x] 3.2 Create /add-url and /add-js-html commands with URL parameter
  - [x] 3.3 Implement /list-urls and /list-js-html commands with pagination
  - [x] 3.4 Create /remove-url and /remove-js-html commands with line number parameter
  - [x] 3.5 Implement /update-url and /update-js-html commands with line number and URL parameters
  - [x] 3.6 Add confirmation messages for destructive operations (remove/update)
  - [x] 3.7 Implement rate limiting to prevent command spam
  - [x] 3.8 Add command logging and error response handling
- [x] 4.0 Integrate MonsterInc Service Monitoring
  - [x] 4.1 Create service_monitor.go with process checking functions
  - [x] 4.2 Implement /status command to check MonsterInc service health
  - [x] 4.3 Create /scan-onetime command to execute onetime scans
  - [x] 4.4 Add process execution with proper error handling and timeout
  - [x] 4.5 Implement scan status reporting back to Discord
  - [x] 4.6 Add cross-platform process monitoring (Windows/Unix)
  - [x] 4.7 Integrate service monitoring with Discord bot commands
- [x] 5.0 Create Watchdog Script and Service Management
  - [x] 5.1 Create monsterinc-watchdog.sh bash script with service monitoring
  - [x] 5.2 Implement 60-minute interval health checking
  - [x] 5.3 Add automatic service restart functionality
  - [x] 5.4 Create logging system to ~/logs/monsterinc-watchdog.log
  - [x] 5.5 Implement Discord notifications for service restart events
  - [x] 5.6 Add command-line argument handling for MonsterInc command parameters
  - [x] 5.7 Create systemd service file for watchdog deployment 