package helpers

import (
	"reflect"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

type Filter struct {
	clusterNodes []*v1alpha1.ClusterNode
	clusters     []*v1alpha1.Cluster
	nodeConfigs  []*v1alpha1.NodeConfig
	clustersMap  map[string]*v1alpha1.Cluster
}

func NewFilter(clusterNodes []v1alpha1.ClusterNode, clusters []v1alpha1.Cluster, nodeConfigs []v1alpha1.NodeConfig) *Filter {
	cm := buildClustersMap(clusters)
	cs := convertToPointerSlice(clusters)
	cns := convertToPointerSlice(clusterNodes)
	ncs := convertToPointerSlice(nodeConfigs)
	return &Filter{
		clusterNodes: cns.([]*v1alpha1.ClusterNode),
		clusters:     cs.([]*v1alpha1.Cluster),
		nodeConfigs:  ncs.([]*v1alpha1.NodeConfig),
		clustersMap:  cm,
	}
}

func (f *Filter) GetGatewayNodes() []*v1alpha1.ClusterNode {
	var results []*v1alpha1.ClusterNode
	for _, cluster := range f.clusters {
		gw := f.GetGatewayNodeByClusterName(cluster.Name)
		if gw != nil {
			results = append(results, gw)
		}
	}
	return results
}

func (f *Filter) GetInternalNodes() []*v1alpha1.ClusterNode {
	var results []*v1alpha1.ClusterNode
	for _, node := range f.clusterNodes {
		cluster := f.GetClusterByName(node.Spec.ClusterName)
		if cluster.IsGateway() && !node.IsGateway() {
			results = append(results, node)
		}
	}
	return results
}

func (f *Filter) GetEndpointNodes() []*v1alpha1.ClusterNode {
	var results []*v1alpha1.ClusterNode
	for _, node := range f.clusterNodes {
		cluster := f.GetClusterByName(node.Spec.ClusterName)
		if cluster.IsP2P() && !node.IsGateway() {
			results = append(results, node)
		}
	}
	return results
}

func (f *Filter) GetClusters() []*v1alpha1.Cluster {
	var results []*v1alpha1.Cluster
	for _, cluster := range f.clusters {
		results = append(results, cluster)
	}
	return nil
}

func (f *Filter) GetGatewayClusters() []*v1alpha1.Cluster {
	var results []*v1alpha1.Cluster
	for _, cluster := range f.clusters {
		if cluster.IsGateway() {
			results = append(results, cluster)
		}
	}
	return nil
}

func (f *Filter) GetClusterNodes() []*v1alpha1.ClusterNode {
	var results []*v1alpha1.ClusterNode
	for _, node := range f.clusterNodes {
		results = append(results, node)
	}
	return results
}

func (f *Filter) GetGatewayNodeByClusterName(clusterName string) *v1alpha1.ClusterNode {
	for _, node := range f.clusterNodes {
		if node.Spec.ClusterName == clusterName && node.IsGateway() {
			return node
		}
	}
	return nil
}

func (f *Filter) GetAllNodesByClusterName(name string) []*v1alpha1.ClusterNode {
	var results []*v1alpha1.ClusterNode
	for _, node := range f.clusterNodes {
		if node.Spec.ClusterName == name {
			results = append(results, node)
		}
	}
	return results
}

func (f *Filter) GetAllNodesExceptCluster(name string) []*v1alpha1.ClusterNode {
	var results []*v1alpha1.ClusterNode
	for _, node := range f.clusterNodes {
		if node.Spec.ClusterName != name {
			results = append(results, node)
		}
	}
	return results
}

func (f *Filter) GetClusterByName(name string) *v1alpha1.Cluster {
	return f.clustersMap[name]
}

func (f *Filter) GetGatewayClusterNodes() []*v1alpha1.ClusterNode {
	var results []*v1alpha1.ClusterNode
	for _, cluster := range f.clusters {
		if cluster.IsGateway() {
			nodes := f.GetAllNodesByClusterName(cluster.Name)
			results = append(results, nodes...)
		}
	}
	return results
}

func (f *Filter) SupportIPv4(node *v1alpha1.ClusterNode) bool {
	cluster := f.GetClusterByName(node.Spec.ClusterName)
	return cluster.Spec.IPFamily == v1alpha1.IPFamilyTypeALL || cluster.Spec.IPFamily == v1alpha1.IPFamilyTypeIPV4
}

func (f *Filter) SupportIPv6(node *v1alpha1.ClusterNode) bool {
	cluster := f.GetClusterByName(node.Spec.ClusterName)
	return cluster.Spec.IPFamily == v1alpha1.IPFamilyTypeALL || cluster.Spec.IPFamily == v1alpha1.IPFamilyTypeIPV6
}

func (f *Filter) GetDeviceFromNodeConfig(nodeName string, devName string) *v1alpha1.Device {
	for _, config := range f.nodeConfigs {
		if config.Name == nodeName {
			for i, dev := range config.Spec.Devices {
				if dev.Name == devName {
					return &config.Spec.Devices[i]
				}
			}
			break
		}
	}
	return nil
}

func buildClustersMap(clusters []v1alpha1.Cluster) map[string]*v1alpha1.Cluster {
	results := make(map[string]*v1alpha1.Cluster)
	for i, c := range clusters {
		results[c.Name] = &clusters[i]
	}
	return results
}

func convertToPointerSlice(slice interface{}) interface{} {
	sliceValue := reflect.ValueOf(slice)
	sliceType := reflect.TypeOf(slice)

	pointerType := reflect.PtrTo(sliceType.Elem())
	pointerSlice := reflect.MakeSlice(reflect.SliceOf(pointerType), sliceValue.Len(), sliceValue.Len())

	for i := 0; i < sliceValue.Len(); i++ {
		element := sliceValue.Index(i)
		pointer := reflect.New(pointerType.Elem())
		pointer.Elem().Set(element)
		pointerSlice.Index(i).Set(pointer)
	}

	return pointerSlice.Interface()
}
