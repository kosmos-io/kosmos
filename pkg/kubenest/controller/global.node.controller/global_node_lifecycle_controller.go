package globalnodecontroller

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

const (
	GlobalNodeStatusControllerName = "global-node-status-controller"
	DefaultStatusUpdateInterval    = 15 * time.Second
	ClientHeartbeatThreshold       = 10 * time.Second
)

type GlobalNodeStatusController struct {
	root           client.Client
	statusInterval time.Duration

	kosmosClient versioned.Interface
}

func NewGlobalNodeStatusController(
	root client.Client,
	kosmosClient versioned.Interface,
) *GlobalNodeStatusController {
	return &GlobalNodeStatusController{
		root:           root,
		statusInterval: DefaultStatusUpdateInterval,
		kosmosClient:   kosmosClient,
	}
}
func (c *GlobalNodeStatusController) Start(ctx context.Context) error {
	// 启动状态同步
	go wait.UntilWithContext(ctx, c.syncGlobalNodeStatus, c.statusInterval)

	// 等待上下文完成
	<-ctx.Done()
	return nil
}
func (c *GlobalNodeStatusController) syncGlobalNodeStatus(ctx context.Context) {
	// 动态获取 GlobalNode 列表
	globalNodes := make([]*v1alpha1.GlobalNode, 0)
	//c.globalNodeLock.Lock()
	//defer c.globalNodeLock.Unlock()

	// 获取 GlobalNode 对象列表
	nodeList, err := c.kosmosClient.KosmosV1alpha1().GlobalNodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to fetch GlobalNodes: %v", err)
		return
	}

	// 克隆并存储节点副本
	for _, node := range nodeList.Items {
		nodeCopy := node.DeepCopy()
		globalNodes = append(globalNodes, nodeCopy)
	}

	// 执行状态更新
	err = c.updateGlobalNodeStatus(ctx, globalNodes)
	if err != nil {
		klog.Errorf("Failed to sync global node status: %v", err)
	}
}

// updateGlobalNodeStatus 更新 GlobalNode 的状态
func (c *GlobalNodeStatusController) updateGlobalNodeStatus(
	ctx context.Context,
	globalNodes []*v1alpha1.GlobalNode,
) error {
	for _, globalNode := range globalNodes {
		// 获取或创建更新状态的逻辑
		err := c.updateStatusForGlobalNode(ctx, globalNode)
		if err != nil {
			klog.Errorf("Failed to update status for global node %s: %v", globalNode.Name, err)
			return err
		}
	}
	return nil
}

// updateStatusForGlobalNode 更新单个 GlobalNode 的状态
func (c *GlobalNodeStatusController) updateStatusForGlobalNode(
	ctx context.Context,
	globalNode *v1alpha1.GlobalNode,
) error {
	// 使用 retry 重试机制
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// 动态获取最新的 GlobalNode
		currentNode, err := c.kosmosClient.KosmosV1alpha1().GlobalNodes().Get(ctx, globalNode.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to fetch the latest GlobalNode %s: %v", globalNode.Name, err)
			return err
		}

		// 确保 Status.Conditions 不为空
		if len(currentNode.Status.Conditions) == 0 {
			klog.Warningf("GlobalNode %s has no conditions, skipping status update", currentNode.Name)
			return nil
		}

		// 获取 LastHeartbeatTime
		condition := currentNode.Status.Conditions[0]
		lastHeartbeatTime := condition.LastHeartbeatTime
		timeDiff := time.Since(lastHeartbeatTime.Time)

		// 更新状态条件
		statusType := "Ready"
		if timeDiff > ClientHeartbeatThreshold {
			statusType = "NotReady"
		}

		// 检查状态是否需要更新
		if string(condition.Type) != statusType {
			condition.Type = v1.NodeConditionType(statusType)
			condition.LastTransitionTime = metav1.NewTime(time.Now())

			currentNode.Status.Conditions[0] = condition

			// 提交状态更新
			_, err = c.kosmosClient.KosmosV1alpha1().GlobalNodes().UpdateStatus(ctx, currentNode, metav1.UpdateOptions{})
			if err != nil {
				if errors.IsConflict(err) {
					klog.Warningf("Conflict detected while updating status for GlobalNode %s, retrying...", globalNode.Name)
				} else {
					klog.Errorf("Failed to update status for GlobalNode %s: %v", globalNode.Name, err)
				}
				return err
			}

			klog.Infof("Successfully updated status for GlobalNode %s to %s", globalNode.Name, statusType)
		} else {
			klog.Infof("No status update required for GlobalNode %s, current status: %s", globalNode.Name, condition.Type)
		}

		return nil
	})
}
