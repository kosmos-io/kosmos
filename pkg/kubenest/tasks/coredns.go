package tasks

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane/coredns"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewCoreDNSTask() workflow.Task {
	return workflow.Task{
		Name:        "coreDns",
		Run:         runCoreDns,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-core-dns-in-host-cluster",
				Run:  runCoreDnsHostTask,
			},
			{
				Name: "check-core-dns",
				Run:  runCheckCoreDnsTask,
			},
			{
				Name: "deploy-core-dns-service-in-virtual-cluster",
				Run:  runCoreDnsVirtualTask,
			},
		},
	}
}

func runCoreDns(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("coreDns task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[coreDns] Running coreDns task", "virtual cluster", klog.KObj(data))
	return nil
}

func UninstallCoreDNSTask() workflow.Task {
	return workflow.Task{
		Name:        "coredns",
		Run:         runCoreDns,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "remove-core-dns-in-host-cluster",
				Run:  uninstallCorednsHostTask,
			},
		},
	}
}

// in host
func runCoreDnsHostTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}

	err := coredns.EnsureHostCoreDns(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
	)

	if err != nil {
		return fmt.Errorf("failed to install core-dns in host, err: %w", err)
	}

	klog.V(2).InfoS("[VirtualClusterCoreDns] Successfully installed core-dns in host", "core-dns", klog.KObj(data))
	return nil
}

func uninstallCorednsHostTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}

	err := coredns.DeleteCoreDnsDeployment(
		data.RemoteClient(),
		data.GetName(),
		data.GetNamespace(),
	)

	if err != nil {
		return err
	}
	return nil
}

// in host
func runCheckCoreDnsTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster manifests-components task invoked with an invalid data struct")
	}
	ctx := context.TODO()

	waitCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	isReady := false

	wait.UntilWithContext(waitCtx, func(ctx context.Context) {
		_, err := data.RemoteClient().CoreV1().Services(data.GetNamespace()).Get(context.TODO(), constants.KubeDNSSVCName, metav1.GetOptions{})
		if err == nil {
			// TODO: check endpoints
			isReady = true
			cancel()
		}
	}, 10*time.Second) // Interval time

	if isReady {
		return nil
	}

	return fmt.Errorf("kube-dns is not ready")
}

func runCoreDnsVirtualTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster coreDns task invoked with an invalid data struct")
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
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	kubesvc, err := data.RemoteClient().CoreV1().Services(data.GetNamespace()).Get(context.TODO(), constants.KubeDNSSVCName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	DNSPort := int32(0)
	DNSTCPPort := int32(0)
	MetricsPort := int32(0)

	for _, port := range kubesvc.Spec.Ports {
		if port.Name == "dns" {
			DNSPort = port.NodePort
		}
		if port.Name == "dns-tcp" {
			DNSTCPPort = port.NodePort
		}
		if port.Name == "metrics" {
			MetricsPort = port.NodePort
		}
	}
	HostNodeAddress := os.Getenv("EXECTOR_HOST_MASTER_NODE_IP")
	if len(HostNodeAddress) == 0 {
		return fmt.Errorf("get master node ip from env failed")
	}

	templatedMapping := map[string]interface{}{
		"Namespace":       data.GetNamespace(),
		"Name":            data.GetName(),
		"DNSPort":         DNSPort,
		"DNSTCPPort":      DNSTCPPort,
		"MetricsPort":     MetricsPort,
		"HostNodeAddress": HostNodeAddress,
	}
	for k, v := range data.PluginOptions() {
		templatedMapping[k] = v
	}

	err = coredns.EnsureVirtualClusterCoreDns(dynamicClient, templatedMapping)
	if err != nil {
		return err
	}

	return nil
}
