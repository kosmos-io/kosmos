package tasks

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

var (
	etcdLabels = labels.Set{constants.Label: constants.Etcd}
)

func NewEtcdTask() workflow.Task {
	return workflow.Task{
		Name:        "Etcd",
		Run:         runEtcd,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-etcd",
				Run:  runDeployEtcd,
			},
			{
				Name: "check-etcd",
				Run:  runCheckEtcd,
			},
		},
	}
}

func runEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("etcd task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[etcd] Running etcd task", "virtual cluster", klog.KObj(data))
	return nil
}

func runDeployEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("deploy-etcd task invoked with an invalid data struct")
	}

	err := controlplane.EnsureVirtualClusterEtcd(data.RemoteClient(), data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install etcd component, err: %w", err)
	}

	klog.V(2).InfoS("[deploy-etcd] Successfully installed etcd component", "virtual cluster", klog.KObj(data))
	return nil
}

func runCheckEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("check-etcd task invoked with an invalid data struct")
	}

	checker := apiclient.NewVirtualClusterChecker(data.RemoteClient(), constants.ComponentBeReadyTimeout)

	if err := checker.WaitForSomePods(etcdLabels.String(), data.GetNamespace(), 1); err != nil {
		return fmt.Errorf("checking for virtual cluster etcd to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[check-etcd] the etcd pods is ready", "virtual cluster", klog.KObj(data))
	return nil
}
