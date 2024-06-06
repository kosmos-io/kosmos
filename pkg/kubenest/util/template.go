package util

import (
	"bytes"
	"fmt"
	"text/template"
)

// pluginOptions
func defaultValue(value interface{}, defaultVal string) string {
	if str, ok := value.(string); ok && str != "" {
		return str
	}
	return defaultVal
}

// ParseTemplate validates and parses passed as argument template
func ParseTemplate(strtmpl string, obj interface{}) (string, error) {
	var buf bytes.Buffer
	tmpl := template.New("template").Funcs(template.FuncMap{
		"defaultValue": defaultValue,
	})
	tmpl, err := tmpl.Parse(strtmpl)
	if err != nil {
		return "", fmt.Errorf("error when parsing template, err: %w", err)
	}
	err = tmpl.Execute(&buf, obj)
	if err != nil {
		return "", fmt.Errorf("error when executing template, err: %w", err)
	}
	return buf.String(), nil
}
