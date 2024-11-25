package utils

import (
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	netutils "k8s.io/utils/net"
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

func IPFamilyGenerator(apiServerServiceSubnet string) []corev1.IPFamily {
	ipNetStrArray := strings.Split(apiServerServiceSubnet, ",")
	ipFamilies := []corev1.IPFamily{}
	for _, ipstr := range ipNetStrArray {
		if IsIPv6(ipstr) {
			ipFamilies = append(ipFamilies, corev1.IPv6Protocol)
		} else {
			ipFamilies = append(ipFamilies, corev1.IPv4Protocol)
		}
	}
	return ipFamilies
}

func FormatCIDR(cidr string) (string, error) {
	_, ipNet, err := netutils.ParseCIDRSloppy(cidr)
	if err != nil {
		return "", fmt.Errorf("failed to parse  cidr %s, err: %s", cidr, err.Error())
	}
	return ipNet.String(), nil
}

func HasKosmosNodeLabel(node *corev1.Node) bool {
	if kosmosNodeLabel, ok := node.Labels[KosmosNodeLabel]; ok && kosmosNodeLabel == KosmosNodeValue {
		return true
	}

	return false
}
