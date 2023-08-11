package elector

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

func TestEnsureGateWayRole(t *testing.T) {
	elector := &Elector{
		nodeName:    "node1",
		clusterName: "cluster1",
	}
	var tests = []struct {
		name              string
		inputClusterNodes []v1alpha1.ClusterNode
		inputCluster      *v1alpha1.Cluster
		want              []v1alpha1.ClusterNode
	}{
		{
			name: "do nothing with clusterNode in other cluster",
			inputClusterNodes: []v1alpha1.ClusterNode{
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster2-node1",
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster2",
						NodeName:    "node1",
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster2-node2",
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster2",
						NodeName:    "node2",
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster2-node3",
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster2",
						Roles:       []v1alpha1.Role{v1alpha1.RoleInternal},
						NodeName:    "node3",
					},
				},
			},
			want: []v1alpha1.ClusterNode{},
		},
		{
			name: "add gateway role to current node with nil roles",
			inputClusterNodes: []v1alpha1.ClusterNode{
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, elector.nodeName),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: elector.clusterName,
						NodeName:    elector.nodeName,
					},
				},
			},
			want: []v1alpha1.ClusterNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, elector.nodeName),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: elector.clusterName,
						NodeName:    elector.nodeName,
						Roles:       []v1alpha1.Role{v1alpha1.RoleGateway},
					},
				},
			},
		},
		{
			name: "add gateway role to current node",
			inputClusterNodes: []v1alpha1.ClusterNode{
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, elector.nodeName),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: elector.clusterName,
						NodeName:    elector.nodeName,
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, "node2"),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster1",
						NodeName:    "node2",
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1-node3",
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster1",
						NodeName:    "node3",
						Roles:       []v1alpha1.Role{v1alpha1.RoleInternal},
					},
				},
			},
			want: []v1alpha1.ClusterNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, elector.nodeName),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: elector.clusterName,
						NodeName:    elector.nodeName,
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint, v1alpha1.RoleGateway},
					},
				},
			},
		},
		{
			name: "add gateway role to current node, and remove gateway role from other node in the same cluster",
			inputClusterNodes: []v1alpha1.ClusterNode{
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, elector.nodeName),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: elector.clusterName,
						NodeName:    elector.nodeName,
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, "node2"),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster1",
						NodeName:    "node2",
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint, v1alpha1.RoleGateway},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1-node3",
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster1",
						NodeName:    "node3",
						Roles:       []v1alpha1.Role{v1alpha1.RoleInternal, v1alpha1.RoleGateway},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster2-node1",
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster2",
						NodeName:    "node1",
						Roles:       []v1alpha1.Role{v1alpha1.RoleInternal},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster2-node2",
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster2",
						NodeName:    "node2",
						Roles:       []v1alpha1.Role{v1alpha1.RoleInternal, v1alpha1.RoleGateway},
					},
				},
			},
			want: []v1alpha1.ClusterNode{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, elector.nodeName),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: elector.clusterName,
						NodeName:    elector.nodeName,
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint, v1alpha1.RoleGateway},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, "node2"),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster1",
						NodeName:    "node2",
						Roles:       []v1alpha1.Role{v1alpha1.RoleEndpoint},
					},
				},
				{

					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-%s", elector.clusterName, "node3"),
					},
					Spec: v1alpha1.ClusterNodeSpec{
						ClusterName: "cluster1",
						NodeName:    "node3",
						Roles:       []v1alpha1.Role{v1alpha1.RoleInternal},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := elector.genModifyNode(tt.inputClusterNodes)
			if !isClusterNodesEqual(target, tt.want) {
				t.Errorf("genModifyNode()=%v, want %v", target, tt.want)
			}
		})
	}
}

func isClusterNodesEqual(target, want []v1alpha1.ClusterNode) bool {
	if len(target) != len(want) {
		return false
	}

	sort.Slice(target, func(i, j int) bool {
		return target[i].Name < target[j].Name
	})

	sort.Slice(want, func(i, j int) bool {
		return want[i].Name < want[j].Name
	})

	for i := range target {
		t := target[i]
		w := want[i]
		if t.Name != w.Name || len(t.Spec.Roles) != len(w.Spec.Roles) {
			return false
		}
		sort.Slice(t.Spec.Roles, func(i, j int) bool {
			return t.Spec.Roles[i] < t.Spec.Roles[j]
		})
		sort.Slice(w.Spec.Roles, func(i, j int) bool {
			return w.Spec.Roles[i] < w.Spec.Roles[j]
		})
		if !reflect.DeepEqual(t.Spec.Roles, w.Spec.Roles) {
			return false
		}
	}
	return true
}
