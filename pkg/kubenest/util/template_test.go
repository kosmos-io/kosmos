package util

import (
	"testing"
)

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name      string
		strtmpl   string
		obj       interface{}
		want      string
		expectErr bool
	}{
		{
			name:      "valid template with defaultValue",
			strtmpl:   `Hello, {{defaultValue .Name "World"}}!`,
			obj:       map[string]interface{}{"Name": "Alice"},
			want:      "Hello, Alice!",
			expectErr: false,
		},
		{
			name:      "valid template with default value",
			strtmpl:   `Hello, {{defaultValue .Name "World"}}!`,
			obj:       map[string]interface{}{},
			want:      "Hello, World!",
			expectErr: false,
		},
		{
			name:      "invalid template",
			strtmpl:   `Hello, {{.Name`, // Missing closing braces
			obj:       map[string]interface{}{"Name": "Alice"},
			want:      "",
			expectErr: true,
		},
		{
			name:      "template execution error",
			strtmpl:   `Hello, {{.Name}}!`,
			obj:       nil, // obj is nil, so this will fail during execution
			want:      "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTemplate(tt.strtmpl, tt.obj)
			if (err != nil) != tt.expectErr {
				t.Errorf("ParseTemplate() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}
