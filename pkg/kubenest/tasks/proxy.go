package tasks

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane"
	apiclient "github.com/kosmos.io/kosmos/pkg/kubenest/util/api-client"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

var (
	virtualClusterProxyLabels = labels.Set{constants.Label: constants.Proxy}
)

func NewVirtualClusterProxyTask() workflow.Task {
	return workflow.Task{
		Name:        "proxy",
		Run:         runProxy,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-proxy",
				Run:  runVirtualClusterProxy,
			},
			{
				Name: "check-proxy",
				Run:  runCheckVirtualClusterProxy,
			},
		},
	}
}

func runProxy(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("proxy task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[proxy] Running proxy task", "virtual cluster", klog.KObj(data))
	return nil
}

func runVirtualClusterProxy(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster proxy task invoked with an invalid data struct")
	}

	kubeNestOpt := data.KubeNestOpt()

	// Get the kubeconfig of virtual cluster and put it into the cm of kube-proxy
	secret, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(),
		fmt.Sprintf("%s-%s", data.GetName(), constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Get virtualcluster kubeconfig secret error")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[constants.KubeConfig])
	if err != nil {
		return err
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	var virtualClient clientset.Interface = client

	kubeconfigString := string(secret.Data[constants.KubeConfig])

	err = controlplane.EnsureVirtualClusterProxy(
		virtualClient,
		kubeconfigString,
		kubeNestOpt.ClusterCIDR,
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster proxy component, err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterProxy] Successfully installed virtual cluster proxy component", "virtual cluster", klog.KObj(data))
	return nil
}

func runCheckVirtualClusterProxy(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("check-VirtualClusterProxy task invoked with an invalid data struct")
	}

	checker := apiclient.NewVirtualClusterChecker(data.RemoteClient(), constants.ComponentBeReadyTimeout)

	err := checker.WaitForSomePods(virtualClusterProxyLabels.String(), data.GetNamespace(), 1)
	if err != nil {
		return fmt.Errorf("checking for virtual cluster proxy to ready timeout, err: %w", err)
	}

	klog.V(2).InfoS("[check-VirtualClusterProxy] the virtual cluster proxy is ready", "virtual cluster", klog.KObj(data))
	return nil
}

func UninstallVirtualClusterProxyTask() workflow.Task {
	return workflow.Task{
		Name:        "proxy",
		Run:         runProxy,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: constants.ApiServer,
				Run:  uninstallVirtualClusterProxy,
			},
		},
	}
}

func uninstallVirtualClusterProxy(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster proxy task invoked with an invalid data struct")
	}

	secret, err := data.RemoteClient().CoreV1().Secrets(data.GetNamespace()).Get(context.TODO(),
		fmt.Sprintf("%s-%s", data.GetName(), constants.AdminConfig), metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Get virtualcluster kubeconfig secret error")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[constants.KubeConfig])
	if err != nil {
		return err
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	var virtualClient clientset.Interface = client

	err = controlplane.DeleteVirtualClusterProxy(
		virtualClient,
	)
	if err != nil {
		return fmt.Errorf("failed to install virtual cluster proxy component, err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterProxy] Successfully uninstalled virtual cluster proxy component", "virtual cluster", klog.KObj(data))
	return nil
}
