package ipam

import (
	"net"
	"testing"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

const iprange = "10.233.4.0/24"
const ip6range = "2409:8C85:6200::/120"

// prepare test data
func generateTestEnvDataForGW() ([]*clusterlinkv1alpha1.Cluster, []*clusterlinkv1alpha1.ClusterNode) {

	cluster1 := &clusterlinkv1alpha1.Cluster{}

	cluster1.SetName("cluster-host-local")
	cluster1.Spec = clusterlinkv1alpha1.ClusterSpec{
		Namespace:    "clusterlink-system",
		NetworkType:  "gateway",
		PodCIDRs:     []string{"10.233.64.0/18"},
		ServiceCIDRs: []string{"10.233.0.0/18"},
		UseIPPool:    true,
		LocalCIDRs: clusterlinkv1alpha1.VxlanCIDRs{
			IP:  iprange,
			IP6: ip6range,
		},
	}

	cluster1Node1 := &clusterlinkv1alpha1.ClusterNode{}
	cluster1Node1.SetName("cluster-host-local-cluster-host-local-control-plane")
	cluster1Node1.Spec = clusterlinkv1alpha1.ClusterNodeSpec{
		ClusterName: "cluster-host-local",
		IP:          "172.19.0.4",
		NodeName:    "cluster-host-local-control-plane",
		PodCIDRs:    []string{"10.233.92.0/26"},
		Roles:       []clusterlinkv1alpha1.Role{},
		VxlanLocal: &clusterlinkv1alpha1.VxlanInterface{
			IP:  "210.19.0.4",
			Mac: "a2:c3:60:25:77:3a",
		},
	}

	cluster1Node2 := &clusterlinkv1alpha1.ClusterNode{}
	cluster1Node2.SetName("cluster-host-local-cluster-host-local-worker")
	cluster1Node2.Spec = clusterlinkv1alpha1.ClusterNodeSpec{
		ClusterName: "cluster-host-local",
		IP:          "172.19.0.3",
		NodeName:    "cluster-host-local-worker",
		PodCIDRs:    []string{"10.233.102.128/26"},
		Roles:       []clusterlinkv1alpha1.Role{},
		VxlanLocal: &clusterlinkv1alpha1.VxlanInterface{
			IP:  "210.19.0.3",
			Mac: "2e:0b:1a:b9:d7:a4",
		},
	}

	cluster1Node3 := &clusterlinkv1alpha1.ClusterNode{}
	cluster1Node3.SetName("cluster-host-local-cluster-host-local-worker2")
	cluster1Node3.Spec = clusterlinkv1alpha1.ClusterNodeSpec{
		ClusterName: "cluster-host-local",
		IP:          "172.19.0.2",
		NodeName:    "cluster-host-local-worker2",
		PodCIDRs:    []string{"10.233.102.192/26"},
		Roles:       []clusterlinkv1alpha1.Role{"gateway"},
		VxlanLocal: &clusterlinkv1alpha1.VxlanInterface{
			IP:  "210.19.0.2",
			Mac: "06:04:be:00:55:7e",
		},
		VxlanBridge: &clusterlinkv1alpha1.VxlanInterface{
			IP:  "220.19.0.2",
			Mac: "d2:c0:27:e0:6c:04",
		},
	}

	clusters := []*clusterlinkv1alpha1.Cluster{
		cluster1,
	}
	clusterNodes := []*clusterlinkv1alpha1.ClusterNode{
		cluster1Node1, cluster1Node2, cluster1Node3,
	}

	return clusters, clusterNodes
}

func TestIPPool(t *testing.T) {

	pool := map[string]net.IP{
		"a": net.ParseIP("10.233.4.9"),
		"c": net.ParseIP("10.233.4.2"),
	}
	ippool := NewIPPool(iprange, ip6range, pool)

	t.Run("test ipv4 success", func(t *testing.T) {

		newIP, _, _, err := ippool.Allocate("test")
		if err != nil {
			t.Errorf("allocate error : %v", err)
			return
		}

		if _, net, _ := net.ParseCIDR(iprange); !net.Contains(newIP) {
			t.Errorf("new ip is not in cidr, : %v", newIP)
			return
		}

	})

	t.Run("test ipv6 success", func(t *testing.T) {
		newIP, _, _, err := ippool.Allocate("test6")
		if err != nil {
			t.Errorf("allocate error : %v", err)
			return
		}

		newIP6, err := ippool.ToIPv6(newIP)
		if err != nil {
			t.Errorf("gen ipv6 error : %v", newIP6)
			return
		}

		ip4byte := newIP6[12:]

		for index, b := range ip4byte {
			if b != newIP[index] {
				t.Errorf("gen ipv6 error, ip not match, %v, %v", newIP, ip4byte)
				return
			}
		}

	})
}

func TestIPPoolManager(t *testing.T) {
	t.Run("test ipv4 success", func(t *testing.T) {
		clusters, _ := generateTestEnvDataForGW()

		clustersNil := []*clusterlinkv1alpha1.Cluster{}
		clusterNodesNil := []*clusterlinkv1alpha1.ClusterNode{}

		ipMgr := NewIPPoolManager(clustersNil, clusterNodesNil)

		_, _, _, err := ipMgr.AllocateLocalIP(clusters[0], "cluster-host-local-cluster-host-local-control-plane")
		if err != nil {
			t.Errorf("gen ipv error by manager: %v", err)
			return
		}
		// fmt.Println(ip.String())
	})
}

func TestCIDRIPGenerator(t *testing.T) {
	t.Run("test CIDRIPGenerator", func(t *testing.T) {

		data := [][]string{
			{
				"210.0.0.0/8",
				"100.31.10.1",
				"210.31.10.1",
			},
			{
				"9480::/16",
				"7712::22:11",
				"9480::22:11",
			},
		}

		for _, d := range data {
			r, _, _, err := CIDRIPGenerator(d[0], d[1])
			if err != nil {
				t.Error(err)
				return
			}
			if r.String() != d[2] {
				t.Errorf("result is unexcepted: %v, %v, %v, %v", d[0], d[1], d[2], r.String())
				return
			}
		}

	})
}
