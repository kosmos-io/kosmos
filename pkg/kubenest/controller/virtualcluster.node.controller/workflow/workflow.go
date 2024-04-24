package workflow

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
)

func NewJoinWorkerFlow() WorkflowData {
	joinTasks := []Task{
		NewCheckEnvTask(),
		NewKubeadmResetTask(),
		NewReomteUploadCATask(),
		NewRemoteUpdateKubeletConfTask(),
		NewRemoteUpdateConfigYamlTask(),
		NewRemoteNodeJoinTask(),
		NewWaitNodeReadyTask(),
		NewUpdateNodeLabelsTask(),
		NewUpdateNodePoolItemStatusTask(v1alpha1.NodeInUse, false),
	}

	return WorkflowData{
		Tasks: joinTasks,
	}
}

func NewUnjoinworkerFlow() WorkflowData {
	unjoinTasks := []Task{
		NewCheckEnvTask(),
		NewRemoveNodeFromVirtualTask(),
		NewExecShellUnjoinCmdTask(),
		NewJoinNodeToHostCmd(),
		NewUpdateNodePoolItemStatusTask(v1alpha1.NodeFreeState, true),
	}
	return WorkflowData{
		Tasks: unjoinTasks,
	}
}

const (
	retryCount = 0
	maxRetries = 3
)

type WorkflowData struct {
	Tasks []Task
}

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

func RunWithRetry(ctx context.Context, task Task, opt TaskOpt, preArgs interface{}) (interface{}, error) {
	i := retryCount
	var err error
	var args interface{}
	for ; i < maxRetries; i++ {
		if args, err = task.Run(ctx, opt, preArgs); err != nil {
			if !task.Retry {
				break
			}
			klog.V(4).Infof("work flow retry %d, task name: %s, err: %s", i, task.Name, err)
		} else {
			break
		}
	}
	if err != nil {
		klog.V(4).Infof("work flow interrupt, task name: %s, err: %s", task.Name, err)
		return nil, err
	}
	return args, nil
}

func (w WorkflowData) RunTask(ctx context.Context, opt TaskOpt) error {
	var args interface{}
	for i, task := range w.Tasks {
		klog.V(4).Infof("HHHHHHHHHHHH (%d/%d) work flow run task %s  HHHHHHHHHHHH", i, len(w.Tasks), task.Name)
		if len(task.SubTasks) > 0 {
			for j, subTask := range task.SubTasks {
				klog.V(4).Infof("HHHHHHHHHHHH (%d/%d) work flow run sub task %s HHHHHHHHHHHH", j, len(task.SubTasks), subTask.Name)
				if nextArgs, err := RunWithRetry(ctx, subTask, opt, args); err != nil {
					return err
				} else {
					args = nextArgs
				}
			}
		} else {
			if nextArgs, err := RunWithRetry(ctx, task, opt, args); err != nil {
				return err
			} else {
				args = nextArgs
			}
		}
	}
	return nil
}
