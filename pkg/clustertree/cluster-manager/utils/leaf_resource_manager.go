package utils

import (
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var (
	instance LeafResourceManager
	once     sync.Once
)

type LeafMode int

const (
	ALL LeafMode = iota
	Node
	Party
)

type LeafType string

const (
	LeafTypeK8s        LeafType = "k8s"
	LeafTypeServerless LeafType = "serverless"
)

type ClusterNode struct {
	NodeName         string
	LeafMode         LeafMode
	LeafNodeSelector kosmosv1alpha1.NodeSelector
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
	LeafType             LeafType

	ServerlessClient *ServerlessClient
}

func (lr *LeafResource) GetLeafType() LeafType {
	return lr.LeafType
}

type LeafResourceManager interface {
	AddLeafResource(lr *LeafResource, cluster *kosmosv1alpha1.Cluster, node []*corev1.Node)
	RemoveLeafResource(clusterName string)
	// get leafresource by cluster name
	GetLeafResource(clusterName string) (*LeafResource, error)
	// get leafresource by node name
	GetLeafResourceByNodeName(nodeName string) (*LeafResource, error)
	// determine if the cluster is present in the map
	HasCluster(clusterName string) bool
	// determine if the node is present in the map
	HasNode(nodeName string) bool
	// list all all node name
	ListNodes() []string
	// list all all cluster name
	ListClusters() []string
	// get ClusterNode(struct) by node name
	GetClusterNode(nodeName string) *ClusterNode
}

type leafResourceManager struct {
	resourceMap              map[string]*LeafResource
	leafResourceManagersLock sync.Mutex
}

func trimNamePrefix(name string) string {
	return strings.TrimPrefix(name, utils.KosmosNodePrefix)
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

func (l *leafResourceManager) AddLeafResource(lptr *LeafResource, cluster *kosmosv1alpha1.Cluster, nodes []*corev1.Node) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()

	clusterName := cluster.Name

	leafModels := cluster.Spec.ClusterTreeOptions.LeafModels

	clusterNodes := []ClusterNode{}
	for i, n := range nodes {
		if leafModels != nil && leafModels[i].NodeSelector.LabelSelector != nil {
			// TODO: support labelselector
			clusterNodes = append(clusterNodes, ClusterNode{
				NodeName:         trimNamePrefix(n.Name),
				LeafMode:         Party,
				LeafNodeSelector: leafModels[i].NodeSelector,
			})
		} else if leafModels != nil && len(leafModels[i].NodeSelector.NodeName) > 0 {
			clusterNodes = append(clusterNodes, ClusterNode{
				NodeName: n.Name,
				LeafMode: Node,
			})
		} else {
			clusterNodes = append(clusterNodes, ClusterNode{
				NodeName: trimNamePrefix(n.Name),
				LeafMode: ALL,
			})
		}
	}
	lptr.Nodes = clusterNodes
	l.resourceMap[clusterName] = lptr
}

func (l *leafResourceManager) RemoveLeafResource(clusterName string) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	delete(l.resourceMap, clusterName)
}

func (l *leafResourceManager) GetLeafResource(clusterName string) (*LeafResource, error) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	if m, ok := l.resourceMap[clusterName]; ok {
		return m, nil
	} else {
		return nil, fmt.Errorf("cannot get leaf resource, clusterName: %s", clusterName)
	}
}

func (l *leafResourceManager) GetLeafResourceByNodeName(nodeName string) (*LeafResource, error) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	nodeName = trimNamePrefix(nodeName)
	for k := range l.resourceMap {
		if has(l.resourceMap[k].Nodes, nodeName) {
			return l.resourceMap[k], nil
		}
	}

	return nil, fmt.Errorf("cannot get leaf resource, nodeName: %s", nodeName)
}

func (l *leafResourceManager) HasNode(nodeName string) bool {
	nodeName = trimNamePrefix(nodeName)
	for k := range l.resourceMap {
		if has(l.resourceMap[k].Nodes, nodeName) {
			return true
		}
	}

	return false
}

func (l *leafResourceManager) HasCluster(clustername string) bool {
	for k := range l.resourceMap {
		if k == clustername {
			return true
		}
	}

	return false
}

func (l *leafResourceManager) GetClusterNode(nodeName string) *ClusterNode {
	nodeName = trimNamePrefix(nodeName)
	for k := range l.resourceMap {
		if clusterNode := getClusterNode(l.resourceMap[k].Nodes, nodeName); clusterNode != nil {
			return clusterNode
		}
	}
	return nil
}

func (l *leafResourceManager) ListClusters() []string {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	keys := make([]string, 0)
	for k := range l.resourceMap {
		if len(k) == 0 {
			continue
		}

		keys = append(keys, k)
	}
	return keys
}

func (l *leafResourceManager) ListNodes() []string {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	keys := make([]string, 0)
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
