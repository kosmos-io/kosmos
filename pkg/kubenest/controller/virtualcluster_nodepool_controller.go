package controller

import (
	"context"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NodeLabelsConfigMap = "node-labels-config"
	NodePoolConfigMap   = "node-pool"
	NameSpace           = "kosmos-system"
	ShareState          = "share"
	FreeState           = "free"
)

type NodeInfo struct {
	Address string            `json:"address"`
	Labels  map[string]string `json:"labels"`
	Cluster string            `json:"cluster"`
	State   string            `json:"state"`
}

type VirtualClusterNodePoolController struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
}

func (r *VirtualClusterNodePoolController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	labelsConfig := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Namespace: NameSpace, Name: NodeLabelsConfigMap}, labelsConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	nodePool := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Namespace: NameSpace, Name: NodePoolConfigMap}, nodePool)
	if err != nil {
		return ctrl.Result{}, err
	}

	nodeLabels := labelsConfig.Data

	nodesData := nodePool.Data["nodes"]

	var nodes map[string]NodeInfo
	err = json.Unmarshal([]byte(nodesData), &nodes)
	if err != nil {
		klog.V(4).InfoS("Failed to unmarshal nodes data")
		return ctrl.Result{}, err
	}

	// Iterate through each node in the node pool
	for nodeName, nodeData := range nodes {
		labels := nodeData.Labels

		//only state=free can update
		if nodeData.State == FreeState {
			for key, value := range nodeLabels {
				if labelValue, ok := labels[key]; ok && labelValue == value {
					// If a label in the node matches one of the labels in nodeLabels, update the state to "share"
					nodeData.State = ShareState
					break
				}
			}
			nodes[nodeName] = nodeData
		}
	}

	updatedNodesData, _ := json.Marshal(nodes)
	nodePool.Data["nodes"] = string(updatedNodesData)

	err = r.Update(ctx, nodePool)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VirtualClusterNodePoolController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
			return o.GetNamespace() == NameSpace
		}))).
		WithEventFilter(predicate.Or(predicate.NewPredicateFuncs(func(o client.Object) bool {
			return o.GetName() == NodeLabelsConfigMap || o.GetName() == NodePoolConfigMap
		}))).
		Complete(r)
}
