package httpxrunner

import (
	"strconv"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
)

// ProbeResultMapper handles mapping from httpx results to ProbeResult
type ProbeResultMapper struct {
	logger zerolog.Logger
}

// NewProbeResultMapper creates a new probe result mapper
func NewProbeResultMapper(logger zerolog.Logger) *ProbeResultMapper {
	return &ProbeResultMapper{
		logger: logger.With().Str("component", "ProbeResultMapper").Logger(),
	}
}

// MapResult converts an httpx runner.Result to a models.ProbeResult
func (prm *ProbeResultMapper) MapResult(res runner.Result, rootURL string) *models.ProbeResult {
	probeResult := prm.createBaseProbeResult(res, rootURL)

	prm.mapDuration(probeResult, res)
	prm.mapHeaders(probeResult, res)
	prm.mapTechnologies(probeResult, res)
	prm.mapNetworkInfo(probeResult, res)
	prm.mapASNInfo(probeResult, res)

	return probeResult
}

// createBaseProbeResult creates the basic probe result structure
func (prm *ProbeResultMapper) createBaseProbeResult(res runner.Result, rootURL string) *models.ProbeResult {
	return &models.ProbeResult{
		Body:          res.ResponseBody,
		ContentLength: int64(res.ContentLength),
		ContentType:   res.ContentType,
		Error:         res.Error,
		FinalURL:      res.URL,
		InputURL:      res.Input,
		Method:        res.Method,
		RootTargetURL: rootURL,
		StatusCode:    res.StatusCode,
		Timestamp:     res.Timestamp,
		Title:         res.Title,
		WebServer:     res.WebServer,
	}
}

// mapDuration maps response time to duration
func (prm *ProbeResultMapper) mapDuration(probeResult *models.ProbeResult, res runner.Result) {
	if res.ResponseTime == "" {
		return
	}

	// Try parsing as Go duration format first (e.g., "96.0199m", "30s", "1.5h")
	if dur, err := time.ParseDuration(res.ResponseTime); err == nil {
		probeResult.Duration = dur.Seconds()
		return
	}

	// Fallback: try the old method for backward compatibility
	durationStr := strings.TrimSuffix(res.ResponseTime, "s")
	if dur, err := strconv.ParseFloat(durationStr, 64); err == nil {
		probeResult.Duration = dur
	} else {
		prm.logger.Debug().
			Str("response_time", res.ResponseTime).
			Err(err).
			Msg("Failed to parse response time")
	}
}

// mapHeaders maps response headers
func (prm *ProbeResultMapper) mapHeaders(probeResult *models.ProbeResult, res runner.Result) {
	if len(res.ResponseHeaders) == 0 {
		return
	}

	probeResult.Headers = make(map[string]string)
	for k, v := range res.ResponseHeaders {
		probeResult.Headers[k] = prm.convertHeaderValue(v, k)
	}
}

// convertHeaderValue converts header value from interface{} to string
func (prm *ProbeResultMapper) convertHeaderValue(v interface{}, headerKey string) string {
	switch val := v.(type) {
	case string:
		return val
	case []string:
		return strings.Join(val, ", ")
	case []interface{}:
		return prm.convertInterfaceSliceToString(val)
	default:
		prm.logger.Debug().
			Str("header_key", headerKey).
			Interface("value", v).
			Msg("Unknown header value type")
		return ""
	}
}

// convertInterfaceSliceToString converts []interface{} to comma-separated string
func (prm *ProbeResultMapper) convertInterfaceSliceToString(val []interface{}) string {
	var strVals []string
	for _, iv := range val {
		if sv, ok := iv.(string); ok {
			strVals = append(strVals, sv)
		}
	}
	return strings.Join(strVals, ", ")
}

// mapTechnologies maps detected technologies
func (prm *ProbeResultMapper) mapTechnologies(probeResult *models.ProbeResult, res runner.Result) {
	if len(res.Technologies) == 0 {
		return
	}

	probeResult.Technologies = make([]models.Technology, 0, len(res.Technologies))
	for _, techName := range res.Technologies {
		tech := models.Technology{Name: techName}
		probeResult.Technologies = append(probeResult.Technologies, tech)
	}
}

// mapNetworkInfo maps network information
func (prm *ProbeResultMapper) mapNetworkInfo(probeResult *models.ProbeResult, res runner.Result) {
	if len(res.A) > 0 {
		probeResult.IPs = res.A
	}
	// CNAMEs mapping can be added here if needed
}

// mapASNInfo maps ASN information
func (prm *ProbeResultMapper) mapASNInfo(probeResult *models.ProbeResult, res runner.Result) {
	if res.ASN == nil {
		return
	}

	if res.ASN.AsNumber != "" {
		if asnNumber, err := prm.parseASNNumber(res.ASN.AsNumber); err == nil {
			probeResult.ASN = asnNumber
		}
	}

	probeResult.ASNOrg = res.ASN.AsName
}

// parseASNNumber parses ASN number from string
func (prm *ProbeResultMapper) parseASNNumber(asNumber string) (int, error) {
	cleanNumber := strings.ReplaceAll(asNumber, "AS", "")
	return strconv.Atoi(cleanNumber)
}
