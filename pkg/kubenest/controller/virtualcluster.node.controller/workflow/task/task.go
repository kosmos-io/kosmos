package task

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/kubenest/constants"
	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

type TaskOpt struct {
	NodeInfo       v1alpha1.GlobalNode
	VirtualCluster v1alpha1.VirtualCluster
	KubeDNSAddress string

	HostClient       client.Client
	HostK8sClient    kubernetes.Interface
	VirtualK8sClient kubernetes.Interface
}

type Task struct {
	Name        string
	Run         func(context.Context, TaskOpt, interface{}) (interface{}, error)
	Retry       bool
	SubTasks    []Task
	ErrorIgnore bool
}

func NewCheckEnvTask() Task {
	return Task{
		Name:  "remote environment check",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")
			// check
			checkCmd := &exector.CMDExector{
				Cmd: fmt.Sprintf("sh %s check", env.GetExectorShellName()),
			}
			ret := exectHelper.DoExector(ctx.Done(), checkCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("check node %s failed: %s", to.NodeInfo.Name, ret.String())
			}
			return nil, nil
		},
	}
}

func NewKubeadmResetTask() Task {
	return Task{
		Name:  "remote kubeadm reset",
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

func NewDrainHostNodeTask() Task {
	return Task{
		Name:  "drain host node",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			targetNode, err := to.HostK8sClient.CoreV1().Nodes().Get(ctx, to.NodeInfo.Name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, fmt.Errorf("get node %s failed: %s", to.NodeInfo.Name, err)
			}

			if err := util.DrainNode(ctx, targetNode.Name, to.HostK8sClient, targetNode, env.GetDrainWaitSeconds()); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
}

func NewDrainVirtualNodeTask() Task {
	return Task{
		Name:  "drain virtual-control-plane node",
		Retry: true,
		// ErrorIgnore: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			targetNode, err := to.VirtualK8sClient.CoreV1().Nodes().Get(ctx, to.NodeInfo.Name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, fmt.Errorf("get node %s failed: %s", to.NodeInfo.Name, err)
			}

			if err := util.DrainNode(ctx, targetNode.Name, to.HostK8sClient, targetNode, env.GetDrainWaitSeconds()); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
}

func NewCleanHostClusterNodeTask() Task {
	return Task{
		Name:  "clean host cluster node",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			targetNode := &v1.Node{}
			if err := to.HostClient.Get(ctx, types.NamespacedName{
				Name: to.NodeInfo.Name,
			}, targetNode); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, fmt.Errorf("get target node %s failed: %s", to.NodeInfo.Name, err)
			}

			if err := to.HostClient.Delete(ctx, targetNode); err != nil {
				return nil, err
			}

			return nil, nil
		},
	}
}

func NewReomteUploadCATask() Task {
	return Task{
		Name:  "remote upload ca.crt",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")

			nn := types.NamespacedName{
				Namespace: to.VirtualCluster.Namespace,
				Name:      fmt.Sprintf("%s-cert", to.VirtualCluster.Name),
			}
			targetCert := &v1.Secret{}
			if err := to.HostClient.Get(ctx, nn, targetCert); err != nil {
				return nil, fmt.Errorf("get target cert %s failed: %s", nn, err)
			}

			cacrt := targetCert.Data["ca.crt"]
			scpCrtCmd := &exector.SCPExector{
				DstFilePath: env.GetExectorTmpPath(),
				DstFileName: "ca.crt",
				SrcByte:     cacrt,
			}
			ret := exectHelper.DoExector(ctx.Done(), scpCrtCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("scp ca.crt to node %s failed: %s", to.NodeInfo.Name, ret.String())
			}
			return nil, nil
		},
	}
}

func NewRemoteUpdateKubeletConfTask() Task {
	return Task{
		Name:  "remote upload kubelet.conf",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")

			kubeconfig, err := base64.StdEncoding.DecodeString(to.VirtualCluster.Spec.Kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("decode target kubeconfig %s failed: %s", to.VirtualCluster.Name, err)
			}

			scpKCCmd := &exector.SCPExector{
				DstFilePath: env.GetExectorTmpPath(),
				DstFileName: "kubelet.conf",
				SrcByte:     kubeconfig,
			}
			ret := exectHelper.DoExector(ctx.Done(), scpKCCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("scp kubeconfig to node %s failed: %s", to.NodeInfo.Name, ret.String())
			}
			return nil, nil
		},
	}
}

func NewRemoteUpdateConfigYamlTask() Task {
	return Task{
		Name:  "remote upload config.yaml",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")

			scpKubeletConfigCmd := &exector.SCPExector{
				DstFilePath: env.GetExectorTmpPath(),
				DstFileName: "config.yaml",
				SrcFile:     env.GetExectorWorkerDir() + "config.yaml", // from configmap volumn
			}

			ret := exectHelper.DoExector(ctx.Done(), scpKubeletConfigCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("scp kubelet config to node %s failed: %s", to.NodeInfo.Name, ret.String())
			}
			return nil, nil
		},
	}
}

func NewRemoteNodeJoinTask() Task {
	return Task{
		Name:  "remote join node to virtual control plane",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")

			joinCmd := &exector.CMDExector{
				Cmd: fmt.Sprintf("sh %s join %s", env.GetExectorShellName(), to.KubeDNSAddress),
			}
			ret := exectHelper.DoExector(ctx.Done(), joinCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("join node %s failed: %s", to.NodeInfo.Name, ret.String())
			}
			return nil, nil
		},
	}
}

func NewWaitNodeReadyTask() Task {
	return Task{
		Name: "wait new node ready",
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second) // total waiting time
			defer cancel()

			isReady := false

			wait.UntilWithContext(waitCtx, func(ctx context.Context) {
				node, err := to.VirtualK8sClient.CoreV1().Nodes().Get(waitCtx, to.NodeInfo.Name, metav1.GetOptions{})
				if err == nil {
					if util.IsNodeReady(node.Status.Conditions) {
						klog.V(4).Infof("node %s is ready", to.NodeInfo.Name)
						isReady = true
						cancel()
					} else {
						klog.V(4).Infof("node %s is not ready, status: %s", to.NodeInfo.Name, node.Status.Phase)
					}
				} else {
					klog.V(4).Infof("get node %s failed: %s", to.NodeInfo.Name, err)
				}
			}, 10*time.Second) // Interval time

			if isReady {
				return nil, nil
			}

			return nil, fmt.Errorf("node %s is not ready", to.NodeInfo.Name)
		},
	}
}

// nolint:dupl
func NewUpdateVirtualNodeLabelsTask() Task {
	return Task{
		Name:  "update new-node labels",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				node, err := to.VirtualK8sClient.CoreV1().Nodes().Get(ctx, to.NodeInfo.Name, metav1.GetOptions{})
				if err != nil {
					klog.V(4).Infof("get node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}

				updateNode := node.DeepCopy()
				for k, v := range to.NodeInfo.Labels {
					node.Labels[k] = v
				}

				// add free label
				node.Labels[constants.StateLabelKey] = string(v1alpha1.NodeInUse)

				if _, err := to.VirtualK8sClient.CoreV1().Nodes().Update(ctx, updateNode, metav1.UpdateOptions{}); err != nil {
					klog.V(4).Infof("add label to node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}
				return nil
			})

			return nil, err
		},
	}
}

// nolint:dupl
func NewUpdateHostNodeLabelsTask() Task {
	return Task{
		Name:  "update host-node labels",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				node, err := to.HostK8sClient.CoreV1().Nodes().Get(ctx, to.NodeInfo.Name, metav1.GetOptions{})
				if err != nil {
					klog.V(4).Infof("get node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}

				updateNode := node.DeepCopy()
				for k, v := range to.NodeInfo.Labels {
					node.Labels[k] = v
				}

				// add free label
				node.Labels[constants.StateLabelKey] = string(v1alpha1.NodeFreeState)

				if _, err := to.HostK8sClient.CoreV1().Nodes().Update(ctx, updateNode, metav1.UpdateOptions{}); err != nil {
					klog.V(4).Infof("add label to node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}
				return nil
			})

			return nil, err
		},
	}
}

func NewUpdateNodePoolItemStatusTask(nodeState v1alpha1.NodeState, isClean bool) Task {
	return Task{
		Name: "Update node status in NodePool ",
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				targetGlobalNode := v1alpha1.GlobalNode{}

				if err := to.HostClient.Get(ctx, types.NamespacedName{Name: to.NodeInfo.Name}, &targetGlobalNode); err != nil {
					klog.Errorf("get global node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}

				updateGlobalNode := targetGlobalNode.DeepCopy()

				updateGlobalNode.Spec.State = nodeState
				if err := to.HostClient.Update(ctx, updateGlobalNode); err != nil {
					klog.Errorf("update global node %s spec.state failed: %s", updateGlobalNode.Name, err)
					return err
				}
				if isClean {
					updateGlobalNode.Status.VirtualCluster = ""
					if err := to.HostClient.Status().Update(ctx, updateGlobalNode); err != nil {
						klog.Errorf("update global node %s status failed: %s", updateGlobalNode.Name, err)
						return err
					}
				}
				return nil
			})

			return nil, err
		},
	}
}

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
