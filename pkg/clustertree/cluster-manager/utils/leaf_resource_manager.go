package leafUtils

import (
	"fmt"
	"sync"

	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LeafResource struct {
	Client               client.Client
	DynamicClient        dynamic.Interface
	NodeName             string
	Namespace            string
	IgnoreLabels         []string
	EnableServiceAccount bool
}

type LeafResourceManager interface {
	AddLeafResource(string, *LeafResource)
	RemoveLeafResource(string)
	GetLeafResource(string) (*LeafResource, error)
	IsInCluded(string) bool
	ListNodeNames() []string
}

type leafResourceManager struct {
	resourceMap              map[string]*LeafResource
	leafResourceManagersLock sync.Mutex
}

func (l *leafResourceManager) AddLeafResource(nodename string, lptr *LeafResource) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	l.resourceMap[nodename] = lptr
}

func (l *leafResourceManager) RemoveLeafResource(nodename string) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	delete(l.resourceMap, nodename)
}

func (l *leafResourceManager) GetLeafResource(nodename string) (*LeafResource, error) {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	if m, ok := l.resourceMap[nodename]; ok {
		return m, nil
	} else {
		return nil, fmt.Errorf("cannot get leaf resource, nodename: %s", nodename)
	}
}

func (l *leafResourceManager) IsInCluded(nodename string) bool {
	if _, err := l.GetLeafResource(nodename); err != nil {
		return false
	}
	return true
}

func (l *leafResourceManager) ListNodeNames() []string {
	l.leafResourceManagersLock.Lock()
	defer l.leafResourceManagersLock.Unlock()
	keys := make([]string, 0, len(l.resourceMap))
	for k := range l.resourceMap {
		if len(k) == 0 {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func NewLeafResourceManager() LeafResourceManager {
	return &leafResourceManager{
		resourceMap: make(map[string]*LeafResource),
	}
}
