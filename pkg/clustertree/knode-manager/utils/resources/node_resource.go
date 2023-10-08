package resources

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type NodeResource struct {
	sync.Mutex
	*corev1.Node
}

func (n *NodeResource) AddResource(resource *Resource) {
	if n.Node == nil {
		klog.Errorf("nodeResource node has not init")
		return
	}
	n.Lock()
	defer n.Unlock()
	vkResource := ConvertToResource(n.Status.Capacity)

	vkResource.Add(resource)
	vkResource.SetResourcesToNode(n.Node)
}

func (n *NodeResource) SubResource(resource *Resource) {
	if n.Node == nil {
		klog.Errorf("nodeResource node has not init")
		return
	}
	n.Lock()
	defer n.Unlock()
	vkResource := ConvertToResource(n.Status.Capacity)

	vkResource.Sub(resource)
	vkResource.SetResourcesToNode(n.Node)
}

func (n *NodeResource) DeepCopy() *corev1.Node {
	n.Lock()
	node := n.Node.DeepCopy()
	n.Unlock()
	return node
}
