package config

const (
	// HTTPXRunner Defaults
	DefaultHTTPXThreads              = 25
	DefaultHTTPXTimeoutSecs          = 10
	DefaultHTTPXRetries              = 1
	DefaultHTTPXFollowRedirects      = true
	DefaultHTTPXMaxRedirects         = 10
	DefaultHTTPXVerbose              = false
	DefaultHTTPXMethod               = "GET"
	DefaultHTTPXTechDetect           = true
	DefaultHTTPXExtractTitle         = true
	DefaultHTTPXExtractStatusCode    = true
	DefaultHTTPXExtractLocation      = true
	DefaultHTTPXExtractContentLength = true
	DefaultHTTPXExtractServerHeader  = true
	DefaultHTTPXExtractContentType   = true
	DefaultHTTPXExtractIPs           = true
	DefaultHTTPXExtractBody          = false
	DefaultHTTPXExtractHeaders       = true
	DefaultHTTPXRateLimit            = 0
	DefaultHTTPXExtractASN           = true
)

type HttpxRunnerConfig struct {
	CustomHeaders        map[string]string `json:"custom_headers,omitempty" yaml:"custom_headers,omitempty"`
	ExtractASN           bool              `json:"extract_asn" yaml:"extract_asn"`
	ExtractBody          bool              `json:"extract_body" yaml:"extract_body"`
	ExtractContentLength bool              `json:"extract_content_length" yaml:"extract_content_length"`
	ExtractContentType   bool              `json:"extract_content_type" yaml:"extract_content_type"`
	ExtractHeaders       bool              `json:"extract_headers" yaml:"extract_headers"`
	ExtractIPs           bool              `json:"extract_ips" yaml:"extract_ips"`
	ExtractLocation      bool              `json:"extract_location" yaml:"extract_location"`
	ExtractServerHeader  bool              `json:"extract_server_header" yaml:"extract_server_header"`
	ExtractStatusCode    bool              `json:"extract_status_code" yaml:"extract_status_code"`
	ExtractTitle         bool              `json:"extract_title" yaml:"extract_title"`
	FollowRedirects      bool              `json:"follow_redirects" yaml:"follow_redirects"`
	MaxRedirects         int               `json:"max_redirects,omitempty" yaml:"max_redirects,omitempty" validate:"omitempty,min=0"`
	Method               string            `json:"method,omitempty" yaml:"method,omitempty"`
	RateLimit            int               `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty" validate:"omitempty,min=0"`
	RequestURIs          []string          `json:"request_uris,omitempty" yaml:"request_uris,omitempty" validate:"omitempty,dive,url"`
	Retries              int               `json:"retries,omitempty" yaml:"retries,omitempty" validate:"omitempty,min=0"`
	TechDetect           bool              `json:"tech_detect" yaml:"tech_detect"`
	Threads              int               `json:"threads,omitempty" yaml:"threads,omitempty" validate:"omitempty,min=1"`
	TimeoutSecs          int               `json:"timeout_secs,omitempty" yaml:"timeout_secs,omitempty" validate:"omitempty,min=1"`
	Verbose              bool              `json:"verbose" yaml:"verbose"`
}

func NewDefaultHTTPXRunnerConfig() HttpxRunnerConfig {
	return HttpxRunnerConfig{
		CustomHeaders:        make(map[string]string),
		ExtractASN:           DefaultHTTPXExtractASN,
		ExtractBody:          DefaultHTTPXExtractBody,
		ExtractContentLength: DefaultHTTPXExtractContentLength,
		ExtractContentType:   DefaultHTTPXExtractContentType,
		ExtractHeaders:       DefaultHTTPXExtractHeaders,
		ExtractIPs:           DefaultHTTPXExtractIPs,
		ExtractLocation:      DefaultHTTPXExtractLocation,
		ExtractServerHeader:  DefaultHTTPXExtractServerHeader,
		ExtractStatusCode:    DefaultHTTPXExtractStatusCode,
		ExtractTitle:         DefaultHTTPXExtractTitle,
		FollowRedirects:      DefaultHTTPXFollowRedirects,
		MaxRedirects:         DefaultHTTPXMaxRedirects,
		Method:               DefaultHTTPXMethod,
		RateLimit:            DefaultHTTPXRateLimit,
		RequestURIs:          []string{},
		Retries:              DefaultHTTPXRetries,
		TechDetect:           DefaultHTTPXTechDetect,
		Threads:              DefaultHTTPXThreads,
		TimeoutSecs:          DefaultHTTPXTimeoutSecs,
		Verbose:              DefaultHTTPXVerbose,
	}
}
