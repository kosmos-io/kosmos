package tasks

import (
	"errors"
	"fmt"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane/etcd"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
	"k8s.io/klog/v2"
)

func NewVirtualClusterEtcdServiceTask() workflow.Task {
	return workflow.Task{
		Name:        "etcd-service",
		Run:         runEtcdService,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-etcd-service",
				Run:  runVirtualClusterEtcdService,
			},
		},
	}
}

func runEtcdService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("etcd-service task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[etcd-service] Running etcd-service task", "virtual cluster", klog.KObj(data))
	return nil
}

func runVirtualClusterEtcdService(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("VirtualClusterEtcd task invoked with an invalid data struct")
	}

	err := etcd.EnsureVirtualClusterEtcdService(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster etcd-service , err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterEtcdService] Successfully installed virtualCluster-etcd service", "virtual cluster", klog.KObj(data))
	return nil
}
