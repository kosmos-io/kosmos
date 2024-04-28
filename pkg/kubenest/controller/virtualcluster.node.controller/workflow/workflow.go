package workflow

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/workflow/task"
)

const (
	retryCount = 0
	maxRetries = 3
)

type WorkflowData struct {
	Tasks []task.Task
}

func RunWithRetry(ctx context.Context, task task.Task, opt task.TaskOpt, preArgs interface{}) (interface{}, error) {
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

func (w WorkflowData) RunTask(ctx context.Context, opt task.TaskOpt) error {
	var args interface{}
	for i, task := range w.Tasks {
		klog.V(4).Infof("HHHHHHHHHHHH (%d/%d) work flow run task %s  HHHHHHHHHHHH", i+1, len(w.Tasks), task.Name)
		if len(task.SubTasks) > 0 {
			for j, subTask := range task.SubTasks {
				klog.V(4).Infof("HHHHHHHHHHHH (%d/%d) work flow run sub task %s HHHHHHHHHHHH", j+1, len(task.SubTasks), subTask.Name)
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

func NewJoinWorkFlow() WorkflowData {
	joinTasks := []task.Task{
		task.NewCheckEnvTask(),
		task.NewKubeadmResetTask(),
		task.NewCleanHostClusterNodeTask(),
		task.NewReomteUploadCATask(),
		task.NewRemoteUpdateKubeletConfTask(),
		task.NewRemoteUpdateConfigYamlTask(),
		task.NewRemoteNodeJoinTask(),
		task.NewWaitNodeReadyTask(),
		task.NewUpdateNodeLabelsTask(),
		task.NewUpdateNodePoolItemStatusTask(v1alpha1.NodeInUse, false),
	}

	return WorkflowData{
		Tasks: joinTasks,
	}
}

func NewUnjoinWorkFlow() WorkflowData {
	unjoinTasks := []task.Task{
		task.NewCheckEnvTask(),
		task.NewRemoveNodeFromVirtualTask(),
		task.NewExecShellUnjoinCmdTask(),
		task.NewJoinNodeToHostCmd(),
		task.NewUpdateNodePoolItemStatusTask(v1alpha1.NodeFreeState, true),
	}
	return WorkflowData{
		Tasks: unjoinTasks,
	}
}

func NewCleanNodeWorkFlow() WorkflowData {
	cleanNodeTasks := []task.Task{
		task.NewUpdateNodePoolItemStatusTask(v1alpha1.NodeFreeState, true),
	}
	return WorkflowData{
		Tasks: cleanNodeTasks,
	}
}
