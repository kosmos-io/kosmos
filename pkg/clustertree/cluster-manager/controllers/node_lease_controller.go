package controllers

import (
	"context"
	"fmt"
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

	"github.com/kosmos.io/kosmos/pkg/utils"
)

const (
	NodeLeaseControllerName = "node-lease-controller"

	DefaultLeaseDuration         = 40
	DefaultRenewIntervalFraction = 0.25

	DefaultNodeStatusUpdateInterval = 1 * time.Minute
)

type NodeLeaseController struct {
	leafClient kubernetes.Interface
	rootClient kubernetes.Interface
	root       client.Client

	leaseInterval  time.Duration
	statusInterval time.Duration

	node     *corev1.Node
	nodeLock sync.Mutex
}

func NewNodeLeaseController(leafClient kubernetes.Interface, root client.Client, node *corev1.Node, rootClient kubernetes.Interface) *NodeLeaseController {
	c := &NodeLeaseController{
		leafClient:     leafClient,
		rootClient:     rootClient,
		root:           root,
		node:           node,
		leaseInterval:  getRenewInterval(),
		statusInterval: DefaultNodeStatusUpdateInterval,
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
	c.nodeLock.Lock()
	node := c.node.DeepCopy()
	c.nodeLock.Unlock()

	err := c.updateNodeStatus(ctx, node)
	if err != nil {
		klog.Errorf(err.Error())
	}
}

func (c *NodeLeaseController) updateNodeStatus(ctx context.Context, n *corev1.Node) error {
	node := &corev1.Node{}
	namespacedName := types.NamespacedName{
		Name: n.Name,
	}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := c.root.Get(ctx, namespacedName, node)
		if err != nil {
			// TODO: If a node is accidentally deleted, recreate it
			return fmt.Errorf("cannot get node while update node status %s, err: %v", n.Name, err)
		}

		clone := node.DeepCopy()
		clone.Status.Conditions = utils.NodeConditions()

		patch, err := utils.CreateMergePatch(node, clone)
		if err != nil {
			return fmt.Errorf("cannot get node while update node status %s, err: %v", node.Name, err)
		}

		if node, err = c.rootClient.CoreV1().Nodes().PatchStatus(ctx, node.Name, patch); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (c *NodeLeaseController) syncLease(ctx context.Context) {
	c.nodeLock.Lock()
	node := c.node.DeepCopy()
	c.nodeLock.Unlock()

	_, err := c.leafClient.Discovery().ServerVersion()
	if err != nil {
		klog.Errorf("failed to ping leaf %s", c.node.Name)
		return
	}

	_, _, err = c.createLeaseIfNotExists(ctx, node)
	if err != nil {
		return
	}

	err = c.updateLeaseWithRetry(ctx, node)
	if err != nil {
		klog.Errorf("lease has failed, and the maximum number of retries has been reached, %v", err)
		return
	}

	klog.V(4).Infof("Successfully updated lease")
}

func (c *NodeLeaseController) createLeaseIfNotExists(ctx context.Context, node *corev1.Node) (*coordinationv1.Lease, bool, error) {
	namespaceName := types.NamespacedName{
		Namespace: corev1.NamespaceNodeLease,
		Name:      node.Name,
	}
	lease := &coordinationv1.Lease{}
	err := c.root.Get(ctx, namespaceName, lease)
	if apierrors.IsNotFound(err) {
		leaseToCreate := c.newLease(node)
		err := c.root.Create(ctx, leaseToCreate)
		if err != nil {
			klog.Errorf("create lease %s failed", node.Name)
			return nil, false, err
		}
		return lease, true, nil
	} else if err != nil {
		klog.Errorf("get lease %s failed", node.Name, err)
		return nil, false, err
	}
	return lease, true, nil
}

func (c *NodeLeaseController) updateLeaseWithRetry(ctx context.Context, node *corev1.Node) error {
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
	return err
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
