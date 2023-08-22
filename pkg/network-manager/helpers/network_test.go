package helpers

import (
	"strings"
	"testing"
)

func Test_GenerateVxlanIP(t *testing.T) {

	tests := []struct {
		name    string
		ip      string
		destNet string
		want    string
	}{
		{
			name:    "ipv6",
			ip:      "2409:7c85:6200::a0e:1702",
			destNet: "9480::/16",
			want:    "9480:7c85:6200::a0e:1702/16",
		},
		{
			name:    "ipv4",
			ip:      "100.10.10.1",
			destNet: "210.0.0.0/8",
			want:    "210.10.10.1/8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := GenerateVxlanIP(tt.ip, tt.destNet); !strings.EqualFold(got, tt.want) {
				t.Errorf("helpers.GenerateVxlanIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
