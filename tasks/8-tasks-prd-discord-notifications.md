## Relevant Files

- `internal/notifier/discord.go` - Core Discord notification logic.
- `internal/notifier/embed.go` - Discord embed message formatting.
- `internal/notifier/splitter.go` - Message splitting for large notifications.
- `internal/config/config.go` - Configuration for Discord settings.
- `internal/logger/logger.go` - Logging for notification events.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [ ] 1.0 Implement Discord Notification Core
  - [ ] 1.1 Implement webhook client with retry logic in `internal/notifier/discord.go`.
  - [ ] 1.2 Implement notification sending logic in `internal/notifier/discord.go`.
  - [ ] 1.3 Implement error handling for webhook failures in `internal/notifier/discord.go`.
- [ ] 2.0 Implement Message Formatting
  - [ ] 2.1 Implement scan completion message template in `internal/notifier/embed.go`.
  - [ ] 2.2 Implement critical error message template in `internal/notifier/embed.go`.
  - [ ] 2.3 Implement markdown formatting helpers in `internal/notifier/embed.go`.
- [ ] 3.0 Implement Message Splitting
  - [ ] 3.1 Implement message size calculation in `internal/notifier/splitter.go`.
  - [ ] 3.2 Implement logic to split large messages in `internal/notifier/splitter.go`.
  - [ ] 3.3 Implement rate limiting for multiple messages in `internal/notifier/splitter.go`.
- [ ] 4.0 Implement Configuration Management
  - [ ] 4.1 Define Discord settings in configuration structure in `internal/config/config.go`.
  - [ ] 4.2 Implement configuration loading for Discord settings in `internal/config/config.go`.
- [ ] 5.0 Implement Error Handling and Logging
  - [ ] 5.1 Implement error handling for webhook configuration in `internal/notifier/discord.go`.
  - [ ] 5.2 Implement error handling for message formatting in `internal/notifier/embed.go`.
  - [ ] 5.3 Implement error handling for message splitting in `internal/notifier/splitter.go`.
  - [ ] 5.4 Implement logging for notification events in all relevant files. 