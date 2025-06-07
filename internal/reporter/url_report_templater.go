package reporter

import (
	"encoding/json"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
)

// createTemplateFunctionMap creates the function map for HTML templates
func (r *HtmlReporter) createTemplateFunctionMap() template.FuncMap {
	funcMap := GetCommonTemplateFunctions()

	// Add HTML reporter specific functions
	funcMap["json"] = func(v interface{}) (template.JS, error) {
		a, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return template.JS(a), nil
	}

	return funcMap
}

// loadCustomTemplate loads template from file path
func (r *HtmlReporter) loadCustomTemplate() error {
	customTmpl := template.New(filepath.Base(r.cfg.TemplatePath)).Funcs(r.createTemplateFunctionMap())
	_, err := customTmpl.ParseFiles(r.cfg.TemplatePath)
	if err != nil {
		r.logger.Error().Err(err).Str("path", r.cfg.TemplatePath).Msg("Failed to parse custom report template.")
		return fmt.Errorf("failed to parse custom report template '%s': %w", r.cfg.TemplatePath, err)
	}

	r.template = customTmpl
	return nil
}

// loadEmbeddedTemplate loads the default embedded template
func (r *HtmlReporter) loadEmbeddedTemplate(tmpl *template.Template) error {
	templateContent, err := defaultTemplate.ReadFile("templates/report.html.tmpl")
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to read embedded default report template.")
		return fmt.Errorf("failed to load embedded default report template: %w", err)
	}

	cleanedContent := strings.ReplaceAll(string(templateContent), "\r\n", "\n")
	_, err = tmpl.Parse(cleanedContent)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to parse cleaned embedded report template.")
		return fmt.Errorf("failed to parse cleaned embedded report template: %w", err)
	}

	r.template = tmpl
	return nil
}
