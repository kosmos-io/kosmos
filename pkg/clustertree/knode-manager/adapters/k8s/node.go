package k8sadapter

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/clustertree/knode-manager/app/config"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/clustertree/knode-manager/utils/resources"
)

type NodeAdapter struct {
	client kubernetes.Interface
	master kubernetes.Interface

	cfg   *config.Opts
	knode *kosmosv1alpha1.Knode

	nodeResource resources.NodeResource

	clientNodeLister v1.NodeLister
	clientPodLister  v1.PodLister

	updatedNode chan *corev1.Node
	updatedPod  chan *corev1.Pod

	stopCh <-chan struct{}
}

func NewNodeAdapter(ctx context.Context, cr *kosmosv1alpha1.Knode, ac *AdapterConfig, c *config.Opts) (*NodeAdapter, error) {
	adapter := &NodeAdapter{
		client:           ac.Client,
		master:           ac.Master,
		cfg:              c,
		knode:            cr,
		stopCh:           ctx.Done(),
		clientNodeLister: ac.NodeInformer.Lister(),
		clientPodLister:  ac.PodInformer.Lister(),
		updatedNode:      make(chan *corev1.Node, 10),
	}

	_, err := ac.NodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    adapter.addNode,
		UpdateFunc: adapter.updateNode,
		DeleteFunc: adapter.deleteNode,
	})
	if err != nil {
		return nil, err
	}

	_, err = ac.PodInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    adapter.addPod,
		UpdateFunc: adapter.updatePod,
		DeleteFunc: adapter.deletePod,
	})
	if err != nil {
		return nil, err
	}

	return adapter, nil
}

func (n *NodeAdapter) Probe(_ context.Context) error {
	_, err := n.master.Discovery().ServerVersion()
	if err != nil {
		klog.Error("Failed ping")
		return fmt.Errorf("could not list master apiserver statuses: %v", err)
	}
	_, err = n.client.Discovery().ServerVersion()
	if err != nil {
		klog.Error("Failed ping")
		return fmt.Errorf("could not list client apiserver statuses: %v", err)
	}
	return nil
}

func (n *NodeAdapter) NotifyStatus(ctx context.Context, f func(*corev1.Node)) {
	klog.Info("Called NotifyNodeStatus")
	go func() {
		for {
			select {
			case node := <-n.updatedNode:
				klog.Infof("Enqueue updated node %v", node.Name)
				f(node)
			case <-n.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (n *NodeAdapter) Configure(ctx context.Context, node *corev1.Node) {
	serverVersion, err := n.client.Discovery().ServerVersion()
	if err != nil {
		return
	}

	nodes, err := n.clientNodeLister.List(labels.Everything())
	if err != nil {
		return
	}

	nodeResource := resources.NewResource()
	for _, n := range nodes {
		if n.Spec.Unschedulable {
			continue
		}
		if !checkNodeStatusReady(n) {
			klog.Infof("Node %v not ready", n.Name)
			continue
		}
		nc := resources.ConvertToResource(n.Status.Capacity)
		nodeResource.Add(nc)
	}
	podResource := n.getResourceFromPods(ctx)
	nodeResource.Sub(podResource)
	nodeResource.SetResourcesToNode(node)

	node.Status.NodeInfo.KubeletVersion = serverVersion.GitVersion
	node.Status.NodeInfo.OperatingSystem = utils.DefaultK8sOS
	node.Status.NodeInfo.Architecture = utils.DefaultK8sArch

	node.ObjectMeta.Labels[utils.NodeArchLabelStable] = utils.DefaultK8sArch
	node.ObjectMeta.Labels[utils.NodeOSLabelStable] = utils.DefaultK8sOS
	node.ObjectMeta.Labels[utils.NodeOSLabelBeta] = utils.DefaultK8sOS

	node.Status.Conditions = nodeConditions()
	node.Status.DaemonEndpoints = corev1.NodeDaemonEndpoints{
		KubeletEndpoint: corev1.DaemonEndpoint{
			Port: n.cfg.ListenPort,
		},
	}

	n.nodeResource.Node = node
}

func (n *NodeAdapter) addNode(obj interface{}) {
	addNode := obj.(*corev1.Node).DeepCopy()
	before := n.nodeResource.DeepCopy()

	toAdd := resources.ConvertToResource(addNode.Status.Capacity)
	n.nodeResource.AddResource(toAdd)

	toRemove := n.getResourceFromPodsByNodeName(addNode.Name)
	n.nodeResource.SubResource(toRemove)

	after := n.nodeResource.DeepCopy()
	if !reflect.DeepEqual(before, after) {
		n.updatedNode <- after
	}
}

func (n *NodeAdapter) updateNode(oldObj, newObj interface{}) {
	oldNode, ok1 := oldObj.(*corev1.Node)
	newNode, ok2 := newObj.(*corev1.Node)
	if !ok1 || !ok2 {
		return
	}

	oldCopy := oldNode.DeepCopy()
	newCopy := newNode.DeepCopy()

	klog.V(4).Infof("Node %v updated", oldNode.Name)
	n.updateNodeResources(oldCopy, newCopy)
}

func (n *NodeAdapter) deleteNode(obj interface{}) {
	before := n.nodeResource.DeepCopy()
	deleteNode := obj.(*corev1.Node).DeepCopy()
	toRemove := resources.ConvertToResource(deleteNode.Status.Capacity)
	n.nodeResource.SubResource(toRemove)
	n.nodeResource.AddResource(n.getResourceFromPodsByNodeName(deleteNode.Name))
	after := n.nodeResource.DeepCopy()
	if !reflect.DeepEqual(before, after) {
		n.updatedNode <- after
	}
}

func (n *NodeAdapter) addPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	podCopy := pod.DeepCopy()
	utils.TrimObjectMeta(&podCopy.ObjectMeta)
	if !utils.IsVirtualPod(podCopy) {
		if n.nodeResource.Node == nil {
			return
		}
		if len(podCopy.Spec.NodeName) != 0 {
			podResource := GetRequestFromPod(podCopy)
			podResource.Pods = resource.MustParse("1")
			n.nodeResource.SubResource(podResource)
			klog.Infof("Lower cluster add pod %s, resource: %v, node: %v",
				podCopy.Name, podResource, n.nodeResource.Status.Capacity)
			if n.nodeResource.Node == nil {
				return
			}
			cp := n.nodeResource.DeepCopy()
			n.updatedNode <- cp
		}
		return
	}
	n.updatedPod <- podCopy
}

func (n *NodeAdapter) updatePod(oldObj, newObj interface{}) {
	oldPod, ok1 := oldObj.(*corev1.Pod)
	newPod, ok2 := newObj.(*corev1.Pod)
	oldCopy := oldPod.DeepCopy()
	newCopy := newPod.DeepCopy()
	if !ok1 || !ok2 {
		return
	}
	if !utils.IsVirtualPod(newCopy) {
		if n.nodeResource.Node == nil {
			return
		}
		n.updatePodResources(oldCopy, newCopy)
		return
	}

	if newCopy.DeletionTimestamp != nil {
		newCopy.DeletionGracePeriodSeconds = nil
	}

	if !reflect.DeepEqual(oldCopy.Status, newCopy.Status) || newCopy.DeletionTimestamp != nil {
		utils.TrimObjectMeta(&newCopy.ObjectMeta)
		n.updatedPod <- newCopy
	}
}

func (n *NodeAdapter) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	podCopy := pod.DeepCopy()
	utils.TrimObjectMeta(&podCopy.ObjectMeta)
	if !utils.IsVirtualPod(podCopy) {
		if n.nodeResource.Node == nil {
			return
		}
		if podStopped(pod) {
			return
		}
		if len(podCopy.Spec.NodeName) != 0 {
			podResource := GetRequestFromPod(podCopy)
			podResource.Pods = resource.MustParse("1")
			n.nodeResource.AddResource(podResource)
			klog.Infof("Lower cluster add pod %s, resource: %v, node: %v",
				podCopy.Name, podResource, n.nodeResource.Status.Capacity)
			if n.nodeResource.Node == nil {
				return
			}
			cp := n.nodeResource.DeepCopy()
			n.updatedNode <- cp
		}
		return
	}
	n.updatedPod <- podCopy
}

func (n *NodeAdapter) updateNodeResources(old, new *corev1.Node) {
	oldStatus, newStatus := compareNodeStatusReady(old, new)
	if !oldStatus && !newStatus {
		return
	}

	toRemove := resources.ConvertToResource(old.Status.Capacity)
	toAdd := resources.ConvertToResource(new.Status.Capacity)

	before := n.nodeResource.DeepCopy()
	if old.Spec.Unschedulable && !new.Spec.Unschedulable || newStatus && !oldStatus {
		n.nodeResource.AddResource(toAdd)
		n.nodeResource.SubResource(n.getResourceFromPodsByNodeName(old.Name))
	}
	if !old.Spec.Unschedulable && new.Spec.Unschedulable || oldStatus && !newStatus {
		n.nodeResource.AddResource(n.getResourceFromPodsByNodeName(old.Name))
		n.nodeResource.SubResource(toRemove)
	}

	if !reflect.DeepEqual(old.Status.Allocatable, new.Status.Allocatable) ||
		!reflect.DeepEqual(old.Status.Capacity, new.Status.Capacity) {
		klog.Infof("Start to update node resource, old: %v, new %v", old.Status.Capacity,
			new.Status.Capacity)
		n.nodeResource.AddResource(toAdd)
		n.nodeResource.SubResource(toRemove)
		klog.Infof("Current node resource, resource: %v, allocatable %v", n.nodeResource.Status.Capacity,
			n.nodeResource.Status.Allocatable)
	}
	after := n.nodeResource.DeepCopy()

	if !reflect.DeepEqual(before, after) {
		n.updatedNode <- after
	}
}

func (n *NodeAdapter) updatePodResources(old, new *corev1.Pod) {
	newResource := GetRequestFromPod(new)
	oldResource := GetRequestFromPod(old)
	// create pod
	if old.Spec.NodeName == "" && new.Spec.NodeName != "" {
		newResource.Pods = resource.MustParse("1")
		n.nodeResource.SubResource(newResource)
		klog.Infof("Lower cluster add pod %s, resource: %v, node: %v",
			new.Name, newResource, n.nodeResource.Status.Capacity)
	}
	// delete pod
	if old.Status.Phase == corev1.PodRunning && podStopped(new) {
		klog.Infof("Lower cluster delete pod %s, resource: %v", new.Name, newResource)
		newResource.Pods = resource.MustParse("1")
		n.nodeResource.AddResource(newResource)
	}
	// update pod
	if new.Status.Phase == corev1.PodRunning && !reflect.DeepEqual(old.Spec.Containers,
		new.Spec.Containers) {
		if oldResource.Equal(newResource) {
			return
		}
		n.nodeResource.AddResource(oldResource)
		n.nodeResource.SubResource(newResource)

		klog.Infof("Lower cluster update pod %s, oldResource: %v, newResource: %v",
			new.Name, oldResource, newResource)
	}
	if n.nodeResource.Node == nil {
		return
	}
	cp := n.nodeResource.DeepCopy()
	n.updatedNode <- cp
}

func (n *NodeAdapter) getResourceFromPods(ctx context.Context) *resources.Resource {
	podResource := resources.NewResource()
	pods, err := n.clientPodLister.List(labels.Everything())
	if err != nil {
		return podResource
	}
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodPending && pod.Spec.NodeName != "" ||
			pod.Status.Phase == corev1.PodRunning {
			nodeName := pod.Spec.NodeName
			node, err := n.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				klog.Infof("get node %v failed err: %v", nodeName, err)
				continue
			}
			if node.Spec.Unschedulable || !checkNodeStatusReady(node) {
				continue
			}
			res := GetRequestFromPod(pod)
			res.Pods = resource.MustParse("1")
			podResource.Add(res)
		}
	}
	return podResource
}

func (n *NodeAdapter) getResourceFromPodsByNodeName(nodeName string) *resources.Resource {
	podResource := resources.NewResource()
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + nodeName)
	if err != nil {
		return podResource
	}

	// TODO cache
	pods, err := n.client.CoreV1().Pods(corev1.NamespaceAll).List(context.TODO(),
		metav1.ListOptions{
			FieldSelector: fieldSelector.String(),
		})
	if err != nil {
		return podResource
	}
	for _, pod := range pods.Items {
		pod := pod
		if utils.IsVirtualPod(&pod) {
			continue
		}
		if pod.Status.Phase == corev1.PodPending ||
			pod.Status.Phase == corev1.PodRunning {
			res := GetRequestFromPod(&pod)
			res.Pods = resource.MustParse("1")
			podResource.Add(res)
		}
	}
	return podResource
}
