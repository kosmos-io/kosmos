package task

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

type TaskOpt struct {
	NodeInfo       v1alpha1.GlobalNode
	VirtualCluster v1alpha1.VirtualCluster
	KubeDNSAddress string

	HostK8sClient    client.Client
	VirtualK8sClient kubernetes.Interface
}

type Task struct {
	Name     string
	Run      func(context.Context, TaskOpt, interface{}) (interface{}, error)
	Retry    bool
	SubTasks []Task
}
