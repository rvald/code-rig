package utils

import (
	"bytes"
	"fmt"
	"text/template"
)

func RenderTemplate(tmpl string, vars map[string]any) (string, error) {
	t, err := template.New("").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}
