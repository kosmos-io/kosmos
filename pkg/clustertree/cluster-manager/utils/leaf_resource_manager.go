package utils

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

var (
	instance LeafResourceManager
	once     sync.Once
)

type LeafMode int

const (
	ALL  LeafMode = 0
	Node LeafMode = 1
	// Party LeafMode = 2
)

type ClusterNode struct {
	NodeName string
	LeafMode LeafMode
}

type LeafResource struct {
	Client               client.Client
	DynamicClient        dynamic.Interface
	Clientset            kubernetes.Interface
	KosmosClient         kosmosversioned.Interface
	ClusterName          string
	Namespace            string
	IgnoreLabels         []string
	EnableServiceAccount bool
	Nodes                []ClusterNode
	RestConfig           *rest.Config
}

type LeafResourceManager interface {
	AddLeafResource(string, *LeafResource, []kosmosv1alpha1.LeafModel, []*corev1.Node)
	RemoveLeafResource(string)
	// get leafresource by cluster name
	GetLeafResource(string) (*LeafResource, error)
	// get leafresource by knode name
	GetLeafResourceByNodeName(string) (*LeafResource, error)
	// judge if the map has leafresource of nodename
	Has(string) bool
	HasNodeName(string) bool
	ListNodeNames() []string
	GetClusterNode(string) *ClusterNode
}

type leafResourceManager struct {
	resourceMap              map[string]*LeafResource
	leafResourceManagersLock sync.Mutex
}

func has(clusternodes []ClusterNode, target string) bool {
	for _, v := range clusternodes {
		if v.NodeName == target {
			return true
		}
	}
	return false
}

func getClusterNode(clusternodes []ClusterNode, target string) *ClusterNode {
	for _, v := range clusternodes {
		if v.NodeName == target {
			return &v
		}
	}
	return nil
}

func (l *leafResourceManager) AddLeafResource(clustername string, lptr *LeafResource, leafModels []kosmosv1alpha1.LeafModel, nodes []*corev1.Node) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	clusterNodes := []ClusterNode{}
	for i, n := range nodes {
		if leafModels != nil && len(leafModels[i].NodeSelector.NodeName) > 0 {
			clusterNodes = append(clusterNodes, ClusterNode{
				NodeName: n.Name,
				LeafMode: Node,
			})
			// } else if leafModels != nil && leafModels[i].NodeSelector.LabelSelector != nil {
			// 	// TODO:
		} else {
			clusterNodes = append(clusterNodes, ClusterNode{
				NodeName: n.Name,
				LeafMode: ALL,
			})
		}
	}
	lptr.Nodes = clusterNodes
	l.resourceMap[clustername] = lptr
}

func (l *leafResourceManager) RemoveLeafResource(clustername string) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	delete(l.resourceMap, clustername)
}

func (l *leafResourceManager) GetLeafResource(clustername string) (*LeafResource, error) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	if m, ok := l.resourceMap[clustername]; ok {
		return m, nil
	} else {
		return nil, fmt.Errorf("cannot get leaf resource, clustername: %s", clustername)
	}
}

func (l *leafResourceManager) GetLeafResourceByNodeName(nodename string) (*LeafResource, error) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()

	for k := range l.resourceMap {
		if has(l.resourceMap[k].Nodes, nodename) {
			return l.resourceMap[k], nil
		}
	}

	return nil, fmt.Errorf("cannot get leaf resource, nodename: %s", nodename)
}

func (l *leafResourceManager) HasNodeName(nodename string) bool {
	for k := range l.resourceMap {
		if has(l.resourceMap[k].Nodes, nodename) {
			return true
		}
	}

	return false
}

func (l *leafResourceManager) Has(clustername string) bool {
	for k := range l.resourceMap {
		if k == clustername {
			return true
		}
	}

	return false
}

func (l *leafResourceManager) GetClusterNode(nodename string) *ClusterNode {
	for k := range l.resourceMap {
		if clusterNode := getClusterNode(l.resourceMap[k].Nodes, nodename); clusterNode != nil {
			return clusterNode
		}
	}
	return nil
}

func (l *leafResourceManager) ListNodeNames() []string {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	keys := make([]string, 0, len(l.resourceMap))
	for k := range l.resourceMap {
		if len(k) == 0 {
			continue
		}
		if len(l.resourceMap[k].Nodes) == 0 {
			continue
		}
		for _, node := range l.resourceMap[k].Nodes {
			keys = append(keys, node.NodeName)
		}
	}
	return keys
}

func GetGlobalLeafResourceManager() LeafResourceManager {
	once.Do(func() {
		instance = &leafResourceManager{
			resourceMap: make(map[string]*LeafResource),
		}
	})

	return instance
}
