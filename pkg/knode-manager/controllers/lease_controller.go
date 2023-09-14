package controllers

import (
	"context"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	coordclientset "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

const (
	DefaultLeaseDuration         = 40
	DefaultRenewIntervalFraction = 0.25
)

type leaseController struct {
	leaseClient coordclientset.LeaseInterface

	renewInterval  time.Duration
	nodeController *NodeController
}

func newLeaseController(client kubernetes.Interface, nodeController *NodeController) *leaseController {
	c := &leaseController{
		leaseClient:    client.CoordinationV1().Leases(corev1.NamespaceNodeLease),
		renewInterval:  getRenewInterval(),
		nodeController: nodeController,
	}
	return c
}

func (c *leaseController) Run(ctx context.Context) {
	c.sync(ctx)
	wait.UntilWithContext(ctx, c.sync, c.renewInterval)
}

func (c *leaseController) sync(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	result, err := c.nodeController.nodeProbeController.probe(ctx)
	if err != nil || result.error != nil {
		klog.Errorf("could not get ping status")
		return
	}

	dNode, err := c.nodeController.getDummyNode()
	if err != nil {
		klog.Error("could not get dummy node")
		return
	}

	lease, err := c.createLeaseIfNotExistsWithRetry(ctx, dNode)
	if err != nil {
		klog.Errorf("lease creation has failed, and the maximum number of retries has been reached, %v", err)
		return
	}

	err = c.updateLeaseWithRetry(ctx, dNode, lease)
	if err != nil {
		klog.Errorf("lease has failed, and the maximum number of retries has been reached, %v", err)
		return
	}

	klog.Infof("Successfully updated lease")
}

func (c *leaseController) createLeaseIfNotExistsWithRetry(ctx context.Context, node *corev1.Node) (*coordinationv1.Lease, error) {
	var lease *coordinationv1.Lease
	err := retry.OnError(retry.DefaultBackoff, func(e error) bool {
		return e != nil
	}, func() error {
		r, _, err := c.createLeaseIfNotExists(ctx, node)
		if err != nil {
			return err
		}
		lease = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return lease, nil
}

func (c *leaseController) createLeaseIfNotExists(ctx context.Context, node *corev1.Node) (*coordinationv1.Lease, bool, error) {
	lease, err := c.leaseClient.Get(ctx, node.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		leaseToCreate := c.newLease(node, nil)
		lease, err := c.leaseClient.Create(ctx, leaseToCreate, metav1.CreateOptions{})
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

func (c *leaseController) updateLeaseWithRetry(ctx context.Context, node *corev1.Node, base *coordinationv1.Lease) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		lease := c.newLease(node, base)
		_, err := c.leaseClient.Update(ctx, lease, metav1.UpdateOptions{})
		if err != nil {
			klog.Warningf("update lease %s failed with err %v", node.Name, err)
			return err
		}
		return nil
	})
	return err
}

func (c *leaseController) newLease(node *corev1.Node, base *coordinationv1.Lease) *coordinationv1.Lease {
	var lease *coordinationv1.Lease
	if base == nil {
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      node.Name,
				Namespace: corev1.NamespaceNodeLease,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       pointer.String(node.Name),
				LeaseDurationSeconds: pointer.Int32(DefaultLeaseDuration),
			},
		}
	} else {
		lease = base.DeepCopy()
	}
	lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
	if len(lease.OwnerReferences) == 0 {
		lease.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
				Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
				Name:       node.Name,
				UID:        node.UID,
			},
		}
	}
	return lease
}

func getRenewInterval() time.Duration {
	interval := DefaultLeaseDuration * DefaultRenewIntervalFraction
	intervalDuration := time.Second * time.Duration(int(interval))
	return intervalDuration
}
