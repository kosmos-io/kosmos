package tasks

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	apiclient "github.com/kosmos.io/kosmos/pkg/treeoperator/util/apiclient"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

const (
	KubeControllerManager   = "kube-controller-manager"
	VirtualClusterScheduler = "virtualCluster-scheduler"
)

var (
	kubeControllerManagerLabels = labels.Set{"virtualCluster-app": KubeControllerManager}
	virtualClusterManagerLabels = labels.Set{"virtualCluster-app": VirtualClusterScheduler}
)

func NewCheckApiserverHealthTask() workflow.Task {
	return workflow.Task{
		Name: "check-apiserver-health",
		Run:  runWaitApiserver,
	}
}

func NewCheckControlPlaneTask() workflow.Task {
	return workflow.Task{
		Name:        "wait-controlPlane",
		Run:         runWaitControlPlane,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newWaitControlPlaneSubTask("KubeControllerManager", kubeControllerManagerLabels),
			newWaitControlPlaneSubTask("VirtualClusterScheduler", virtualClusterManagerLabels),
		},
	}
}

func runWaitControlPlane(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-controlPlane task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[wait-controlPlane] Running wait-controlPlane task", "virtual cluster", klog.KObj(data))
	return nil
}

func runWaitApiserver(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return fmt.Errorf("check-apiserver-health task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[check-apiserver-health] Running task", "virtual cluster", klog.KObj(data))

	waiter := apiclient.NewVirtualClusterWaiter(data.ControlplaneConfig(), data.RemoteClient(), componentBeReadyTimeout)

	if err := apiclient.TryRunCommand(waiter.WaitForAPI, 3); err != nil {
		return fmt.Errorf("the virtual cluster apiserver is unhealthy, err: %w", err)
	}
	klog.V(2).InfoS("[check-apiserver-health] the etcd and virtualCluster-apiserver is healthy", "virtual cluster", klog.KObj(data))
	return nil
}

func newWaitControlPlaneSubTask(component string, ls labels.Set) workflow.Task {
	return workflow.Task{
		Name: component,
		Run:  runWaitControlPlaneSubTask(component, ls),
	}
}

func runWaitControlPlaneSubTask(component string, ls labels.Set) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("wait-controlPlane task invoked with an invalid data struct")
		}

		waiter := apiclient.NewVirtualClusterWaiter(nil, data.RemoteClient(), componentBeReadyTimeout)
		if err := waiter.WaitForSomePods(ls.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("waiting for %s to ready timeout, err: %w", component, err)
		}

		klog.V(2).InfoS("[wait-ControlPlane] component status is ready", "component", component, "virtual cluster", klog.KObj(data))
		return nil
	}
}
