package agent

import (
	"testing"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestFormatNodeConfig(t *testing.T) {
	tests := []struct {
		name  string
		input *kosmosv1alpha1.NodeConfig
		want  *kosmosv1alpha1.NodeConfig
	}{
		{
			name: "test ipv4 and ipv6",
			input: &kosmosv1alpha1.NodeConfig{
				Spec: kosmosv1alpha1.NodeConfigSpec{
					Routes: []kosmosv1alpha1.Route{
						{
							CIDR: "2409:8c2f:3800:0011::0a18:0000/114",
						},
						{
							CIDR: "10.237.6.0/18",
						},
					},
				},
			},
			want: &kosmosv1alpha1.NodeConfig{
				Spec: kosmosv1alpha1.NodeConfigSpec{
					Routes: []kosmosv1alpha1.Route{
						{
							CIDR: "2409:8c2f:3800:11::a18:0/114",
						},
						{
							CIDR: "10.237.0.0/18",
						},
					},
				},
			},
		},
		{
			name: "test ipv6",
			input: &kosmosv1alpha1.NodeConfig{
				Spec: kosmosv1alpha1.NodeConfigSpec{
					Routes: []kosmosv1alpha1.Route{
						{
							CIDR: "2409:8c2f:3800:0011::0a18:0000/114",
						},
					},
				},
			},
			want: &kosmosv1alpha1.NodeConfig{
				Spec: kosmosv1alpha1.NodeConfigSpec{
					Routes: []kosmosv1alpha1.Route{
						{
							CIDR: "2409:8c2f:3800:11::a18:0/114",
						},
					},
				},
			},
		},
		{
			name: "test ipv4",
			input: &kosmosv1alpha1.NodeConfig{
				Spec: kosmosv1alpha1.NodeConfigSpec{
					Routes: []kosmosv1alpha1.Route{
						{
							CIDR: "10.237.6.0/18",
						},
					},
				},
			},
			want: &kosmosv1alpha1.NodeConfig{
				Spec: kosmosv1alpha1.NodeConfigSpec{
					Routes: []kosmosv1alpha1.Route{
						{
							CIDR: "10.237.0.0/18",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeconfig, err := formatNodeConfig(tt.input)

			if err != nil {
				t.Errorf("formatNodeConfig() error = %v", err)
			}

			if len(nodeconfig.Spec.Routes) != len(tt.want.Spec.Routes) {
				t.Errorf("formatNodeConfig() = %v, want %v", nodeconfig.Spec.Routes, tt.want.Spec.Routes)
			}

			for i := range nodeconfig.Spec.Routes {
				if nodeconfig.Spec.Routes[i].CIDR != tt.want.Spec.Routes[i].CIDR {
					t.Errorf("formatNodeConfig() = %v, want %v", nodeconfig.Spec.Routes[i].CIDR, tt.want.Spec.Routes[i].CIDR)
				}
			}
		})
	}
}
