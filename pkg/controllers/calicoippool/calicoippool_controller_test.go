package calicoippool

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"

	v3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"

	"github.com/kosmos.io/clusterlink/pkg/utils"
)

type fakeIPPoolClient struct {
	ippools []v3.IPPool
}

func (f *fakeIPPoolClient) CreateOrUpdateCalicoIPPool(ipPools []*ExternalClusterIPPool) error {
	for _, pool := range ipPools {
		ippool := v3.IPPool{}
		ippool.Name = genCalicoIPPoolName(pool.cluster, pool.ipType, pool.ipPool)
		ippool.Spec.CIDR = pool.ipPool
		ippool.Spec.Disabled = true
		f.ippools = append(f.ippools, ippool)
	}
	return nil
}
func (f *fakeIPPoolClient) DeleteIPPool(ipPools []*ExternalClusterIPPool) error {
	newIPPools := make([]v3.IPPool, 0, 5)
	for _, pool := range f.ippools {
		find := false
		for _, target := range ipPools {
			if pool.Spec.CIDR != target.ipPool {
				find = true
			}
		}
		if !find {
			newIPPools = append(newIPPools, pool)
		}
	}
	f.ippools = newIPPools
	return nil
}
func (f *fakeIPPoolClient) ListExternalIPPools() ([]*ExternalClusterIPPool, error) {
	extClusterIpPools := make([]*ExternalClusterIPPool, 0, 5)
	for _, pool := range f.ippools {
		if strings.HasPrefix(pool.Name, utils.ExternalIPPoolNamePrefix) {
			ipType := getIPType(pool.Name)
			extPool := &ExternalClusterIPPool{
				cluster: getClusterName(pool.Name, ipType),
				ipPool:  ipType,
				ipType:  pool.Spec.CIDR,
			}
			extClusterIpPools = append(extClusterIpPools, extPool)
		}
	}
	return extClusterIpPools, nil
}

func TestSyncIPPool(t *testing.T) {
	currentCluster := "cluster1"
	remoteCluster := "cluster2"
	tests := []struct {
		name             string
		globalExtIPPools ExternalIPPoolSet
		client           *fakeIPPoolClient
		want             []ExternalClusterIPPool
	}{
		{
			name: "create",
			globalExtIPPools: ExternalIPPoolSet{
				ExternalClusterIPPool{
					cluster: remoteCluster,
					ipType:  PODIPType,
					ipPool:  "10.232.64.0/18",
				}: struct{}{},
				ExternalClusterIPPool{
					cluster: currentCluster,
					ipType:  PODIPType,
					ipPool:  "10.233.64.0/18",
				}: struct{}{},
			},
			client: &fakeIPPoolClient{
				ippools: []v3.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "default-ipv4",
						},
						Spec: v3.IPPoolSpec{
							CIDR:     "10.233.64.0/18",
							Disabled: false,
						},
					},
				},
			},
			want: []ExternalClusterIPPool{
				{
					cluster: remoteCluster,
					ipType:  PODIPType,
					ipPool:  "10.232.64.0/18",
				},
			},
		},
		{
			name: "delete",
			globalExtIPPools: ExternalIPPoolSet{
				ExternalClusterIPPool{
					cluster: remoteCluster,
					ipType:  PODIPType,
					ipPool:  "10.232.64.0/18",
				}: struct{}{},
				ExternalClusterIPPool{
					cluster: currentCluster,
					ipType:  PODIPType,
					ipPool:  "10.233.64.0/18",
				}: struct{}{},
			},
			client: &fakeIPPoolClient{
				ippools: []v3.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "default-ipv4",
						},
						Spec: v3.IPPoolSpec{
							CIDR:     "10.233.64.0/18",
							Disabled: false,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("clusterlink-%s-pod-10-234-64-0-18", remoteCluster),
						},
						Spec: v3.IPPoolSpec{
							CIDR:     "10.230.64.0/18",
							Disabled: true,
						},
					},
				},
			},
			want: []ExternalClusterIPPool{
				{
					cluster: remoteCluster,
					ipType:  PODIPType,
					ipPool:  "10.232.64.0/18",
				},
			},
		},
	}
	for _, tt := range tests {
		err := syncIPPool(currentCluster, tt.globalExtIPPools, tt.client)
		if err != nil {
			t.Errorf("synIPPool err %v", err)
			return
		}
		ippools := tt.client.ippools
		for _, p1 := range ippools {
			if !strings.HasPrefix(p1.Name, utils.ExternalIPPoolNamePrefix) {
				continue
			}
			find := false
			for _, p2 := range tt.want {
				if p1.Spec.CIDR == p2.ipPool {
					find = true
					break
				}
			}
			if !find {
				t.Errorf("want does has ippool %v", p1)
			}
		}
		for _, p1 := range tt.want {
			find := false
			for _, p2 := range ippools {
				if p1.ipPool == p2.Spec.CIDR {
					find = true
					break
				}
			}
			if !find {
				t.Errorf("ippools does has ippool %v", p1)
			}
		}
	}
}

func TestGetClusterName(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "pod type test1",
			input: []string{"clusterlink-cluster-member1-local-pod-10-234-64-0-18", PODIPType},
			want:  "cluster-member1-local",
		},
		{
			name:  "pod type test2",
			input: []string{"clusterlink-clustermember1local-pod-10-234-64-0-18", PODIPType},
			want:  "clustermember1local",
		},
		{
			name:  "pod type test3",
			input: []string{"clusterlink-cluster-pod-member1-local-pod-10-234-64-0-18", PODIPType},
			want:  "cluster-pod-member1-local",
		},
		{
			name:  "service type test1",
			input: []string{"clusterlink-cluster-host-local-service-10-233-0-0-18", SERVICEIPType},
			want:  "cluster-host-local",
		},
		{
			name:  "service type test2",
			input: []string{"clusterlink-cluster-hostservice-local-service-10-233-0-0-18", SERVICEIPType},
			want:  "cluster-hostservice-local",
		},
		{
			name:  "err1",
			input: []string{"default-ipv4", SERVICEIPType},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := getClusterName(tt.input[0], tt.input[1])
			if name != tt.want {
				t.Errorf("getClusterName()=%v, want %v", name, tt.want)
			}
		})
	}
}
