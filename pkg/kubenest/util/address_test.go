package util

import (
	"testing"

	netutils "k8s.io/utils/net"
)

func TestGetAPIServiceIP(t *testing.T) {
	client, err := prepare()
	if err != nil {
		t.Logf("failed to prepare client: %v", err)
		return
	}

	str, err := GetAPIServiceIP(client)
	if err != nil {
		t.Logf("failed to get api service ip: %v", err)
	}
	if len(str) == 0 {
		t.Logf("api service ip is empty")
	} else {
		t.Logf("api service ip is %s", str)
	}
}

func TestParseIP(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ipv4", "10.237.6.0", "10.237.6.0"},
		{"ipv6", "2409:8c2f:3800:0011::0a18:0000", "2409:8c2f:3800:11::a18:0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := netutils.ParseIPSloppy(tt.input)
			if ip.String() != tt.want {
				t.Fatalf("%s, %s, %s, %s", tt.name, tt.input, ip.String(), tt.want)
			}
		})
	}
}
