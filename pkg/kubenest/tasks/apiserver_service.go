package tasks

import (
	"errors"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane/apiserver"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewVirtualClusterApiserverServiceTask() workflow.Task {
	return workflow.Task{
		Name:        "apiserver-service",
		Run:         runApiserverService,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-apiserver-service",
				Run:  runVirtualClusterAPIServerService,
			},
		},
	}
}

func runApiserverService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("apiserver-service task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[apiserver-service] Running apiserver-service task", "virtual cluster", klog.KObj(data))
	return nil
}

func runVirtualClusterAPIServerService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("VirtualClusterApiserver task invoked with an invalid data struct")
	}

	err := apiserver.EnsureVirtualClusterAPIServerService(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver-service , err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterApiserverService] Successfully installed virtualCluster-apiserver service", "virtual cluster", klog.KObj(data))
	return nil
}
