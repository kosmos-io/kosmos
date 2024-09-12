package utils

import (
	"fmt"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

var (
	clientInstance LeafClientResourceManager
	clientOnce     sync.Once
)

type LeafClientResource struct {
	Client        client.Client
	DynamicClient dynamic.Interface
	Clientset     kubernetes.Interface
	KosmosClient  kosmosversioned.Interface
	RestConfig    *rest.Config
}

type LeafClientResourceManager interface {
	AddLeafClientResource(lcr *LeafClientResource, cluster *kosmosv1alpha1.Cluster)

	RemoveLeafClientResource(actualClusterName string)

	GetLeafResource(actualClusterName string) (*LeafClientResource, error)

	ListActualClusters() []string
}

type leafClientResourceManager struct {
	clientResourceMap              map[string]*LeafClientResource
	leafClientResourceManagersLock sync.Mutex
}

func (cr *leafClientResourceManager) GetLeafResource(actualClusterName string) (*LeafClientResource, error) {
	cr.leafClientResourceManagersLock.Lock()
	defer cr.leafClientResourceManagersLock.Unlock()
	if m, ok := cr.clientResourceMap[actualClusterName]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("cannot get leaf client resource, actualClusterName: %s", actualClusterName)
}

func (cr *leafClientResourceManager) AddLeafClientResource(lcr *LeafClientResource, cluster *kosmosv1alpha1.Cluster) {
	cr.leafClientResourceManagersLock.Lock()
	defer cr.leafClientResourceManagersLock.Unlock()

	actualClusterName := GetActualClusterName(cluster)

	// Only adds or updates the lcr in clientResourceMap if actualClusterName does not exist.
	// This prevents updating the map if an entry for actualClusterName already exists.
	if _, exists := cr.clientResourceMap[actualClusterName]; !exists {
		cr.clientResourceMap[actualClusterName] = lcr
	}
}

func (cr *leafClientResourceManager) ListActualClusters() []string {
	cr.leafClientResourceManagersLock.Lock()
	defer cr.leafClientResourceManagersLock.Unlock()

	keys := make([]string, 0)
	for k := range cr.clientResourceMap {
		if len(k) == 0 {
			continue
		}

		keys = append(keys, k)
	}
	return keys
}

func (cr *leafClientResourceManager) RemoveLeafClientResource(actualClusterName string) {
	cr.leafClientResourceManagersLock.Lock()
	defer cr.leafClientResourceManagersLock.Unlock()
	delete(cr.clientResourceMap, actualClusterName)
}

func GetGlobalLeafClientResourceManager() LeafClientResourceManager {
	clientOnce.Do(func() {
		clientInstance = &leafClientResourceManager{
			clientResourceMap: make(map[string]*LeafClientResource),
		}
	})

	return clientInstance
}

// GetActualClusterName extracts the actualClusterName from the cluster labels, or use the cluster's name if the specific label is not present.
func GetActualClusterName(cluster *kosmosv1alpha1.Cluster) string {
	var actualClusterName string
	labels := cluster.Labels
	if actualName, ok := labels[utils.KosmosActualClusterName]; ok {
		actualClusterName = actualName
	} else {
		actualClusterName = cluster.Name
	}
	return actualClusterName
}
