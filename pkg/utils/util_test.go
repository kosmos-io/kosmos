// nolint:dupl
package utils

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// TestContainsString tests the ContainsString function
// 测试用例说明：
// 1. **String is present in the array**: 测试目标字符串在数组中存在的情况。
// 2. **String is not present in the array**: 测试目标字符串不在数组中的情况。
// 3. **Empty array**: 测试空数组的情况，应该返回  false 。
// 4. **String is an empty substring**: 测试空字符串作为目标字符串的情况，应该返回  true 。
// 5. **Array with empty strings**: 测试数组中包含空字符串的情况，目标字符串为空时，应该返回  true 。
// 6. **Case sensitivity test**: 测试大小写敏感的情况，应该返回  false 。
// 7. **Partial match test**: 测试部分匹配的情况，目标字符串在某个数组元素中。
func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		arr      []string
		s        string
		expected bool
	}{
		{
			name:     "String is present in the array",
			arr:      []string{"apple", "banana", "cherry"},
			s:        "ana",
			expected: true,
		},
		{
			name:     "String is not present in the array",
			arr:      []string{"apple", "banana", "cherry"},
			s:        "orange",
			expected: false,
		},
		{
			name:     "Empty array",
			arr:      []string{},
			s:        "any",
			expected: false,
		},
		{
			name:     "String is an empty substring",
			arr:      []string{"apple", "banana", "cherry"},
			s:        "",
			expected: true, // 空字符串在任何字符串中都被认为是包含的
		},
		{
			name:     "Array with empty strings",
			arr:      []string{"", "banana", "cherry"},
			s:        "",
			expected: true, // 空字符串在数组中的一个元素
		},
		{
			name:     "Case sensitivity test",
			arr:      []string{"Apple", "Banana", "Cherry"},
			s:        "apple",
			expected: false, // 大小写敏感
		},
		{
			name:     "Partial match test",
			arr:      []string{"apple pie", "banana split", "cherry tart"},
			s:        "pie",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsString(tt.arr, tt.s)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsIPv6 tests the IsIPv6 function
// 测试用例说明：
// 1. **Valid IPv6 Address**: 测试有效的 IPv6 地址。
// 2. **Valid IPv6 Address with Port**: 测试带有端口的有效 IPv6 地址。
// 3. **Valid IPv6 Address with Zone ID**: 测试带有区域 ID 的有效 IPv6 地址。
// 4. **Valid IPv4 Address**: 测试有效的 IPv4 地址，应该返回  false 。
// 5. **Valid IPv4 CIDR**: 测试有效的 IPv4 CIDR 地址，应该返回  false 。
// 6. **Invalid Address - Mixed**: 测试混合地址，应该返回  false 。
// 7. **Invalid Address - Only Colons**: 测试仅包含冒号的地址，应该返回  true 。
// 8. **Empty String**: 测试空字符串，应该返回  false 。
// 9. **Invalid Address - Non-IP**: 测试非 IP 地址的字符串，应该返回  false 。
func TestIsIPv6(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Valid IPv6 Address",
			input:    "2001:db8::1",
			expected: true,
		},
		{
			name:     "Valid IPv6 Address with Port",
			input:    "[2001:db8::1]:8080",
			expected: true,
		},
		{
			name:     "Valid IPv6 Address with Zone ID",
			input:    "2001:db8::1%eth0",
			expected: true,
		},
		{
			name:     "Valid IPv4 Address",
			input:    "192.168.1.1",
			expected: false,
		},
		{
			name:     "Valid IPv4 CIDR",
			input:    "192.168.1.0/24",
			expected: false,
		},
		{
			name:     "Invalid Address - Mixed",
			input:    "192.168.1.1:2001:db8::1",
			expected: false,
		},
		{
			name:     "Invalid Address - Only Colons",
			input:    "::",
			expected: true,
		},
		{
			name:     "Empty String",
			input:    "",
			expected: false,
		},
		{
			name:     "Invalid Address - Non-IP",
			input:    "invalid_address",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPv6(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestGetEnvWithDefaultValue tests the GetEnvWithDefaultValue function
// 测试用例说明：
// 1. **Environment variable exists**: 测试环境变量存在且有值的情况。
// 2. **Environment variable does not exist**: 测试环境变量不存在的情况，应该返回默认值。
// 3. **Environment variable is empty**: 测试环境变量存在但为空的情况，应该返回默认值。
// 4. **Environment variable is empty but has default**: 测试环境变量为空并且有不同的默认值。
func TestGetEnvWithDefaultValue(t *testing.T) {
	tests := []struct {
		name         string
		envName      string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "Environment variable exists",
			envName:      "TEST_ENV",
			envValue:     "some_value",
			defaultValue: "default_value",
			expected:     "some_value",
		},
		{
			name:         "Environment variable does not exist",
			envName:      "NON_EXISTENT_ENV",
			envValue:     "",
			defaultValue: "default_value",
			expected:     "default_value",
		},
		{
			name:         "Environment variable is empty",
			envName:      "EMPTY_ENV",
			envValue:     "",
			defaultValue: "default_value",
			expected:     "default_value",
		},
		{
			name:         "Environment variable is empty but has default",
			envName:      "EMPTY_ENV_WITH_DEFAULT",
			envValue:     "",
			defaultValue: "another_default_value",
			expected:     "another_default_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the environment variable for the test
			if tt.envValue != "" {
				os.Setenv(tt.envName, tt.envValue)
			} else {
				os.Unsetenv(tt.envName)
			}

			result := GetEnvWithDefaultValue(tt.envName, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}

	// Clean up environment variables after tests
	defer func() {
		os.Unsetenv("TEST_ENV")
		os.Unsetenv("NON_EXISTENT_ENV")
		os.Unsetenv("EMPTY_ENV")
		os.Unsetenv("EMPTY_ENV_WITH_DEFAULT")
	}()
}

// TestGenerateAddr tests the GenerateAddr function.
// 测试用例说明：
// 1. **Valid IPv4 Address**: 测试有效的 IPv4 地址。
// 2. **Valid IPv6 Address**: 测试有效的 IPv6 地址。
// 3. **Valid IPv4 Address with Port**: 测试有效的 IPv4 地址和端口。
// 4. **Valid IPv6 Address with Port**: 测试有效的 IPv6 地址和端口。
// 5. **Empty Address**: 测试空地址的情况，检查是否能正确处理。
// 6. **Empty Port**: 测试空端口的情况，检查是否能正确处理。
// 7. **IPv4 Address with Special Characters**: 测试有效的 IPv4 地址与特殊字符。
func TestGenerateAddrStr(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		port     string
		expected string
	}{
		{
			name:     "Valid IPv4 Address",
			addr:     "192.168.1.1",
			port:     "8080",
			expected: "192.168.1.1:8080",
		},
		{
			name:     "Valid IPv6 Address",
			addr:     "2001:db8::1",
			port:     "8080",
			expected: "[2001:db8::1]:8080",
		},
		{
			name:     "Valid IPv4 Address with Port",
			addr:     "10.0.0.2",
			port:     "80",
			expected: "10.0.0.2:80",
		},
		{
			name:     "Valid IPv6 Address with Port",
			addr:     "::1",
			port:     "443",
			expected: "[::1]:443",
		},
		{
			name:     "Empty Address",
			addr:     "",
			port:     "3000",
			expected: ":3000",
		},
		{
			name:     "Empty Port",
			addr:     "192.168.1.1",
			port:     "",
			expected: "192.168.1.1:",
		},
		{
			name:     "IPv4 Address with Special Characters",
			addr:     "192.168.1.1",
			port:     "1234",
			expected: "192.168.1.1:1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateAddrStr(tt.addr, tt.port)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestIPFamilyGenerator tests the IPFamilyGenerator function.
// 测试用例说明：
// 1. **Single IPv4**: 测试单个 IPv4 地址。
// 2. **Single IPv6**: 测试单个 IPv6 地址。
// 3. **Mixed IPv4 and IPv6**: 测试同时包含 IPv4 和 IPv6 地址的情况。
// 4. **Multiple IPv4**: 测试多个 IPv4 地址。
// 5. **Multiple IPv6**: 测试多个 IPv6 地址。
func TestIPFamilyGenerator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []corev1.IPFamily
	}{
		{
			name:     "Single IPv4",
			input:    "192.168.1.0/24",
			expected: []corev1.IPFamily{corev1.IPv4Protocol},
		},
		{
			name:     "Single IPv6",
			input:    "2001:db8::/32",
			expected: []corev1.IPFamily{corev1.IPv6Protocol},
		},
		{
			name:     "Mixed IPv4 and IPv6",
			input:    "192.168.1.0/24,2001:db8::/32",
			expected: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
		},
		{
			name:     "Multiple IPv4",
			input:    "10.0.0.0/8,172.16.0.0/12",
			expected: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv4Protocol},
		},
		{
			name:     "Multiple IPv6",
			input:    "2001:db8::/32,2001:db8:1::/32",
			expected: []corev1.IPFamily{corev1.IPv6Protocol, corev1.IPv6Protocol},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IPFamilyGenerator(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected[i], result[i])
				}
			}
		})
	}
}

// TestFormatCIDR tests the FormatCIDR function.
// 测试用例说明：
// 1. **Valid IPv4 CIDR**: 测试有效的 IPv4 CIDR 表示法。
// 2. **Valid IPv6 CIDR**: 测试有效的 IPv6 CIDR 表示法。
// 3. **Invalid CIDR - No IP**: 测试缺少 IP 的无效 CIDR 表示法。
// 4. **Invalid CIDR - No Prefix**: 测试缺少前缀的无效 CIDR 表示法。
// 5. **Invalid CIDR - Incorrect Format**: 测试格式不正确的无效 CIDR 表示法（前缀超出范围）。
// 6. **Invalid CIDR - Non-IP**: 测试非 IP 的无效 CIDR 表示法。
// 7. **Empty CIDR**: 测试空字符串作为输入的情况。
func TestFormatCIDR(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:      "Valid IPv4 CIDR",
			input:     "192.168.1.0/24",
			expected:  "192.168.1.0/24",
			expectErr: false,
		},
		{
			name:      "Valid IPv6 CIDR",
			input:     "2001:db8::/32",
			expected:  "2001:db8::/32",
			expectErr: false,
		},
		{
			name:      "Invalid CIDR - No IP",
			input:     "/24",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "Invalid CIDR - No Prefix",
			input:     "192.168.1.0/",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "Invalid CIDR - Incorrect Format",
			input:     "192.168.1.0/33",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "Invalid CIDR - Non-IP",
			input:     "invalidCIDR",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "Empty CIDR",
			input:     "",
			expected:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatCIDR(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
