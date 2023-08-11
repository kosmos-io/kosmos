package ipam

import (
	"context"
	"fmt"
	"net"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"k8s.io/klog/v2"
)

const (
	ClusterGroup    = "clusterlink.io"
	ClusterVersion  = "v1alpha1"
	ClusterResource = "clusters"
	ClusterKind     = "Cluster"
)

const (
	ClusterNodeGroup    = "clusterlink.io"
	ClusterNodeVersion  = "v1alpha1"
	ClusterNodeResource = "clusternodes"
	ClusterNodeKind     = "ClusterNode"
)

type IPPoolManager struct {
	LocalIPPool  map[string]*IPPool
	BridgeIPPool map[string]*IPPool
}

func getClusters(dynamicClient *dynamic.DynamicClient) ([]*clusterlinkv1alpha1.Cluster, error) {
	gvr := schema.GroupVersionResource{
		Group:    ClusterGroup,
		Version:  ClusterVersion,
		Resource: ClusterResource,
	}
	listClusters, err := dynamicClient.Resource(gvr).List(context.TODO(), meta.ListOptions{})
	if err != nil {
		return nil, err
	}

	clusterList := &clusterlinkv1alpha1.ClusterList{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(listClusters.UnstructuredContent(), clusterList); err != nil {
		return nil, err
	}

	ret := []*clusterlinkv1alpha1.Cluster{}

	for index, _ := range clusterList.Items {
		ret = append(ret, &clusterList.Items[index])
	}
	return ret, nil
}

func getClusterNodes(dynamicClient *dynamic.DynamicClient) ([]*clusterlinkv1alpha1.ClusterNode, error) {
	gvr := schema.GroupVersionResource{
		Group:    ClusterNodeGroup,
		Version:  ClusterNodeVersion,
		Resource: ClusterNodeResource,
	}
	listClusterNodes, err := dynamicClient.Resource(gvr).List(context.TODO(), meta.ListOptions{})
	if err != nil {
		return nil, err
	}

	clusterNodeList := &clusterlinkv1alpha1.ClusterNodeList{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(listClusterNodes.UnstructuredContent(), clusterNodeList); err != nil {
		return nil, err
	}

	ret := []*clusterlinkv1alpha1.ClusterNode{}

	for index, _ := range clusterNodeList.Items {
		ret = append(ret, &clusterNodeList.Items[index])
	}
	return ret, nil
}

func NewIPPoolManagerByKubeConfig(restConfig *rest.Config) (*IPPoolManager, error) {

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	clusters, err := getClusters(dynamicClient)
	if err != nil {
		return nil, err
	}

	clusterNodes, err := getClusterNodes(dynamicClient)
	if err != nil {
		return nil, err
	}

	return NewIPPoolManager(clusters, clusterNodes), nil

}

func NewIPPoolManager(clusters []*v1alpha1.Cluster, nodes []*v1alpha1.ClusterNode) *IPPoolManager {

	LocalIPPool := map[string]*IPPool{}
	BridgeIPPool := map[string]*IPPool{}

	for _, cluster := range clusters {
		if !cluster.Spec.UseIPPool {
			continue
		}
		local := map[string]net.IP{}
		bridge := map[string]net.IP{}
		for _, node := range nodes {
			if node.Spec.ClusterName == cluster.Name {
				if len(node.Spec.VxlanLocal.IP) > 0 {
					local[node.Name] = net.ParseIP(node.Spec.VxlanLocal.IP)
				}
				if len(node.Spec.VxlanBridge.IP) > 0 {
					bridge[node.Name] = net.ParseIP(node.Spec.VxlanBridge.IP)
				}
			}
		}

		LocalIPPool[cluster.Name] = NewIPPool(cluster.Spec.LocalCIDRs.IP, cluster.Spec.LocalCIDRs.IP6, local)
		BridgeIPPool[cluster.Name] = NewIPPool(cluster.Spec.BridgeCIDRs.IP, cluster.Spec.BridgeCIDRs.IP6, bridge)
	}

	klog.Info(LocalIPPool)
	klog.Info(BridgeIPPool)

	return &IPPoolManager{
		LocalIPPool,
		BridgeIPPool,
	}
}

func (i *IPPoolManager) IPGen(cluster *v1alpha1.Cluster, cidr string, internalIP string, f func() (net.IP, int, int, error)) (net.IP, int, int, error) {
	if cluster.Spec.UseIPPool {
		return f()
	}

	ip, ones, bits, err := CIDRIPGenerator(cidr, internalIP)
	if err != nil {
		return nil, 0, 0, err
	}
	return *ip, ones, bits, nil
}

func (i *IPPoolManager) UpdateClusterNode(clusterNode *v1alpha1.ClusterNode, cluster *v1alpha1.Cluster, internalIP, internalIP6 string) error {
	// clusterNode.Spec.AssignIPs = clusterlinkv1alpha1.AssignIPs{}
	ipKey := clusterNode.Name

	if cluster.Spec.IPFamily == clusterlinkv1alpha1.IPFamilyTypeALL || cluster.Spec.IPFamily == clusterlinkv1alpha1.IPFamilyTypeIPV4 {
		clusterNode.Spec.IP = internalIP

		bridgeIP, ones, bits, err := i.IPGen(cluster, cluster.Spec.BridgeCIDRs.IP, internalIP, func() (net.IP, int, int, error) { return i.AllocateBridgeIP(cluster, ipKey) })
		if err != nil {
			return fmt.Errorf("could not create cluster node %s error: %v, when allocate bridge ip", clusterNode.Name, err)

		}
		if clusterNode.Spec.VxlanBridge == nil {
			clusterNode.Spec.VxlanBridge = &v1alpha1.VxlanInterface{}
		}
		clusterNode.Spec.VxlanBridge.IP = bridgeIP.String()
		clusterNode.Spec.VxlanBridge.Ones = ones
		clusterNode.Spec.VxlanBridge.Bits = bits

		localIP, ones, bits, err := i.IPGen(cluster, cluster.Spec.LocalCIDRs.IP, internalIP, func() (net.IP, int, int, error) { return i.AllocateLocalIP(cluster, ipKey) })
		if err != nil {
			return fmt.Errorf("could not create cluster node %s error: %v, when allocate local ip", clusterNode.Name, err)
		}
		if clusterNode.Spec.VxlanLocal == nil {
			clusterNode.Spec.VxlanLocal = &v1alpha1.VxlanInterface{}
		}
		clusterNode.Spec.VxlanLocal.IP = localIP.String()
		clusterNode.Spec.VxlanLocal.Ones = ones
		clusterNode.Spec.VxlanLocal.Bits = bits
	}
	if cluster.Spec.IPFamily == clusterlinkv1alpha1.IPFamilyTypeALL || cluster.Spec.IPFamily == clusterlinkv1alpha1.IPFamilyTypeIPV6 {
		clusterNode.Spec.IP6 = internalIP6

		bridgeIP6, ones, bits, err := i.IPGen(cluster, cluster.Spec.BridgeCIDRs.IP6, internalIP6, func() (net.IP, int, int, error) { return i.AllocateBridgeIP6(cluster, ipKey) })

		if err != nil {
			return fmt.Errorf("could not create cluster node %s error: %v, when allocate bridge6 ip", clusterNode.Name, err)
		}
		if clusterNode.Spec.VxlanBridge6 == nil {
			clusterNode.Spec.VxlanBridge6 = &v1alpha1.VxlanInterface{}
		}
		clusterNode.Spec.VxlanBridge6.IP = bridgeIP6.String()
		clusterNode.Spec.VxlanBridge6.Ones = ones
		clusterNode.Spec.VxlanBridge6.Bits = bits

		localIP6, ones, bits, err := i.IPGen(cluster, cluster.Spec.LocalCIDRs.IP6, internalIP6, func() (net.IP, int, int, error) { return i.AllocateLocalIP6(cluster, ipKey) })
		if err != nil {
			return fmt.Errorf("could not create cluster node %s error: %v, when allocate local6 ip", clusterNode.Name, err)
		}
		if clusterNode.Spec.VxlanLocal6 == nil {
			clusterNode.Spec.VxlanLocal6 = &v1alpha1.VxlanInterface{}
		}
		clusterNode.Spec.VxlanLocal6.IP = localIP6.String()
		clusterNode.Spec.VxlanLocal6.Ones = ones
		clusterNode.Spec.VxlanLocal6.Bits = bits
	}
	return nil
}

func (i *IPPoolManager) AddCluster(cluster *v1alpha1.Cluster) {
	if !cluster.Spec.UseIPPool {
		return
	}
	i.LocalIPPool[cluster.Name] = NewIPPool(cluster.Spec.LocalCIDRs.IP, cluster.Spec.LocalCIDRs.IP6, nil)
	i.BridgeIPPool[cluster.Name] = NewIPPool(cluster.Spec.BridgeCIDRs.IP, cluster.Spec.BridgeCIDRs.IP6, nil)

}
func (i *IPPoolManager) DeleteCluster(clusterName string) {
	delete(i.LocalIPPool, clusterName)
	delete(i.BridgeIPPool, clusterName)
}

// TODO: some ip while allocated, because of node-controller cannot deal with delete event of clusternode
func (i *IPPoolManager) DeletelusterNode(clusterNode *v1alpha1.ClusterNode) {
	ipool := i.LocalIPPool[clusterNode.Spec.ClusterName]
	ipool.Release(clusterNode.Name)

	ipool2 := i.BridgeIPPool[clusterNode.Spec.ClusterName]
	ipool2.Release(clusterNode.Name)
}

func (i *IPPoolManager) AllocateLocalIP(cluster *v1alpha1.Cluster, key string) (net.IP, int, int, error) {
	klog.Infof("ippool-mgr: allocate local ip: clusterName: %v, key: %v", cluster.GetName(), key)
	ipool := i.LocalIPPool[cluster.GetName()]
	if ipool == nil {
		i.AddCluster(cluster)
		ipool = i.LocalIPPool[cluster.GetName()]
	}
	return ipool.Allocate(key)
}

func (i *IPPoolManager) AllocateBridgeIP(cluster *v1alpha1.Cluster, key string) (net.IP, int, int, error) {
	klog.Infof("ippool-mgr: allocate bridge ip: clusterName: %v, key: %v", cluster.GetName(), key)
	ipool := i.BridgeIPPool[cluster.GetName()]
	if ipool == nil {
		i.AddCluster(cluster)
		ipool = i.BridgeIPPool[cluster.GetName()]
	}
	return ipool.Allocate(key)
}

func (i *IPPoolManager) AllocateLocalIP6(cluster *v1alpha1.Cluster, key string) (net.IP, int, int, error) {
	klog.Infof("ippool-mgr: allocate local ip6: clusterName: %v, key: %v", cluster.GetName(), key)
	ipool := i.LocalIPPool[cluster.GetName()]
	if ipool == nil {
		i.AddCluster(cluster)
		ipool = i.LocalIPPool[cluster.GetName()]
	}
	ip4, ones, bits, err := ipool.Allocate(key)
	if err != nil {
		return nil, ones, bits, err
	}
	ret, err := ipool.ToIPv6(ip4)
	return ret, ones, bits, err
}

func (i *IPPoolManager) AllocateBridgeIP6(cluster *v1alpha1.Cluster, key string) (net.IP, int, int, error) {
	klog.Infof("ippool-mgr: allocate bridge ip6: clusterName: %v, key: %v", cluster.GetName(), key)
	ipool := i.BridgeIPPool[cluster.GetName()]
	if ipool == nil {
		i.AddCluster(cluster)
		ipool = i.BridgeIPPool[cluster.GetName()]
	}
	ip4, ones, bits, err := ipool.Allocate(key)
	if err != nil {
		return nil, ones, bits, err
	}
	ret, err := ipool.ToIPv6(ip4)
	return ret, ones, bits, err
}

func CIDRIPGenerator(ipcidr string, internalIP string) (*net.IP, int, int, error) {
	cidrip, ipNet, err := net.ParseCIDR(ipcidr)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("CIDRIPGenerator err: %v", err)
	}

	nodeIP := net.ParseIP(internalIP)

	ret := net.ParseIP("0.0.0.0")
	for i := range ipNet.Mask {
		ret[len(ret)-i-1] = ^byte(ipNet.Mask[len(ipNet.Mask)-i-1])
	}

	ones, bits := ipNet.Mask.Size()

	klog.Info("CIDRIPGenerator -> ", "ones: ", ones, "bits: ", bits)

	for i := range nodeIP {
		ret[i] = byte(ret[i]) & byte(nodeIP[i])
	}

	for i := range cidrip {
		ret[i] = byte(ret[i]) | byte(cidrip[i])
	}

	return &ret, ones, bits, nil
}
