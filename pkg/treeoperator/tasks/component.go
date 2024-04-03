package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/controlplane"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

const (
	KubeControllerManagerComponent   = "KubeControllerManager"
	VirtualClusterSchedulerComponent = "VirtualClusterScheduler"
)

func NewComponentTask() workflow.Task {
	return workflow.Task{
		Name:        "components",
		Run:         runComponents,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newComponentSubTask(KubeControllerManagerComponent),
			newComponentSubTask(VirtualClusterSchedulerComponent),
		},
	}
}

func runComponents(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("components task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[components] Running components task", "virtual cluster", klog.KObj(data))
	return nil
}

func newComponentSubTask(component string) workflow.Task {
	return workflow.Task{
		Name: component,
		Run:  runComponentSubTask(component),
	}
}

func runComponentSubTask(component string) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("components task invoked with an invalid data struct")
		}

		err := controlplane.EnsureControlPlaneComponent(
			component,
			data.GetName(),
			data.GetNamespace(),
			data.RemoteClient(),
		)
		if err != nil {
			return fmt.Errorf("failed to apply component %s, err: %w", component, err)
		}

		klog.V(2).InfoS("[components] Successfully applied component", "component", component, "virtual cluster", klog.KObj(data))
		return nil
	}
}
