package globalnodecontroller

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosfake "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned/fake"
)

// nolint
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
			expectedStatus: "NotReady",
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

	ctx := context.TODO()

	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	rootClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	kosmosclient := kosmosfake.NewSimpleClientset()

	controller := NewGlobalNodeStatusController(
		rootClient,
		kosmosclient,
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println(tt.initialNode.Name + "success")
			for _, node := range tt.nodeList {
				_, err := kosmosclient.KosmosV1alpha1().GlobalNodes().Create(ctx, node, metav1.CreateOptions{})
				if err != nil {
					return
				}
			}
			err := controller.updateStatusForGlobalNode(ctx, tt.initialNode)
			if err != nil {
				return
			}
			fmt.Println(string(tt.initialNode.Status.Conditions[0].Type) == tt.expectedStatus)
		})
	}
}
