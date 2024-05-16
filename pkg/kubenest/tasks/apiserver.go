package tasks

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewVirtualClusterApiserverTask() workflow.Task {
	return workflow.Task{
		Name:        "apiserver",
		Run:         runApiserver,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-apiserver",
				Run:  runVirtualClusterAPIServer,
			},
			{
				Name: "check-apiserver",
				Run:  runCheckVirtualClusterAPIServer,
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
		return errors.New("Virtual cluster apiserver task invoked with an invalid data struct")
	}

	err := controlplane.EnsureVirtualClusterAPIServer(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
		data.HostPort(),
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver component, err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterApiserver] Successfully installed virtual cluster apiserver component", "virtual cluster", klog.KObj(data))
	return nil
}

func runCheckVirtualClusterAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("check-VirtualClusterAPIServer task invoked with an invalid data struct")
	}

	checker := apiclient.NewVirtualClusterChecker(data.RemoteClient(), constants.ComponentBeReadyTimeout)

	err := checker.WaitForSomePods(virtualClusterApiserverLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("checking for virtual cluster apiserver to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[check-VirtualClusterAPIServer] the virtual cluster apiserver is ready", "virtual cluster", klog.KObj(data))
	return nil
}

func UninstallVirtualClusterApiserverTask() workflow.Task {
	return workflow.Task{
		Name:        "apiserver",
		Run:         runApiserver,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: constants.ApiServer,
				Run:  uninstallVirtualClusterAPIServer,
			},
		},
	}
}

func uninstallVirtualClusterAPIServer(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster apiserver task invoked with an invalid data struct")
	}

	err := controlplane.DeleteVirtualClusterAPIServer(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster apiserver component, err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterApiserver] Successfully uninstalled virtual cluster apiserver component", "virtual cluster", klog.KObj(data))
	return nil
}
