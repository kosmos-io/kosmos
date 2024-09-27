package controllers

import (
	"context"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
)

const (
	NodeLeaseControllerName = "node-lease-controller"

	DefaultLeaseDuration         = 40
	DefaultRenewIntervalFraction = 0.25

	DefaultNodeStatusUpdateInterval = 1 * time.Minute
)

type NodeLeaseController struct {
	leafClient       kubernetes.Interface
	rootClient       kubernetes.Interface
	root             client.Client
	LeafModelHandler leafUtils.LeafModelHandler

	leaseInterval  time.Duration
	statusInterval time.Duration

	nodes             []*corev1.Node
	LeafNodeSelectors map[string]kosmosv1alpha1.NodeSelector
	nodeLock          sync.Mutex
}

func NewNodeLeaseController(leafClient kubernetes.Interface, root client.Client, nodes []*corev1.Node, LeafNodeSelectors map[string]kosmosv1alpha1.NodeSelector, rootClient kubernetes.Interface, LeafModelHandler leafUtils.LeafModelHandler) *NodeLeaseController {
	c := &NodeLeaseController{
		leafClient:        leafClient,
		rootClient:        rootClient,
		root:              root,
		nodes:             nodes,
		LeafModelHandler:  LeafModelHandler,
		LeafNodeSelectors: LeafNodeSelectors,
		leaseInterval:     getRenewInterval(),
		statusInterval:    DefaultNodeStatusUpdateInterval,
	}
	return c
}

func (c *NodeLeaseController) Start(ctx context.Context) error {
	go wait.UntilWithContext(ctx, c.syncLease, c.leaseInterval)
	go wait.UntilWithContext(ctx, c.syncNodeStatus, c.statusInterval)
	<-ctx.Done()
	return nil
}

func (c *NodeLeaseController) syncNodeStatus(ctx context.Context) {
	klog.V(4).Infof("NODESYNC syncNodeStatus start")
	defer klog.V(4).Infof("NODESYNC syncNodeStatus done")
	nodes := make([]*corev1.Node, 0)
	c.nodeLock.Lock()
	for _, nodeIndex := range c.nodes {
		nodeCopy := nodeIndex.DeepCopy()
		nodes = append(nodes, nodeCopy)
	}
	c.nodeLock.Unlock()

	err := c.updateNodeStatus(ctx, nodes, c.LeafNodeSelectors)
	if err != nil {
		klog.Errorf(err.Error())
	}
}

// nolint
func (c *NodeLeaseController) updateNodeStatus(ctx context.Context, n []*corev1.Node, leafNodeSelector map[string]kosmosv1alpha1.NodeSelector) error {
	err := c.LeafModelHandler.UpdateRootNodeStatus(ctx, n, leafNodeSelector)
	if err != nil {
		klog.Errorf("Could not update node status in root cluster,Error: %v", err)
	}
	return nil
}

func (c *NodeLeaseController) syncLease(ctx context.Context) {
	klog.V(4).Infof("NODESYNC syncLease start")
	defer klog.V(4).Infof("NODESYNC syncLease done")
	nodes := make([]*corev1.Node, 0)
	c.nodeLock.Lock()
	for _, nodeIndex := range c.nodes {
		nodeCopy := nodeIndex.DeepCopy()
		nodes = append(nodes, nodeCopy)
	}
	c.nodeLock.Unlock()

	_, err := c.leafClient.Discovery().ServerVersion()
	if err != nil {
		klog.Errorf("failed to ping leaf cluster")
		return
	}

	err = c.createLeaseIfNotExists(ctx, nodes)
	if err != nil {
		return
	}

	err = c.updateLeaseWithRetry(ctx, nodes)
	if err != nil {
		klog.Errorf("lease has failed, and the maximum number of retries has been reached, %v", err)
		return
	}

	klog.V(4).Infof("Successfully updated lease")
}

func (c *NodeLeaseController) createLeaseIfNotExists(ctx context.Context, nodes []*corev1.Node) error {
	for _, node := range nodes {
		namespaceName := types.NamespacedName{
			Namespace: corev1.NamespaceNodeLease,
			Name:      node.Name,
		}
		lease := &coordinationv1.Lease{}
		err := c.root.Get(ctx, namespaceName, lease)
		if err != nil {
			if apierrors.IsNotFound(err) {
				leaseToCreate := c.newLease(node)
				err = c.root.Create(ctx, leaseToCreate)
				if err != nil {
					klog.Errorf("create lease %s failed", node.Name)
					return err
				}
			} else {
				klog.Errorf("get lease %s failed", node.Name, err)
				return err
			}
		}
	}
	return nil
}

func (c *NodeLeaseController) updateLeaseWithRetry(ctx context.Context, nodes []*corev1.Node) error {
	for _, node := range nodes {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			lease := &coordinationv1.Lease{}
			namespaceName := types.NamespacedName{
				Namespace: corev1.NamespaceNodeLease,
				Name:      node.Name,
			}
			if err := c.root.Get(ctx, namespaceName, lease); err != nil {
				klog.Warningf("get lease %s failed with err %v", node.Name, err)
				return err
			}

			lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
			lease.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
					Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
					Name:       node.Name,
					UID:        node.UID,
				},
			}
			err := c.root.Update(ctx, lease)
			if err != nil {
				klog.Warningf("update lease %s failed with err %v", node.Name, err)
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *NodeLeaseController) newLease(node *corev1.Node) *coordinationv1.Lease {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: corev1.NamespaceNodeLease,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
					Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
					Name:       node.Name,
					UID:        node.UID,
				},
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       pointer.String(node.Name),
			LeaseDurationSeconds: pointer.Int32(DefaultLeaseDuration),
			RenewTime:            &metav1.MicroTime{Time: time.Now()},
		},
	}
	return lease
}

func getRenewInterval() time.Duration {
	interval := DefaultLeaseDuration * DefaultRenewIntervalFraction
	intervalDuration := time.Second * time.Duration(int(interval))
	return intervalDuration
}
