package httpxrunner

// Config holds the configuration for the httpx runner
type Config struct {
	CustomHeaders        map[string]string
	ExtractASN           bool
	ExtractBody          bool
	ExtractCNAMEs        bool
	ExtractContentLength bool
	ExtractContentType   bool
	ExtractHeaders       bool
	ExtractIPs           bool
	ExtractLocation      bool
	ExtractServerHeader  bool
	ExtractStatusCode    bool
	ExtractTitle         bool
	FollowRedirects      bool
	Method               string
	RateLimit            int
	RequestURIs          []string
	Retries              int
	Targets              []string
	TechDetect           bool
	Threads              int
	Timeout              int // In seconds
	Verbose              bool
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		CustomHeaders:        make(map[string]string),
		ExtractASN:           true,
		ExtractBody:          false,
		ExtractCNAMEs:        true,
		ExtractContentLength: true,
		ExtractContentType:   true,
		ExtractHeaders:       true,
		ExtractIPs:           true,
		ExtractLocation:      true,
		ExtractServerHeader:  true,
		ExtractStatusCode:    true,
		ExtractTitle:         true,
		FollowRedirects:      true,
		Method:               "GET",
		RateLimit:            0,
		RequestURIs:          []string{},
		Retries:              1,
		Targets:              []string{},
		TechDetect:           true,
		Threads:              25,
		Timeout:              10,
		Verbose:              false,
	}
}
