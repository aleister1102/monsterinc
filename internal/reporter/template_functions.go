package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/httpxrunner"
)

// GetCommonTemplateFunctions returns common functions for templates
func GetCommonTemplateFunctions() template.FuncMap {
	return template.FuncMap{
		"json": func(v interface{}) (template.JS, error) {
			data, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(data), nil
		},
		"jsonMarshal": func(v interface{}) template.JS {
			data, err := json.Marshal(v)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Template: jsonMarshal error: %v\n", err)
				return ""
			}
			return template.JS(data)
		},
		"ToLower": strings.ToLower,
		"title":   strings.Title,
		"joinStrings": func(s []string, sep string) string {
			return strings.Join(s, sep)
		},
		"formatTime": func(t time.Time, layout string) string {
			if t.IsZero() {
				return "N/A"
			}
			return t.Format(layout)
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"inc": func(i int) int {
			return i + 1
		},
		"slice": sliceString,
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"gt": compareGreaterThan,
	}
}

// GetDiffTemplateFunctions returns functions specific for diff templates
func GetDiffTemplateFunctions() template.FuncMap {
	funcMap := GetCommonTemplateFunctions()

	// Add functions specific for diff
	funcMap["prettyJson"] = func(b []byte) template.HTML {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, b, "", "  "); err != nil {
			return template.HTML("Error pretty printing JSON")
		}
		return template.HTML(prettyJSON.String())
	}

	funcMap["operationToString"] = func(op httpxrunner.DiffOperation) string {
		switch op {
		case httpxrunner.DiffDelete:
			return "Delete"
		case httpxrunner.DiffInsert:
			return "Insert"
		case httpxrunner.DiffEqual:
			return "Equal"
		default:
			return "Unknown"
		}
	}

	funcMap["replaceNewlinesWithBR"] = func(s string) template.HTML {
		return template.HTML(strings.ReplaceAll(s, "\n", "<br>"))
	}

	return funcMap
}

// sliceString slices string by start and end index
func sliceString(s string, start int, end ...int) string {
	if len(s) == 0 {
		return s
	}

	if start < 0 {
		start = len(s) + start
	}
	if start < 0 {
		start = 0
	}
	if start >= len(s) {
		return ""
	}

	if len(end) > 0 {
		endIdx := end[0]
		if endIdx < 0 {
			endIdx = len(s) + endIdx
		}
		if endIdx > len(s) {
			endIdx = len(s)
		}
		if endIdx <= start {
			return ""
		}
		return s[start:endIdx]
	}
	return s[start:]
}

// compareGreaterThan compares two values
func compareGreaterThan(a, b interface{}) bool {
	switch av := a.(type) {
	case int:
		if bv, ok := b.(int); ok {
			return av > bv
		}
	case string:
		if bv, ok := b.(string); ok {
			return len(av) > len(bv)
		}
	}
	return false
}

func getFuncMap() template.FuncMap {
	return template.FuncMap{
		"prettyPrintJSON": func(v interface{}) (template.HTML, error) {
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return "", err
			}
			return template.HTML(b), nil
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"hasData": func(probeResult httpxrunner.ProbeResult) bool {
			return probeResult.StatusCode > 0
		},
		"statusClass": func(status string) string {
			switch status {
			case "new":
				return "status-new"
			case "old":
				return "status-old"
			case "existing":
				return "status-existing"
			default:
				return ""
			}
		},
	}
}
