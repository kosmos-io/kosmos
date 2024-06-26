package utils

import (
	"fmt"
	"os"
	"strings"
)

func ContainsString(arr []string, s string) bool {
	for _, str := range arr {
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}

func IsIPv6(s string) bool {
	// 0.234.63.0 and 0.234.63.0/22
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return false
		case ':':
			return true
		}
	}
	return false
}

func GetEnvWithDefaultValue(envName string, defaultValue string) string {
	v := os.Getenv(envName)
	if len(v) == 0 {
		return defaultValue
	}
	return v
}

func GenerateAddrStr(addr string, port string) string {
	if IsIPv6(addr) {
		return fmt.Sprintf("[%s]:%s", addr, port)
	}
	return fmt.Sprintf("%s:%s", addr, port)
}
