package treeoperator

import (
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	JoinControllerName = "virtual-cluster-join-controller"
)

type VirtualClusterJoinController struct {
	client.Client
	EventRecorder record.EventRecorder
}

func (c *VirtualClusterJoinController) SetupWithManager(mgr manager.Manager) error {
	return nil
}
