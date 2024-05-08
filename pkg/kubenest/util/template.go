package util

import (
	"bytes"
	"fmt"
	"text/template"
)

// ParseTemplate validates and parses passed as argument template
func ParseTemplate(strtmpl string, obj interface{}) (string, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("template").Parse(strtmpl)
	if err != nil {
		return "", fmt.Errorf("error when parsing template, err: %w", err)
	}
	err = tmpl.Execute(&buf, obj)
	if err != nil {
		return "", fmt.Errorf("error when executing template, err: %w", err)
	}
	return buf.String(), nil
}
