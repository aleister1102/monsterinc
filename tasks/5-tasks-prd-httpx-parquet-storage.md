## Relevant Files

- `internal/datastore/parquet_writer.go` - Core logic for writing probe results to Parquet files.
- `internal/datastore/parquet_reader.go` - Core logic for reading probe results from Parquet files.
- `internal/models/parquet_schema.go` - Defines the `ParquetProbeResult` struct which dictates the Parquet schema.
- `internal/models/probe_result.go` - Defines the `ProbeResult` struct (source data).
- `internal/orchestrator/orchestrator.go` - Uses `ParquetWriter` to store results and `ParquetReader` (via `UrlDiffer`) to get historical data.
- `internal/config/config.go` - Contains `StorageConfig`.
- `cmd/monsterinc/main.go` - Initializes `ParquetWriter` and `ParquetReader`.
- `internal/scheduler/scheduler.go` - Initializes `ParquetWriter` and `ParquetReader` for automated mode.

### Notes

- The primary goal is to move from one Parquet file per scan ID per target to **one consolidated Parquet file per target**, which is updated/overwritten with each new scan.
- This single file (e.g., `database/example.com/data.parquet`) will contain all known probe results for that target, with timestamps indicating first seen, last seen, and last scan.

## Tasks

- [x] 1.0. Parquet Writer (`internal/datastore/parquet_writer.go`)
  - [x] 1.1. Modify `NewParquetWriter` to accept a `*datastore.ParquetReader` argument. (REVERTED - No longer needs ParquetReader for internal merge)
  - [x] 1.2. Change file path logic in `Write` method:
        *   [x] Path should point to a single `data.parquet` file under a directory named after the sanitized `rootTarget` (e.g., `database/example.com/data.parquet`).
        *   [x] Remove `scanSessionID` from the directory path components.
  - [-] 1.3. Implement `mergeProbeResults(currentProbes []models.ProbeResult, historicalProbes []models.ProbeResult, currentScanTime time.Time) []models.ProbeResult` function: (REMOVED/COMMENTED - Writer no longer merges internally)
        *   [-] Takes current scan probes and historical probes (read from the existing `data.parquet`).
        *   [-] Returns a single, merged slice of `models.ProbeResult`.
        *   [-] **Update existing records**: If a URL from `currentProbes` matches one in `historicalProbes` (by `InputURL`):
            *   [-] Preserve `OldestScanTimestamp` (FirstSeen) from the historical record.
            *   [-] Update `Timestamp` (LastSeen/ScanTimestamp) to `currentScanTime`.
            *   [-] Update all other probe data fields (StatusCode, Title, Technologies, etc.) from the `currentProbes` entry.
        *   [-] **Add new records**: If a URL from `currentProbes` is not in `historicalProbes`:
            *   [-] Set both `OldestScanTimestamp` and `Timestamp` to `currentScanTime`.
        *   [-] **Handle old records**: URLs in `historicalProbes` but not in `currentProbes` should be included in the merged result to preserve their history (their `URLStatus` will be marked "old" by the differ later).
  - [x] 1.4. Modify `transformToParquetResult`:
        *   [x] Ensure `ParquetProbeResult.FirstSeenTimestamp` is populated from `ProbeResult.OldestScanTimestamp` (which should be set by merge logic or to current scan time if new).
        *   [x] Ensure `ParquetProbeResult.LastSeenTimestamp` is populated from the current scan's time (`ProbeResult.Timestamp`).
        *   [x] `ParquetProbeResult.ScanTimestamp` should also be the current scan's time.
  - [x] 1.5. Update `Write` method logic:
        *   [-] Call `pw.parquetReader.FindAllProbeResultsForTarget(rootTarget)` to get historical data. (REMOVED - Writer no longer reads internally)
        *   [-] Call `pw.mergeProbeResults` with current and historical data. (REMOVED - Writer no longer merges internally)
        *   [x] Write the resulting merged data (or just current data if not merging) to the single Parquet file for the target, **overwriting** it.
- [x] 2.0. Parquet Reader (`internal/datastore/parquet_reader.go`)
  - [x] 2.1. Rename `FindHistoricalDataForTarget` to `FindAllProbeResultsForTarget(rootTargetURL string) ([]models.ProbeResult, time.Time, error)`.
  - [x] 2.2. Modify `FindAllProbeResultsForTarget` logic:
        *   [x] It should now read the single consolidated `data.parquet` file for the given `rootTargetURL` (e.g., `database/example.com/data.parquet`).
        *   [x] Return all `models.ProbeResult` records from this file and the file's last modification time.
        *   [x] If the file doesn't exist, return `nil` for results and `time.Time{}` for modTime, and no error (this is not an error condition, just means no history).
  - [x] 2.3. Remove `FindMostRecentScanURLs` method as it's no longer applicable.
  - [x] 2.4. Keep `readProbeResultsFromSpecificFile` as is, it's a useful helper.
- [x] 3.0. Update Calling Code
  - [x] 3.1. `internal/differ/url_differ.go`:
        *   [x] Update `Compare` method to call `parquetReader.FindAllProbeResultsForTarget(rootTarget)` to get historical data.
        *   [x] Adjust logic to correctly diff against the full historical dataset returned.
  - [x] 3.2. `cmd/monsterinc/main.go` (for `runOnetimeScan`):
        *   [x] Update `NewParquetWriter` call if its signature changed (e.g., to include `ParquetReader` - REVERTED, no longer needs it).
  - [x] 3.3. `internal/scheduler/scheduler.go` (for `NewScheduler`):
        *   [x] Update `NewParquetWriter` call if its signature changed (e.g., to include `ParquetReader` - REVERTED, no longer needs it).
- [x] 4.0. Documentation
  - [x] 4.1. Update `internal/datastore/README.md` to reflect the new single-file-per-target storage logic, the merge process in the writer, and the updated reader methods.
  - [x] 4.2. Update `internal/differ/README.md` if `Compare` logic or data fetching changes significantly.
  - [x] 4.3. Update `internal/orchestrator/README.md` to detail how it uses the modified datastore components for historical data and writing new data.

- [ ] 5.0. Testing (SKIPPED)
  - [ ] 5.1. Write/update unit tests for `ParquetWriter` focusing on the merge logic and file overwriting.
  - [ ] 5.2. Write/update unit tests for `ParquetReader` focusing on reading the consolidated file. 