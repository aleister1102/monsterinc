# MonsterInc

A Go-based tool for web asset discovery and monitoring.

## Features

### Target Group

- **Target Management**: Ingests a list of target URLs from a text file and normalizes them.
- **Target Crawling**: Utilizes [hakrawler](https://github.com/hakluke/hakrawler) to crawl targets.
- **HTTP(S) Probing**: Sends requests to URLs found by hakrawler using [httpx](https://github.com/projectdiscovery/httpx).
- **Reporting**: Generates searchable HTML reports with sortable fields and pagination for httpx results. Each scan produces a new report.
- **httpx Options**: `-sc -cl -ct -title -server -td -ip -cname -t 40 -fr -nc` (integrated as a library).
- **Storage**: Uses Parquet for efficient data storage.
- **Result Differencing**: Compares new crawl results with existing ones, marking old URLs and adding new ones.
- **Configuration**: Reads settings from a JSON configuration file.
- **Notifications**: Sends requests to a Discord webhook, with configurable concurrency, delay, user-agents, and excluded httpx extensions.
- **Logging**: Implements a logging mechanism (not stored in Parquet).

### HTML/JS Group

- **HTML/JS Monitoring**: Accepts paths to HTML/JS files and schedules daily checks to fetch their content.
- **Content Differencing**: Compares file content between scans and generates an HTML report showing differences in a diff view.
- **Path Extraction**: Uses [jsluice](https://github.com/BishopFox/jsluice) to find paths within HTML/JS content.
- **Secret Detection**: Employs [trufflehog](https://github.com/trufflesecurity/trufflehog) and regexes from [mantra](https://github.com/brosck/mantra) to find secrets in HTML/JS files.

## Project Structure

```
monsterinc/
├── cmd/
│   └── monsterinc/
│       └── main.go
├── internal/
│   ├── config/         // Configuration handling (JSON)
│   ├── core/           // Core logic, orchestration
│   ├── crawler/        // hakrawler integration
│   ├── datastore/      // Parquet storage logic
│   ├── htmljs/         // HTML/JS monitoring, jsluice, trufflehog
│   ├── httpxrunner/    // httpx integration
│   ├── models/         // Data structures (Target, ScanResult, etc.)
│   ├── normalizer/     // URL normalization
│   ├── notifier/       // Discord notifications
│   ├── reporter/       // HTML report generation
│   └── utils/          // Common utility functions
├── pkg/                // Libraries (if any)
├── go.mod
├── go.sum
└── README.md
``` 