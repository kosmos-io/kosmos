package tasks

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/controlplane/etcd"
	apiclient "github.com/kosmos.io/kosmos/pkg/treeoperator/util/apiclient"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

var (
	componentBeReadyTimeout = 120 * time.Second
	etcdLabels              = labels.Set{"virtualCluster-app": "etcd"}
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
				Name: "wait-etcd",
				Run:  runWaitEtcd,
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

	err := etcd.EnsureKarmadaEtcd(data.RemoteClient(), data.GetName(), data.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to install etcd component, err: %w", err)
	}

	klog.V(2).InfoS("[deploy-etcd] Successfully installed etcd component", "virtual cluster", klog.KObj(data))
	return nil
}

func runWaitEtcd(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-etcd task invoked with an invalid data struct")
	}

	waiter := apiclient.NewVirtualClusterWaiter(data.ControlplaneConfig(), data.RemoteClient(), componentBeReadyTimeout)

	if err := waiter.WaitForSomePods(etcdLabels.String(), data.GetNamespace(), 1); err != nil {
		return fmt.Errorf("waiting for virtualCluster-etcd to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[wait-etcd] the etcd pods is ready", "virtual cluster", klog.KObj(data))
	return nil
}
