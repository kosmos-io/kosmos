package tasks

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controlplane"
	"github.com/kosmos.io/kosmos/pkg/kubenest/workflow"
)

func NewEndPointTask() workflow.Task {
	return workflow.Task{
		Name:        "endpoint",
		Run:         runEndpoint,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "deploy-endpoint-in-virtual-cluster",
				Run:  runEndPointInVirtualClusterTask,
			},
		},
	}
}

func runEndpoint(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("endPoint task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[endPoint] Running endPoint task", "virtual cluster", klog.KObj(data))
	return nil
}

func runEndPointInVirtualClusterTask(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("Virtual cluster endpoint task invoked with an invalid data struct")
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

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	err = controlplane.EnsureApiServerExternalEndPoint(kubeClient)
	if err != nil {
		return err
	}
	return nil
}
