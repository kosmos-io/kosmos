package utils

import (
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
