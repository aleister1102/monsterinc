package datastore

import (
	"encoding/json"
	"time"

	httpx "github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// RecordTransformer handles transformation of records
type RecordTransformer struct {
	logger zerolog.Logger
}

// NewRecordTransformer creates a new RecordTransformer
func NewRecordTransformer(logger zerolog.Logger) *RecordTransformer {
	return &RecordTransformer{
		logger: logger.With().Str("component", "RecordTransformer").Logger(),
	}
}

// TransformToParquetResult converts a models.ProbeResult to a models.ParquetProbeResult
func (rt *RecordTransformer) TransformToParquetResult(pr httpx.ProbeResult, scanTime time.Time, scanSessionID string) models.ParquetProbeResult {
	headersJSON := rt.marshalHeaders(pr.Headers, pr.InputURL)
	techNames := rt.extractTechnologyNames(pr.Technologies)
	firstSeen := rt.determineFirstSeenTimestamp(pr.FirstSeenTimestamp, scanTime)

	return models.ParquetProbeResult{
		OriginalURL:   pr.InputURL,
		FinalURL:      StringPtrOrNil(pr.FinalURL),
		StatusCode:    Int32PtrOrNilZero(int32(pr.StatusCode)),
		ContentLength: Int64PtrOrNilZero(pr.ContentLength),
		ContentType:   StringPtrOrNil(pr.ContentType),
		Title:         StringPtrOrNil(pr.Title),
		WebServer:     StringPtrOrNil(pr.WebServer),
		Technologies:  techNames,
		IPAddress:     pr.IPs,
		RootTargetURL: StringPtrOrNil(pr.RootTargetURL),
		ProbeError:    StringPtrOrNil(pr.Error),
		Method:        StringPtrOrNil(pr.Method),
		HeadersJSON:   headersJSON,

		DiffStatus:         StringPtrOrNil(pr.URLStatus),
		ScanSessionID:      StringPtrOrNil(scanSessionID),
		ScanTimestamp:      scanTime.UnixMilli(),
		FirstSeenTimestamp: models.TimePtrToUnixMilliOptional(firstSeen),
		LastSeenTimestamp:  models.TimePtrToUnixMilliOptional(scanTime),
	}
}

// marshalHeaders converts headers map to JSON string pointer
func (rt *RecordTransformer) marshalHeaders(headers map[string]string, inputURL string) *string {
	if len(headers) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(headers)
	if err != nil {
		rt.logger.Error().Err(err).Str("url", inputURL).Msg("Failed to marshal headers")
		return nil
	}

	strData := string(jsonData)
	return &strData
}

// extractTechnologyNames extracts technology names from Technology slice
func (rt *RecordTransformer) extractTechnologyNames(technologies []httpx.Technology) []string {
	var techNames []string
	for _, tech := range technologies {
		techNames = append(techNames, tech.Name)
	}
	return techNames
}

// determineFirstSeenTimestamp determines the first seen timestamp
func (rt *RecordTransformer) determineFirstSeenTimestamp(oldestScanTimestamp time.Time, scanTime time.Time) time.Time {
	if oldestScanTimestamp.IsZero() {
		return scanTime
	}
	return oldestScanTimestamp
}
