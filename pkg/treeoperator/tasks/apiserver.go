package tasks

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/treeoperator/controlplane/apiserver"
	apiclient "github.com/kosmos.io/kosmos/pkg/treeoperator/util/apiclient"
	"github.com/kosmos.io/kosmos/pkg/treeoperator/workflow"
)

const (
	VirtualClusterAPIserverComponent = "VirtualClusterAPIServer"
	APIServer                        = "apiserver"
)

var (
	virtualClusterApiserverLabels = labels.Set{"virtualCluster-app": APIServer}
)

func NewVirtualClusterApiserverTask() workflow.Task {
	return workflow.Task{
		Name:        "apiserver",
		Run:         runApiserver,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: VirtualClusterAPIserverComponent,
				Run:  runVirtualClusterAPIServer,
			},
			{
				Name: fmt.Sprintf("%s-%s", "wait", VirtualClusterAPIserverComponent),
				Run:  runWaitVirtualClusterAPIServer,
			},
		},
	}
}

func runApiserver(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("apiserver task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[apiserver] Running apiserver task", "virtual cluster", klog.KObj(data))
	return nil
}

func runVirtualClusterAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("VirtualClusterApiserver task invoked with an invalid data struct")
	}

	err := apiserver.EnsureVirtualClusterAPIServer(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver component, err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterApiserver] Successfully installed virtualCluster-apiserver component", "virtual cluster", klog.KObj(data))
	return nil
}

func runWaitVirtualClusterAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("wait-VirtualClusterAPIServer task invoked with an invalid data struct")
	}

	waiter := apiclient.NewVirtualClusterWaiter(data.ControlplaneConfig(), data.RemoteClient(), componentBeReadyTimeout)

	err := waiter.WaitForSomePods(virtualClusterApiserverLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("waiting for virtualCluster-apiserver to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[wait-VirtualClusterAPIServer] the virtualCluster-apiserver is ready", "virtual cluster", klog.KObj(data))
	return nil
}
