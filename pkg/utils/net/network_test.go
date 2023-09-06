package net

import "testing"

func TestIntersect(t *testing.T) {
	type args struct {
		net1 string
		net2 string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "net1 contains net 2",
			args: args{
				net1: "192.168.0.0/24",
				net2: "192.168.0.0/26",
			},
			want: true,
		},
		{
			name: "net2 contains net1",
			args: args{
				net1: "10.233.1.0/8",
				net2: "10.233.0.0/24",
			},
			want: true,
		},
		{
			name: "net 1 intersect net 2",
			args: args{
				net1: "192.168.1.0/25",
				net2: "192.168.1.127/25",
			},
			want: true,
		},
		{
			name: "not intersect",
			args: args{
				net1: "192.168.1.0/25",
				net2: "192.168.1.128/25",
			},
			want: false,
		},
		{
			name: "the same",
			args: args{
				net1: "192.168.1.0/25",
				net2: "192.168.1.0/25",
			},
			want: true,
		},
		{
			name: "ipV6: net1 contains net 2",
			args: args{
				net1: "2001:db8:1234:5678::/64",
				net2: "2001:db8:1234:5678:abcd::/80",
			},
			want: true,
		},
		{
			name: "ipV6: net2 contains net1",
			args: args{
				net1: "2001:db8:1234:5678:abcd::/80",
				net2: "2001:db8:1234:5678::/64",
			},
			want: true,
		},
		{
			name: "ipV6: net 1 intersect net 2",
			args: args{
				net1: "::ffff:192.168.1.0/121",
				net2: "::ffff:192.168.1.128/130",
			},
			want: true,
		},
		{
			name: "ipV6: not intersect",
			args: args{
				net1: "2001:0db8:1234:5678::/64",
				net2: "2001:0db8:1234:5679::/64",
			},
			want: false,
		},
		{
			name: "ipV6: the same",
			args: args{
				net1: "2001:0db8:1234:5678::/64",
				net2: "2001:0db8:1234:5678::/64",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Intersect(tt.args.net1, tt.args.net2); got != tt.want {
				t.Errorf("Intersect() = %v, want %v", got, tt.want)
			}
		})
	}
}
