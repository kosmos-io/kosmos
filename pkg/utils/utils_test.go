package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestContainsStringInUtils(t *testing.T) {
	tests := []struct {
		arr    []string
		s      string
		expect bool
	}{
		{[]string{"apple", "banana", "cherry"}, "ban", true},
		{[]string{"apple", "banana", "cherry"}, "berry", false},
		{[]string{"apple", "banana", "cherry"}, "apple", true},
		{[]string{"apple", "banana", "cherry"}, "plum", false},
	}

	for _, test := range tests {
		result := ContainsString(test.arr, test.s)
		assert.Equal(t, test.expect, result, "Expected ContainsString(%v, %s) to be %v", test.arr, test.s, test.expect)
	}
}

func TestIsIPv6InUtils(t *testing.T) {
	tests := []struct {
		ip     string
		expect bool
	}{
		{"192.168.1.1", false},
		{"::1", true},
		{"2001:db8::1", true},
		{"127.0.0.1", false},
	}

	for _, test := range tests {
		result := IsIPv6(test.ip)
		assert.Equal(t, test.expect, result, "Expected IsIPv6(%s) to be %v", test.ip, test.expect)
	}
}

func TestGetEnvWithDefaultValueInUtils(t *testing.T) {
	os.Setenv("EXISTING_ENV", "value")
	defer os.Unsetenv("EXISTING_ENV")

	tests := []struct {
		envName      string
		defaultValue string
		expected     string
	}{
		{"EXISTING_ENV", "default", "value"},
		{"NON_EXISTING_ENV", "default", "default"},
	}

	for _, test := range tests {
		result := GetEnvWithDefaultValue(test.envName, test.defaultValue)
		assert.Equal(t, test.expected, result, "Expected GetEnvWithDefaultValue(%s, %s) to be %s", test.envName, test.defaultValue, test.expected)
	}
}

func TestGenerateAddrStrInUtils(t *testing.T) {
	tests := []struct {
		addr   string
		port   string
		expect string
	}{
		{"192.168.1.1", "8080", "192.168.1.1:8080"},
		{"::1", "8080", "[::1]:8080"},
		{"2001:db8::1", "8080", "[2001:db8::1]:8080"},
	}

	for _, test := range tests {
		result := GenerateAddrStr(test.addr, test.port)
		assert.Equal(t, test.expect, result, "Expected GenerateAddrStr(%s, %s) to be %s", test.addr, test.port, test.expect)
	}
}

func TestIPFamilyGeneratorInUtils(t *testing.T) {
	tests := []struct {
		apiServerServiceSubnet string
		expect                 []corev1.IPFamily
	}{
		{"192.168.0.0/16", []corev1.IPFamily{corev1.IPv4Protocol}},
		{"2001:db8::/32", []corev1.IPFamily{corev1.IPv6Protocol}},
		{"192.168.0.0/16,2001:db8::/32", []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}},
	}

	for _, test := range tests {
		result := IPFamilyGenerator(test.apiServerServiceSubnet)
		assert.Equal(t, test.expect, result, "Expected IPFamilyGenerator(%s) to be %v", test.apiServerServiceSubnet, test.expect)
	}
}

func TestFormatCIDRInUtils(t *testing.T) {
	tests := []struct {
		cidr     string
		expect   string
		hasError bool
	}{
		{"192.168.0.0/16", "192.168.0.0/16", false},
		{"2001:db8::/32", "2001:db8::/32", false},
		{"invalid_cidr", "", true},
	}

	for _, test := range tests {
		result, err := FormatCIDR(test.cidr)
		if test.hasError {
			assert.Error(t, err, "Expected FormatCIDR(%s) to return an error", test.cidr)
		} else {
			assert.NoError(t, err, "Expected FormatCIDR(%s) to not return an error", test.cidr)
			assert.Equal(t, test.expect, result, "Expected FormatCIDR(%s) to be %s", test.cidr, test.expect)
		}
	}
}
