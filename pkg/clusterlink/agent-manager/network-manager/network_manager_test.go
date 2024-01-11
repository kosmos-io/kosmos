package networkmanager

import (
	"fmt"
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network"
)

func TestNetworkManager_Diff(t *testing.T) {
	type args struct {
		oldConfig    *clusterlinkv1alpha1.NodeConfigSpec
		newConfig    *clusterlinkv1alpha1.NodeConfigSpec
		deleteConfig *clusterlinkv1alpha1.NodeConfigSpec
		createConfig *clusterlinkv1alpha1.NodeConfigSpec
	}

	tests := []struct {
		name string
		args args
	}{
		{name: "Test1", args: args{
			oldConfig: &clusterlinkv1alpha1.NodeConfigSpec{
				Devices:  []clusterlinkv1alpha1.Device{{"vxlan1", "vx-local", "210.168.11.13/8", "7a:0e:b7:89:90:45", "*", 55, 4877}},
				Routes:   []clusterlinkv1alpha1.Route{{"10.234.0.0/16", "220.168.11.4", "vx-bridge"}},
				Iptables: []clusterlinkv1alpha1.Iptables{{"nat", "POSTROUTING", "-s 220.0.0.0/8 -j MASQUERADE"}},
				Fdbs:     []clusterlinkv1alpha1.Fdb{{"192.168.11.7", "b2:52:aa:8d:37:b1", "vx-local"}, {"192.168.11.7", "ff:ff:ff:ff:ff:ff", "vx-local"}},
				Arps:     []clusterlinkv1alpha1.Arp{{"220.168.11.4", "8a:5c:50:6b:30:99", "vx-bridge"}},
			},
			newConfig: &clusterlinkv1alpha1.NodeConfigSpec{
				Devices:  []clusterlinkv1alpha1.Device{{"vxlan", "vx-local", "210.168.11.13/8", "7a:0e:b7:89:90:45", "*", 55, 4877}},
				Routes:   []clusterlinkv1alpha1.Route{{"10.234.0.0/16", "220.168.11.4", "vx-bridge"}},
				Iptables: []clusterlinkv1alpha1.Iptables{{"nat", "POSTROUTING", "-s 220.0.0.0/8 -j MASQUERADE"}},
				Fdbs:     []clusterlinkv1alpha1.Fdb{{"192.168.11.7", "ff:ff:ff:ff:ff:f1", "vx-local"}},
				Arps:     []clusterlinkv1alpha1.Arp{{"220.168.11.4", "8a:5c:50:6b:30:99", "vx-bridge"}},
			},
			deleteConfig: &clusterlinkv1alpha1.NodeConfigSpec{},
			createConfig: &clusterlinkv1alpha1.NodeConfigSpec{},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			net := network.NewNetWork(false)
			nManager := NewNetworkManager(net)
			flag, deleteConfig, createConfig := nManager.Diff(tt.args.oldConfig, tt.args.newConfig)
			fmt.Println("flag: ", flag)
			fmt.Println("deleteConfig: ", deleteConfig)
			fmt.Println("createConfig: ", createConfig)
		})
	}
}

func TestNetworkManager_WriteSys(t *testing.T) {
	type args struct {
		configDiff *ConfigDiff
	}

	tests := []struct {
		name string
		args args
	}{
		{"Test1", args{
			configDiff: &ConfigDiff{
				DeleteConfig: &clusterlinkv1alpha1.NodeConfigSpec{
					Devices:  []clusterlinkv1alpha1.Device{{"vxlan1", "vx-local", "210.168.11.13/8", "7a:0e:b7:89:90:45", "*", 55, 4877}},
					Routes:   []clusterlinkv1alpha1.Route{},
					Iptables: []clusterlinkv1alpha1.Iptables{},
					Fdbs:     []clusterlinkv1alpha1.Fdb{{"192.168.11.7", "b2:52:aa:8d:37:b1", "vx-local"}},
					Arps:     []clusterlinkv1alpha1.Arp{},
				},
				CreateConfig: &clusterlinkv1alpha1.NodeConfigSpec{
					Devices:  []clusterlinkv1alpha1.Device{{"vxlan1", "vx-local", "210.168.11.13/8", "7a:0e:b7:89:90:45", "*", 55, 4877}},
					Routes:   []clusterlinkv1alpha1.Route{},
					Iptables: []clusterlinkv1alpha1.Iptables{},
					Fdbs:     []clusterlinkv1alpha1.Fdb{{"192.168.11.7", "ff:ff:ff:ff:ff:ff", "vx-local"}},
					Arps:     []clusterlinkv1alpha1.Arp{},
				},
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			net := network.NewNetWork(false)
			nManager := NewNetworkManager(net)
			err := nManager.WriteSys(tt.args.configDiff)
			fmt.Println(err)
		})
	}

}
