package util

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFindAvailableIP(t *testing.T) {
	type args struct {
		vipPool       []string
		allocatedVips []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				vipPool:       []string{"192.168.0.1", "192.168.0.2", "192.168.0.3"},
				allocatedVips: []string{"192.168.0.1", "192.168.0.2"},
			},
			want:    "192.168.0.3",
			wantErr: false,
		},
		{
			name: "test2",
			args: args{
				vipPool: []string{
					"192.168.0.1",
					"192.168.0.2-192.168.0.10",
					"192.168.1.0/24",
					"2001:db8::1",
					"2001:db8::1-2001:db8::10",
					"2001:db8::/64",
				},
				allocatedVips: []string{"192.168.0.1", "192.168.0.2"},
			},
			want:    "192.168.0.3",
			wantErr: false,
		},
		{
			name: "test3",
			args: args{
				vipPool: []string{
					"192.168.6.110-192.168.6.120",
				},
				allocatedVips: []string{},
			},
			want:    "192.168.6.110",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindAvailableIP(tt.args.vipPool, tt.args.allocatedVips)
			fmt.Printf("got vip : %v", got)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindAvailableIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FindAvailableIP() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindAvailableIP2(t *testing.T) {
	type HostPortPool struct {
		PortsPool []int32 `yaml:"portsPool"`
	}
	type VipPool struct {
		Vip []string `yaml:"vipPool"`
	}
	var vipPool VipPool
	var hostPortPool HostPortPool
	yamlData2 := `
portsPool:
  - 33001
  - 33002
  - 33003
  - 33004
  - 33005
  - 33006
  - 33007
  - 33008
  - 33009
  - 33010
`
	yamlData := `
vipPool:
  - 192.168.6.110-192.168.6.120
`
	if err := yaml.Unmarshal([]byte(yamlData), &vipPool); err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal([]byte(yamlData2), &hostPortPool); err != nil {
		panic(err)
	}
	fmt.Printf("vipPool: %v", vipPool)
}
