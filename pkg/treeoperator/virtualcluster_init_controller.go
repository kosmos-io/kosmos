package treeoperator

import (
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	InitControllerName = "virtual-cluster-init-controller"
)

type VirtualClusterInitController struct {
	client.Client
	EventRecorder record.EventRecorder
}

func (c *VirtualClusterInitController) SetupWithManager(mgr manager.Manager) error {
	return nil
}
