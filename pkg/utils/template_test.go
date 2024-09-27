// nolint:dupl
package utils

import (
	"testing"
)

// TestParseTemplate tests the ParseTemplate function.
// 测试用例说明：
// 1. **Valid template with field**: 测试有效的Template，能够正确解析并替换字段。
// 2. **Valid template with multiple fields**: 测试包含多个字段的有效Template。
// 3. **Template with missing field**: 测试Template中缺少字段的情况，期望执行时出错。
// 4. **Invalid template syntax**: 测试无效的Template语法，期望解析时出错。
// 5. **Empty template**: 测试空Template的情况，应该返回空字节并不报错。
func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name      string
		strTmpl   string
		obj       interface{}
		expected  string
		expectErr bool
	}{
		{
			name:      "Valid template with field",
			strTmpl:   "Hello, {{.Name}}!",
			obj:       struct{ Name string }{Name: "Alice"},
			expected:  "Hello, Alice!",
			expectErr: false,
		},
		{
			name:    "Valid template with multiple fields",
			strTmpl: "{{.Greeting}}, {{.Name}}!",
			obj: struct {
				Greeting string
				Name     string
			}{Greeting: "Hello", Name: "Bob"},
			expected:  "Hello, Bob!",
			expectErr: false,
		},
		{
			name:      "Template with missing field",
			strTmpl:   "Hello, {{.Name}}!",
			obj:       struct{ Age int }{Age: 30},
			expected:  "",
			expectErr: true, // 期望执行时出错
		},
		{
			name:      "Invalid template syntax",
			strTmpl:   "Hello, {{.Name!",
			obj:       struct{ Name string }{Name: "Alice"},
			expected:  "",
			expectErr: true, // 期望解析时出错
		},
		{
			name:      "Empty template",
			strTmpl:   "",
			obj:       struct{ Name string }{Name: "Alice"},
			expected:  "",
			expectErr: false, // 空Template应返回空字节
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTemplate(tt.strTmpl, tt.obj)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
			if string(result) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
