// nolint:dupl
package utils

import "testing"

// TestGetCIDRs tests the GetCIDRs function.
func TestGetCIDRs(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		expected string
	}{
		{
			name:     "nil input",
			obj:      nil,
			expected: "",
		},
		{
			name:     "empty map",
			obj:      map[string]interface{}{},
			expected: "",
		},
		{
			name: "missing spec",
			obj: map[string]interface{}{
				"foo": "bar",
			},
			expected: "",
		},
		{
			name: "spec is not a map",
			obj: map[string]interface{}{
				"spec": "not-a-map",
			},
			expected: "",
		},
		{
			name: "spec without cidr",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"foo": "bar",
				},
			},
			expected: "",
		},
		{
			name: "spec with cidr",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"cidr": "192.168.1.0/24",
				},
			},
			expected: "192.168.1.0/24",
		},
		{
			name: "spec with cidr as empty string",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"cidr": "",
				},
			},
			expected: "",
		},
		{
			name: "cidr is not a string",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"cidr": 12345,
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCIDRs(tt.obj)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
