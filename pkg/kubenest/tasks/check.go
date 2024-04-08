package tasks

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

var (
	kubeControllerManagerLabels = labels.Set{"virtualCluster-app": constants.KubeControllerManager}
	virtualClusterManagerLabels = labels.Set{"virtualCluster-app": constants.VirtualClusterScheduler}
)

func NewCheckApiserverHealthTask() workflow.Task {
	return workflow.Task{
		Name: "check-apiserver-health",
		Run:  runCheckApiserver,
	}
}

func NewCheckControlPlaneTask() workflow.Task {
	return workflow.Task{
		Name:        "check-controlPlane-health",
		Run:         runCheckControlPlane,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			newCheckControlPlaneSubTask("KubeControllerManager", kubeControllerManagerLabels),
			newCheckControlPlaneSubTask("VirtualClusterScheduler", virtualClusterManagerLabels),
		},
	}
}

func newCheckControlPlaneSubTask(component string, ls labels.Set) workflow.Task {
	return workflow.Task{
		Name: component,
		Run:  runCheckControlPlaneSubTask(component, ls),
	}
}

func runCheckApiserver(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return fmt.Errorf("check-apiserver-health task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[check-apiserver-health] Running task", "virtual cluster", klog.KObj(data))

	checker := apiclient.NewVirtualClusterChecker(data.ControlplaneConfig(), data.RemoteClient(), constants.ComponentBeReadyTimeout)

	if err := apiclient.TryRunCommand(checker.WaitForAPI, 3); err != nil {
		return fmt.Errorf("the virtual cluster apiserver is unhealthy, err: %w", err)
	}
	klog.V(2).InfoS("[check-apiserver-health] the etcd and virtualCluster-apiserver is healthy", "virtual cluster", klog.KObj(data))
	return nil
}

func runCheckControlPlane(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("check-controlPlane task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[check-controlPlane] Running wait-controlPlane task", "virtual cluster", klog.KObj(data))
	return nil
}

func runCheckControlPlaneSubTask(component string, ls labels.Set) func(r workflow.RunData) error {
	return func(r workflow.RunData) error {
		data, ok := r.(InitData)
		if !ok {
			return errors.New("check-controlPlane task invoked with an invalid data struct")
		}

		checker := apiclient.NewVirtualClusterChecker(nil, data.RemoteClient(), constants.ComponentBeReadyTimeout)
		if err := checker.WaitForSomePods(ls.String(), data.GetNamespace(), 1); err != nil {
			return fmt.Errorf("checking for %s to ready timeout, err: %w", component, err)
		}

		klog.V(2).InfoS("[check-ControlPlane] component status is ready", "component", component, "virtual cluster", klog.KObj(data))
		return nil
	}
}
