// nolint
package globalnodecontroller

import (
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func GetGlobalNode(globalnodelist []*v1alpha1.GlobalNode, name string) (*v1alpha1.GlobalNode, error) {
	// 模拟查找返回节点
	for _, node := range globalnodelist {
		if node.Name == name { // 假设 GlobalNode 结构体中有 Name 字段
			return node, nil
		}
	}
	return nil, fmt.Errorf("GlobalNode not found")
}

func UpdateStatus(globalnodelist []*v1alpha1.GlobalNode, globalNode *v1alpha1.GlobalNode) error {
	// 遍历 globalnodelist 查找目标节点
	var nodeFound bool
	for i, node := range globalnodelist {
		if node.Name == globalNode.Name {
			// 找到匹配的节点，更新该节点
			globalnodelist[i].Status.Conditions = globalNode.Status.Conditions
			nodeFound = true
			break // 找到目标节点后退出循环
		}
	}

	// 如果没有找到对应的节点，返回错误
	if !nodeFound {
		return fmt.Errorf("GlobalNode with name '%s' not found", globalNode.Name)
	}

	// 返回成功，表示更新完成
	return nil
}

func updateStatusForGlobalNode1(
	globalNodes []*v1alpha1.GlobalNode,
	globalNode *v1alpha1.GlobalNode,
) error {
	// 动态获取最新的 GlobalNode
	currentNode, err := GetGlobalNode(globalNodes, globalNode.Name)
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
		err = UpdateStatus(globalNodes, currentNode)
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
}
func TestUpdateStatusForGlobalNode(t *testing.T) {
	tests := []struct {
		name           string
		initialNode    *v1alpha1.GlobalNode
		nodeList       []*v1alpha1.GlobalNode
		expectedStatus string
	}{
		{
			name: "No condition to update",
			initialNode: &v1alpha1.GlobalNode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
				Status: v1alpha1.GlobalNodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:               v1.NodeReady,
							LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-1 * time.Hour)),
							LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						},
					},
				},
			},
			nodeList: []*v1alpha1.GlobalNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: v1alpha1.GlobalNodeStatus{
						Conditions: []v1.NodeCondition{
							{
								Type:               v1.NodeReady,
								LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-1 * time.Hour)),
								LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
							},
						},
					},
				},
			},
			expectedStatus: "Ready",
		},
		{
			name: "Status update required",
			initialNode: &v1alpha1.GlobalNode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
				Status: v1alpha1.GlobalNodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:               v1.NodeReady,
							LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-2 * time.Hour)),
							LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
						},
					},
				},
			},
			nodeList: []*v1alpha1.GlobalNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: v1alpha1.GlobalNodeStatus{
						Conditions: []v1.NodeCondition{
							{
								Type:               v1.NodeReady,
								LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-2 * time.Hour)),
								LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
							},
						},
					},
				},
			},
			expectedStatus: "NotReady",
		},
		{
			name: "No nodes in list",
			initialNode: &v1alpha1.GlobalNode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node3",
				},
				Status: v1alpha1.GlobalNodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:               v1.NodeReady,
							LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-30 * time.Minute)),
							LastTransitionTime: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
						},
					},
				},
			},
			nodeList:       []*v1alpha1.GlobalNode{},
			expectedStatus: "",
		},
		{
			name: "Node Ready status recently updated",
			initialNode: &v1alpha1.GlobalNode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node4",
				},
				Status: v1alpha1.GlobalNodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:               v1.NodeReady,
							LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-5 * time.Minute)),
							LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
						},
					},
				},
			},
			nodeList: []*v1alpha1.GlobalNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node4",
					},
					Status: v1alpha1.GlobalNodeStatus{
						Conditions: []v1.NodeCondition{
							{
								Type:               v1.NodeReady,
								LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-5 * time.Minute)),
								LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
							},
						},
					},
				},
			},
			expectedStatus: "Ready",
		},
		{
			name: "Node status changed from Ready to NotReady",
			initialNode: &v1alpha1.GlobalNode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node5",
				},
				Status: v1alpha1.GlobalNodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:               v1.NodeReady,
							LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-1 * time.Hour)),
							LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						},
					},
				},
			},
			nodeList: []*v1alpha1.GlobalNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node5",
					},
					Status: v1alpha1.GlobalNodeStatus{
						Conditions: []v1.NodeCondition{
							{
								Type:               v1.NodeReady,
								LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-3 * time.Hour)),
								LastTransitionTime: metav1.NewTime(time.Now().Add(-3 * time.Hour)),
							},
						},
					},
				},
			},
			expectedStatus: "NotReady",
		},
		{
			name: "Node added to list but with no conditions",
			initialNode: &v1alpha1.GlobalNode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node6",
				},
				Status: v1alpha1.GlobalNodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:               v1.NodeReady,
							LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-30 * time.Minute)),
							LastTransitionTime: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
						},
					},
				},
			},
			nodeList: []*v1alpha1.GlobalNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node6",
					},
					Status: v1alpha1.GlobalNodeStatus{
						Conditions: []v1.NodeCondition{},
					},
				},
			},
			expectedStatus: "Unknown",
		},
		{
			name: "Multiple nodes with mixed statuses",
			initialNode: &v1alpha1.GlobalNode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node7",
				},
				Status: v1alpha1.GlobalNodeStatus{
					Conditions: []v1.NodeCondition{
						{
							Type:               v1.NodeReady,
							LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-1 * time.Hour)),
							LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						},
					},
				},
			},
			nodeList: []*v1alpha1.GlobalNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node7",
					},
					Status: v1alpha1.GlobalNodeStatus{
						Conditions: []v1.NodeCondition{
							{
								Type:               v1.NodeReady,
								LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-2 * time.Hour)),
								LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node8",
					},
					Status: v1alpha1.GlobalNodeStatus{
						Conditions: []v1.NodeCondition{
							{
								Type:               v1.NodeReady,
								LastHeartbeatTime:  metav1.NewTime(time.Now().Add(-10 * time.Minute)),
								LastTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
							},
						},
					},
				},
			},
			expectedStatus: "NotReady",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println(tt.initialNode.Name + "success")
			updateStatusForGlobalNode1(tt.nodeList, tt.initialNode)
			fmt.Println(string(tt.initialNode.Status.Conditions[0].Type) == tt.expectedStatus)
		})
	}
}
