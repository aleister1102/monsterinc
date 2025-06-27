package reporter

import (
	"fmt"
	"html/template"
	"strings"
)

// createTemplateFunctionMap creates the function map for HTML templates
func (r *HtmlReporter) createTemplateFunctionMap() template.FuncMap {
	return GetCommonTemplateFunctions()
}

// loadEmbeddedTemplate loads the template from the embedded filesystem
func (r *HtmlReporter) loadEmbeddedTemplate(tmpl *template.Template) error {
	templateContent, err := templatesFS.ReadFile("templates/report_client_side.html.tmpl")
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
