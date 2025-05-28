# PRD: Discord Bot Integration for MonsterInc

## Introduction/Overview

This feature introduces a Discord Bot that provides remote management capabilities for the MonsterInc application. The bot allows users to manage target URLs, trigger scans, and monitor the application status directly from Discord. Additionally, a watchdog script ensures the MonsterInc service remains operational.

The Discord Bot solves the problem of manual file editing and provides a centralized interface for managing MonsterInc operations, making it easier for teams to collaborate and monitor scanning activities.

## Goals

1. Enable remote management of target URLs through Discord slash commands
2. Provide real-time status monitoring and scan triggering capabilities
3. Implement automated service monitoring through a watchdog script
4. Integrate scan results notification with existing Discord webhook system
5. Ensure secure and rate-limited access to bot functionalities

## User Stories

1. **As a security analyst**, I want to add new target URLs to the scan list via Discord so that I can quickly include new discovered assets without accessing the server directly.

2. **As a team lead**, I want to view the current list of monitored targets so that I can review what assets are being scanned.

3. **As a security engineer**, I want to remove outdated or invalid URLs from the target list so that scan resources are not wasted on irrelevant targets.

4. **As an operations team member**, I want to trigger an immediate scan when new critical assets are discovered so that security assessment can be performed without waiting for the scheduled scan.

5. **As a system administrator**, I want to receive notifications when the MonsterInc service goes down so that I can ensure continuous monitoring coverage.

6. **As a security analyst**, I want to add JavaScript and HTML file URLs for content monitoring so that I can track changes in critical web assets.

## Functional Requirements

### Discord Bot Core Features

1. **FR-1**: The system must implement a self-hosted Discord Bot using Discord.py library
2. **FR-2**: The bot must support slash commands for all user interactions
3. **FR-3**: The bot must implement rate limiting to prevent command spam
4. **FR-4**: The bot must provide confirmation messages for all destructive operations (add/delete/update)
5. **FR-5**: The bot must log all user interactions and command executions

### Target Management Commands

6. **FR-6**: The bot must provide `/add-target <url>` command to add URLs to `targets/urls.txt`
7. **FR-7**: The bot must provide `/add-js-html <url>` command to add URLs to `targets/js_html.txt`
8. **FR-8**: The bot must provide `/list-targets` command to display all URLs from `targets/urls.txt`
9. **FR-9**: The bot must provide `/list-js-html` command to display all URLs from `targets/js_html.txt`
10. **FR-10**: The bot must provide `/remove-target <line_number>` command to remove specific lines from `targets/urls.txt`
11. **FR-11**: The bot must provide `/remove-js-html <line_number>` command to remove specific lines from `targets/js_html.txt`
12. **FR-12**: The bot must provide `/update-target <line_number> <new_url>` command to modify existing entries in `targets/urls.txt`
13. **FR-13**: The bot must provide `/update-js-html <line_number> <new_url>` command to modify existing entries in `targets/js_html.txt`

### File Management

14. **FR-14**: The system must create `targets/` directory if it doesn't exist
15. **FR-15**: The system must create `targets/urls.txt` and `targets/js_html.txt` files if they don't exist
16. **FR-16**: The bot must handle file locking to prevent concurrent write operations
17. **FR-17**: The bot must report file operation errors to the user via Discord

### Scan Integration

18. **FR-18**: The bot must provide `/trigger-scan` command to execute onetime scans
19. **FR-19**: The bot must check if MonsterInc service is running before triggering scans
20. **FR-20**: The bot must execute the command: `./monsterinc -mode onetime -uf ./targets/urls.txt`
21. **FR-21**: The bot must report scan execution status (started/failed) to Discord
22. **FR-22**: The system must integrate with existing Discord webhook notifications for scan results

### Service Monitoring

23. **FR-23**: The bot must provide `/status` command to check MonsterInc service health
24. **FR-24**: The bot must provide `/restart-service` command to restart MonsterInc service (if watchdog is running)

### Watchdog Script

25. **FR-25**: The system must include a bash watchdog script that monitors MonsterInc service
26. **FR-26**: The watchdog must check service health every 60 minutes
27. **FR-27**: The watchdog must restart the service if it's not running
28. **FR-28**: The watchdog must log all activities to `~/logs/monsterinc-watchdog.log`
29. **FR-29**: The watchdog must send Discord notifications when service restarts occur
30. **FR-30**: The watchdog must accept MonsterInc command arguments as script parameters

## Non-Goals (Out of Scope)

1. **NG-1**: Advanced user authentication beyond Discord server membership
2. **NG-2**: File backup and versioning system
3. **NG-3**: URL validation and domain restrictions
4. **NG-4**: Queue system for multiple concurrent scan requests
5. **NG-5**: Retry mechanisms for failed operations
6. **NG-6**: Web interface or GUI for bot management
7. **NG-7**: Integration with other chat platforms (Slack, Teams, etc.)
8. **NG-8**: Advanced scheduling features beyond immediate scan triggering

## Technical Considerations

1. **TC-1**: Use Discord.py library for bot implementation
2. **TC-2**: Implement file operations with proper locking mechanisms
3. **TC-3**: Use subprocess module for executing MonsterInc commands
4. **TC-4**: Store bot configuration in YAML format similar to MonsterInc config
5. **TC-5**: Implement graceful error handling for all file and process operations
6. **TC-6**: Use systemd or similar service manager for watchdog script deployment
7. **TC-7**: Ensure bot token security through environment variables or secure config files

## Success Metrics

1. **SM-1**: Bot successfully handles 100% of target management operations without data loss
2. **SM-2**: Scan triggering success rate > 95% when service is healthy
3. **SM-3**: Watchdog detects and restarts failed service within 60 minutes
4. **SM-4**: Zero unauthorized access to bot commands
5. **SM-5**: All bot operations complete within 10 seconds response time
6. **SM-6**: Bot uptime > 99.5% during operational periods

## Open Questions

1. **OQ-1**: Should the bot support bulk operations (adding multiple URLs at once)?
2. **OQ-2**: Do we need role-based permissions for different bot commands?
3. **OQ-3**: Should the watchdog script support multiple MonsterInc instances?
4. **OQ-4**: Do we need a command to view recent scan history/logs?
5. **OQ-5**: Should the bot support configuration management (viewing/updating MonsterInc config)?
6. **OQ-6**: Do we need integration with existing MonsterInc logging system?
7. **OQ-7**: Should the bot support scheduled scan management (not just onetime scans)? 