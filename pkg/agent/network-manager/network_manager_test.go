package networkmanager

import (
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"github.com/kosmos.io/clusterlink/pkg/network"
)

func TestNetworkManager(t *testing.T) {
	t.Run("test networkManager", func(t *testing.T) {
		crd := &clusterlinkv1alpha1.NodeConfig{
			Spec: clusterlinkv1alpha1.NodeConfigSpec{
				Devices: []clusterlinkv1alpha1.Device{
					{
						Type:    "vxlan",
						Name:    "vx-bridge",
						Addr:    "220.20.30.130/8", // some rule
						Mac:     "a6:02:63:1f:cd:a3",
						BindDev: "ens33",
						ID:      54,
						Port:    4876,
					},
					{
						Type:    "vxlan",
						Name:    "vx-local",
						Addr:    "210.20.30.130/8", // some rule
						Mac:     "a6:02:63:1f:cd:a2",
						BindDev: "ens33",
						ID:      55,
						Port:    4866,
					},
				},
				Arps: []clusterlinkv1alpha1.Arp{
					{
						IP:  "220.76.1.4",
						Mac: "a6:01:63:1f:cd:a2",
						Dev: "vx-bridge",
					},
				},
				Fdbs: []clusterlinkv1alpha1.Fdb{
					{
						IP:  "220.76.1.4",
						Mac: "a6:01:63:1f:cd:a2",
						Dev: "vx-bridge",
					},
				},
				Iptables: []clusterlinkv1alpha1.Iptables{
					{
						Table: "nat",
						Chain: "PREROUTING",
						Rule:  "-d 242.222.0.0/18 -j NETMAP --to 10.222.0.0/18",
					},
				},
				Routes: []clusterlinkv1alpha1.Route{
					{
						CIDR: "242.222.0.0/18",
						Gw:   "220.20.30.131", // next jump addr
						Dev:  "vx-bridge",
					},
				},
			},
		}
		net := network.NewNetWork()
		nmr := NewNetworkManager(net)
		status := nmr.UpdateFromCRD(crd)
		if status != NodeConfigSyncSuccess {
			t.Errorf("update from CRD error: %s", nmr.Reason)
			return
		}

		status = nmr.UpdateFromCRD(&clusterlinkv1alpha1.NodeConfig{
			Spec: clusterlinkv1alpha1.NodeConfigSpec{}})

		if status != NodeConfigSyncSuccess {
			t.Errorf("update from CRD error: %s", nmr.Reason)
			return
		}
	})
}
