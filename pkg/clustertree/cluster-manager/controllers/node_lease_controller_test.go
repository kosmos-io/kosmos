package controllers

import (
	"context"
	"sync"
	"testing"
	"time"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNodeLeaseController_updatepodStatus(t *testing.T) {
	type fields struct {
		leafClient        kubernetes.Interface
		rootClient        kubernetes.Interface
		root              client.Client
		LeafModelHandler  leafUtils.LeafModelHandler
		leaseInterval     time.Duration
		statusInterval    time.Duration
		podstatusInterval time.Duration
		nodes             []*corev1.Node
		LeafNodeSelectors map[string]kosmosv1alpha1.NodeSelector
		nodeLock          sync.Mutex
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &NodeLeaseController{
				leafClient:        tt.fields.leafClient,
				rootClient:        tt.fields.rootClient,
				root:              tt.fields.root,
				LeafModelHandler:  tt.fields.LeafModelHandler,
				leaseInterval:     tt.fields.leaseInterval,
				statusInterval:    tt.fields.statusInterval,
				podstatusInterval: tt.fields.podstatusInterval,
				nodes:             tt.fields.nodes,
				LeafNodeSelectors: tt.fields.LeafNodeSelectors,
				nodeLock:          tt.fields.nodeLock,
			}
			if err := c.updatepodStatus(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("NodeLeaseController.updatepodStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
