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

func Test_Intersect(t *testing.T) {
	tests := []struct {
		name  string
		cidr1 string
		cidr2 string
		want  bool
	}{
		{
			name:  "ipv4-1",
			cidr1: "10.233.0.0/16",
			cidr2: "10.233.0.0/18",
			want:  true,
		},
		{
			name:  "ipv4-2",
			cidr1: "10.233.0.0/18",
			cidr2: "10.233.0.0/16",
			want:  true,
		},
		{
			name:  "ipv4-3",
			cidr1: "10.233.0.0/16",
			cidr2: "10.233.1.0/23",
			want:  true,
		},
		{
			name:  "ipv4-4",
			cidr1: "10.222.0.0/16",
			cidr2: "10.223.0.0/16",
			want:  false,
		},
		{
			name:  "ipv6",
			cidr1: "2409:7c85:6200::a0e:1722/16",
			cidr2: "2409:7c85:6200::a0e:1702/12",
			want:  true,
		},
		{
			name:  "err",
			cidr1: "10.233.0/16",
			cidr2: "10.233.0.0/18",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Intersect(tt.cidr1, tt.cidr2); got != tt.want {
				t.Errorf("helpers.Intersect() = %v, want %v", got, tt.want)
			}
		})
	}
}
