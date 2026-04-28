// internal/testrunner/reporter_html.go
package testrunner

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"strings"
)

//go:embed templates/report.html.tmpl
var htmlTemplateFS embed.FS

var htmlTemplate = template.Must(template.New("report.html.tmpl").Funcs(template.FuncMap{
	"lower": strings.ToLower,
	"join":  strings.Join,
}).ParseFS(htmlTemplateFS, "templates/report.html.tmpl"))

// WriteHTML renders a ReportModel to a self-contained HTML file.
func WriteHTML(path string, m ReportModel) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()
	if err := htmlTemplate.Execute(f, m); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}
	return nil
}
