package task

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
)

func NewRemoveNodeFromVirtualTask() Task {
	return Task{
		Name: "remove node from virtual control-plane",
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			err := to.VirtualK8sClient.CoreV1().Nodes().Delete(ctx, to.NodeInfo.Name, metav1.DeleteOptions{})
			if err != nil {
				return nil, fmt.Errorf("remove node from cluster failed, node name:%s, erro: %s", to.NodeInfo.Name, err)
			}
			return nil, nil
		},
	}
}

func NewExecShellUnjoinCmdTask() Task {
	return Task{
		Name:  "exec shell unjoin cmd",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")

			resetCmd := &exector.CMDExector{
				Cmd: fmt.Sprintf("sh %s unjoin", env.GetExectorShellName()),
			}

			ret := exectHelper.DoExector(ctx.Done(), resetCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("reset node %s failed: %s", to.NodeInfo.Name, ret.String())
			}

			return nil, nil
		},
	}
}

func getJoinCmdStr(log string) (string, error) {
	strs := strings.Split(log, "kubeadm join")
	if len(strs) != 2 {
		return "", fmt.Errorf("get join cmd str failed")
	}
	return fmt.Sprintf("kubeadm join %s", strs[1]), nil
}

func NewJoinNodeToHostCmd() Task {
	return Task{
		Name: "join node to host",
		SubTasks: []Task{
			NewGetJoinNodeToHostCmdTask(),
			NewExecJoinNodeToHostCmdTask(),
		},
	}
}

func NewGetJoinNodeToHostCmdTask() Task {
	return Task{
		Name:  "remote get host node join cmd str",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			masterNodeIP := env.GetExectorHostMasterNodeIP()
			hostExectorHelper := exector.NewExectorHelper(masterNodeIP, "")
			joinCmdStrCmd := &exector.CMDExector{
				Cmd: "kubeadm token create --print-join-command",
			}
			ret := hostExectorHelper.DoExector(ctx.Done(), joinCmdStrCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("get host join cmd on node %s failed: %s", to.NodeInfo.Name, ret.String())
			}

			joinCmdStr, err := getJoinCmdStr(ret.LastLog)
			if err != nil {
				return nil, err
			}
			return joinCmdStr, nil
		},
	}
}

func NewExecJoinNodeToHostCmdTask() Task {
	return Task{
		Name:  "remote join node to host",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, args interface{}) (interface{}, error) {
			joinCmdStr, ok := args.(string)
			if !ok {
				return nil, fmt.Errorf("get join cmd str failed")
			}
			joinCmd := &exector.CMDExector{
				Cmd: joinCmdStr,
			}

			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")
			ret := exectHelper.DoExector(ctx.Done(), joinCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("exec join cmd on node %s failed: %s, join cmd: %s", to.NodeInfo.Name, ret.String(), joinCmdStr)
			}
			return nil, nil
		},
	}
}
