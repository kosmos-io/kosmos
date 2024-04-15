package vcnodecontroller

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	vcrnodepoolcontroller "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.nodepool.controller"
)

// TODO: biz
func (r *NodeController) UpdateNodePoolState(ctx context.Context, nodeName string, nodePoolState string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nodePool := v1.ConfigMap{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: NodePoolCMName, Namespace: NodePoolCMNS}, &nodePool); err != nil {
			return err
		}

		updateNodePool := nodePool.DeepCopy()

		yamlStr := updateNodePool.Data[NodePoolCMKeyName]
		nodePoolItem, err := vcrnodepoolcontroller.ConvertYamlToNodePoolItem(yamlStr)
		if err != nil {
			return err
		}

		targetNodePoolItem := nodePoolItem[nodeName]
		targetNodePoolItem.State = nodePoolState

		nodePoolItem[nodeName] = targetNodePoolItem

		nodePoolBytes, err := vcrnodepoolcontroller.ConvertNodePoolItemToYaml(nodePoolItem)
		if err != nil {
			return err
		}

		updateNodePool.Data[NodePoolCMKeyName] = string(nodePoolBytes)

		if err := r.Client.Update(ctx, updateNodePool); err != nil {
			return err
		}

		return nil
	})

	return err
}
