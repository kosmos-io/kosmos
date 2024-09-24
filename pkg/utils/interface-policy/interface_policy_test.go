// nolint:dupl
package interfacepolicy

import (
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func TestGetInterfaceName(t *testing.T) {
	type args struct {
		networkInterfacePolicies []clusterlinkv1alpha1.NICNodeNames
		nodeName                 string
		defaultInterfaceName     string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test case 1",
			args: args{
				networkInterfacePolicies: []clusterlinkv1alpha1.NICNodeNames{
					{
						NodeName: []string{
							"node1",
						},
						InterfaceName: "interface1",
					},
					{
						NodeName: []string{
							"node2",
						},
						InterfaceName: "interface2",
					},
				},
				nodeName:             "node1",
				defaultInterfaceName: "default",
			},
			want: "interface1",
		},
		{
			name: "Test case 2",
			args: args{
				networkInterfacePolicies: []clusterlinkv1alpha1.NICNodeNames{
					{
						NodeName: []string{
							"node1",
						},
						InterfaceName: "interface1",
					},
					{
						NodeName: []string{
							"node2",
						},
						InterfaceName: "interface2",
					},
				},
				nodeName:             "node3",
				defaultInterfaceName: "default",
			},
			want: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetInterfaceName(tt.args.networkInterfacePolicies, tt.args.nodeName, tt.args.defaultInterfaceName); got != tt.want {
				t.Errorf("GetInterfaceName() = %v, want %v", got, tt.want)
			}
		})
	}
}
