package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewVirtualClusterServiceTask() workflow.Task {
	return workflow.Task{
		Name:        "virtual-service",
		Run:         runService,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "virtual-service",
				Run:  runVirtualClusterService,
			},
		},
	}
}

func runService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("service task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[service] Running service task", "virtual cluster", klog.KObj(data))
	return nil
}

func runVirtualClusterService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual service task invoked with an invalid data struct")
	}

	err := controlplane.EnsureVirtualClusterService(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
		data.HostPortMap(),
		data.KubeNestOpt(),
		data.VirtualCluster(),
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster service , err: %w", err)
	}

	klog.V(2).InfoS("[Virtual Cluster Service] Successfully installed virtual cluster service", "virtual cluster", klog.KObj(data))
	return nil
}

func UninstallVirtualClusterServiceTask() workflow.Task {
	return workflow.Task{
		Name:        "virtual-service",
		Run:         runService,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "virtual-service",
				Run:  uninstallVirtualClusterService,
			},
		},
	}
}

func uninstallVirtualClusterService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual service task invoked with an invalid data struct")
	}

	err := controlplane.DeleteVirtualClusterService(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf("failed to uninstall virtual cluster service , err: %w", err)
	}

	klog.V(2).Infof("[Virtual Cluster Service] Successfully uninstalled virtual cluster %s service", data.GetName())
	return nil
}
