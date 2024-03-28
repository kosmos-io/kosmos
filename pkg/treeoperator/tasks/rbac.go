package tasks

import (
	"errors"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/resource/rbac"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

func NewRBACTask() workflow.Task {
	return workflow.Task{
		Name: "rbac",
		Run:  runRBAC,
	}
}

func runRBAC(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("RBAC task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[RBAC] Running rbac task", "Virtual cluster", klog.KObj(data))

	return rbac.EnsureVirtualSchedulerRBAC(data.RemoteClient(), data.GetNamespace())
}
