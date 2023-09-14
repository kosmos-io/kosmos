package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
)

const (
	DefaultStatusUpdateInterval = 1 * time.Minute

	LastNodeAppliedNodeStatus = "kosmos.io/last-applied-node-status"
	LastNodeAppliedObjectMeta = "kosmos.io/last-applied-object-meta"
)

type NodeController struct {
	adapter adapters.NodeHandler
	client  v1.NodeInterface

	dummyNode     *corev1.Node
	dummyNodeLock sync.Mutex

	statusInterval   time.Duration
	statusUpdateChan chan *corev1.Node

	nodeProbeController *nodeProbeController
	leaseController     *leaseController

	group wait.Group
}

func (n *NodeController) getDummyNode() (*corev1.Node, error) {
	n.dummyNodeLock.Lock()
	defer n.dummyNodeLock.Unlock()
	if n.dummyNode == nil {
		return nil, fmt.Errorf("server node does not yet exist")
	}
	return n.dummyNode.DeepCopy(), nil
}

func BuildDummyNode(ctx context.Context, knode *kosmosv1alpha1.Knode, a adapters.NodeHandler) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: knode.Spec.NodeName,
			Labels: map[string]string{
				utils.KosmosNodeLabel:   utils.KosmosNodeValue,
				utils.NodeRoleLabel:     utils.NodeRoleValue,
				utils.NodeHostnameValue: knode.Spec.NodeName,
			},
		},
		Spec: corev1.NodeSpec{},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{},
		},
	}

	// TODO label & taints
	a.Configure(ctx, node)
	return node
}

func NewNodeController(adapter adapters.NodeHandler, client kubernetes.Interface, dummyNode *corev1.Node) (*NodeController, error) {
	n := &NodeController{
		adapter:        adapter,
		client:         client.CoreV1().Nodes(),
		dummyNode:      dummyNode,
		statusInterval: DefaultStatusUpdateInterval,
	}

	n.nodeProbeController = newNodeProbeController(n.adapter)
	n.leaseController = newLeaseController(client, n)

	return n, nil
}

func (n *NodeController) Run(ctx context.Context) error {
	err := n.applyNode(ctx)
	if err != nil {
		return err
	}

	n.statusUpdateChan = make(chan *corev1.Node, 1)
	n.adapter.NotifyStatus(ctx, func(node *corev1.Node) {
		n.statusUpdateChan <- node
	})

	n.group.StartWithContext(ctx, n.nodeProbeController.Run)
	n.group.StartWithContext(ctx, n.leaseController.Run)

	return n.sync(ctx)
}

func (n *NodeController) sync(ctx context.Context) error {
	defer n.group.Wait()

	dummyNode, _ := n.getDummyNode()

	loop := func() bool {
		timer := time.NewTimer(n.statusInterval)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return true
		case updated := <-n.statusUpdateChan:
			klog.Infof("Received node %s status update", updated.Name)

			dummyNode.Status = updated.Status
			dummyNode.ObjectMeta.Annotations = updated.Annotations
			dummyNode.ObjectMeta.Labels = updated.Labels
			if err := n.updateStatus(ctx, dummyNode); err != nil {
				klog.Errorf("failed to update node status, %v", err)
			}
		case <-timer.C:
			if err := n.updateStatus(ctx, dummyNode); err != nil {
				klog.Errorf("failed to update node status, %v", err)
			}
		}
		return false
	}

	for {
		shouldTerminate := loop()
		if shouldTerminate {
			return nil
		}
	}
}

func (n *NodeController) applyNode(ctx context.Context) error {
	n.dummyNodeLock.Lock()
	dNode := n.dummyNode
	n.dummyNodeLock.Unlock()

	err := n.updateStatus(ctx, dNode)
	if err == nil || !errors.IsNotFound(err) {
		return err
	}

	newNode, err := n.client.Create(ctx, dNode, metav1.CreateOptions{})
	if err != nil {
		return pkgerrors.Wrap(err, "error registering node with kubernetes")
	}

	n.dummyNodeLock.Lock()
	n.dummyNode = newNode
	n.dummyNodeLock.Unlock()

	return nil
}

func (n *NodeController) updateStatus(ctx context.Context, node *corev1.Node) (err error) {
	result, err := n.nodeProbeController.probe(ctx)
	if err != nil {
		return err
	} else if result.error != nil {
		return fmt.Errorf("node probe failed: %w", result.error)
	}

	updateNodeStatusHeartbeat(node)

	var updatedNode *corev1.Node
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		apiServerNode, err := n.client.Get(ctx, node.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		patchBytes, err := prepareThreeWayPatchBytesForNodeStatus(node, apiServerNode)
		if err != nil {
			return fmt.Errorf("cannot generate patch %v", err)
		}
		updatedNode, err = n.client.Patch(ctx, node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		if err != nil {
			klog.Errorf("failed to patch node status, %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	n.dummyNodeLock.Lock()
	n.dummyNode = updatedNode
	n.dummyNodeLock.Unlock()
	return nil
}

func updateNodeStatusHeartbeat(n *corev1.Node) {
	now := metav1.NewTime(time.Now())
	for i := range n.Status.Conditions {
		n.Status.Conditions[i].LastHeartbeatTime = now
	}
}

func prepareThreeWayPatchBytesForNodeStatus(nodeFromProvider, apiServerNode *corev1.Node) ([]byte, error) {
	oldVKStatus, ok1 := apiServerNode.Annotations[LastNodeAppliedNodeStatus]
	oldVKObjectMeta, ok2 := apiServerNode.Annotations[LastNodeAppliedObjectMeta]

	oldNode := corev1.Node{}
	if ok1 && ok2 {
		err := json.Unmarshal([]byte(oldVKObjectMeta), &oldNode.ObjectMeta)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "Cannot unmarshal old node object metadata (key: %q): %q", LastNodeAppliedObjectMeta, oldVKObjectMeta)
		}
		err = json.Unmarshal([]byte(oldVKStatus), &oldNode.Status)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "Cannot unmarshal old node status (key: %q): %q", LastNodeAppliedNodeStatus, oldVKStatus)
		}
	}

	newNode := corev1.Node{}
	newNode.ObjectMeta = simplestObjectMetadata(&apiServerNode.ObjectMeta, &nodeFromProvider.ObjectMeta)
	nodeFromProvider.Status.DeepCopyInto(&newNode.Status)

	LastNodeAppliedObjectMetaBytes, err := json.Marshal(newNode.ObjectMeta)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal object meta from provider")
	}
	newNode.Annotations[LastNodeAppliedObjectMeta] = string(LastNodeAppliedObjectMetaBytes)

	LastNodeAppliedNodeStatusBytes, err := json.Marshal(newNode.Status)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal node status from provider")
	}
	newNode.Annotations[LastNodeAppliedNodeStatus] = string(LastNodeAppliedNodeStatusBytes)
	oldNodeBytes, err := json.Marshal(oldNode)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal old node bytes")
	}
	newNodeBytes, err := json.Marshal(newNode)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal new node bytes")
	}
	apiServerNodeBytes, err := json.Marshal(apiServerNode)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal api server node")
	}
	schema, err := strategicpatch.NewPatchMetaFromStruct(&corev1.Node{})
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot get patch schema from node")
	}
	return strategicpatch.CreateThreeWayMergePatch(oldNodeBytes, newNodeBytes, apiServerNodeBytes, schema, true)
}

func simplestObjectMetadata(baseObjectMeta, objectMetaWithLabelsAndAnnotations *metav1.ObjectMeta) metav1.ObjectMeta {
	ret := metav1.ObjectMeta{
		Namespace:   baseObjectMeta.Namespace,
		Name:        baseObjectMeta.Name,
		UID:         baseObjectMeta.UID,
		Annotations: make(map[string]string),
	}
	if objectMetaWithLabelsAndAnnotations != nil {
		if objectMetaWithLabelsAndAnnotations.Labels != nil {
			ret.Labels = objectMetaWithLabelsAndAnnotations.Labels
		} else {
			ret.Labels = make(map[string]string)
		}
		if objectMetaWithLabelsAndAnnotations.Annotations != nil {
			for key := range objectMetaWithLabelsAndAnnotations.Annotations {
				if key == LastNodeAppliedNodeStatus || key == LastNodeAppliedObjectMeta {
					continue
				}
				ret.Annotations[key] = objectMetaWithLabelsAndAnnotations.Annotations[key]
			}
		}
	}
	return ret
}
