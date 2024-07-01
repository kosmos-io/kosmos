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

	Opt    *v1alpha1.KubeNestConfiguration
	logger *PrefixedLogger
}

func (to *TaskOpt) Loger() *PrefixedLogger {
	if to.logger == nil {
		to.logger = NewPrefixedLogger(klog.V(4), fmt.Sprintf("[%s] ", to.NodeInfo.Name))
	}
	return to.logger
}

type Task struct {
	Name        string
	Run         func(context.Context, TaskOpt, interface{}) (interface{}, error)
	Retry       bool
	SubTasks    []Task
	Skip        func(context.Context, TaskOpt) bool
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
				Cmd: fmt.Sprintf("bash %s check", env.GetExectorShellName()),
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
				Cmd: fmt.Sprintf("bash %s unjoin", env.GetExectorShellName()),
			}

			ret := exectHelper.DoExector(ctx.Done(), resetCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("reset node %s failed: %s", to.NodeInfo.Name, ret.String())
			}
			return nil, nil
		},
	}
}

// nolint:dupl
func NewDrainHostNodeTask() Task {
	return Task{
		Name:  "drain host node",
		Retry: true,
		Skip: func(ctx context.Context, opt TaskOpt) bool {
			if opt.Opt != nil {
				return opt.Opt.KubeInKubeConfig.ForceDestroy
			}
			return false
		},
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

// nolint:dupl
func NewDrainVirtualNodeTask() Task {
	return Task{
		Name:  "drain virtual-control-plane node",
		Retry: true,
		// ErrorIgnore: true,
		Skip: func(ctx context.Context, opt TaskOpt) bool {
			if opt.Opt != nil {
				return opt.Opt.KubeInKubeConfig.ForceDestroy
			}
			return false
		},
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
				DstFileName: env.GetKubeletKubeConfigName(),
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
				DstFileName: env.GetKubeletConfigName(),
				SrcFile:     env.GetExectorWorkerDir() + env.GetKubeletConfigName(),
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
				Cmd: fmt.Sprintf("bash %s join %s", env.GetExectorShellName(), to.KubeDNSAddress),
			}
			to.Loger().Infof("join node %s with cmd: %s", to.NodeInfo.Name, joinCmd.Cmd)
			ret := exectHelper.DoExector(ctx.Done(), joinCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("join node %s failed: %s", to.NodeInfo.Name, ret.String())
			}
			return nil, nil
		},
	}
}

func NewWaitNodeReadyTask(isHost bool) Task {
	return Task{
		Name: "wait new node ready",
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			isReady := false

			waitFunc := func(timeout time.Duration) {
				waitCtx, cancel := context.WithTimeout(ctx, timeout) // total waiting time
				defer cancel()
				wait.UntilWithContext(waitCtx, func(ctx context.Context) {
					client := to.VirtualK8sClient
					if isHost {
						client = to.HostK8sClient
					}

					node, err := client.CoreV1().Nodes().Get(waitCtx, to.NodeInfo.Name, metav1.GetOptions{})
					if err == nil {
						if util.IsNodeReady(node.Status.Conditions) {
							to.Loger().Infof("node %s is ready", to.NodeInfo.Name)
							isReady = true
							cancel()
						} else {
							to.Loger().Infof("node %s is not ready, status: %s", to.NodeInfo.Name, node.Status.Phase)
						}
					} else {
						to.Loger().Infof("get node %s failed: %s", to.NodeInfo.Name, err)
					}
				}, 10*time.Second) // Interval time
			}

			waitFunc(time.Duration(env.GetWaitNodeReadTime()) * time.Second)

			if isReady {
				return nil, nil
			}

			// try to restart containerd and kubelet
			to.Loger().Infof("try to restart containerd and kubelet on node: %s", to.NodeInfo.Name)
			exectHelper := exector.NewExectorHelper(to.NodeInfo.Spec.NodeIP, "")

			restartContainerdCmd := &exector.CMDExector{
				Cmd: "systemctl restart containerd",
			}
			ret := exectHelper.DoExector(ctx.Done(), restartContainerdCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("cannot restart containerd: %s", ret.String())
			}

			restartKubeletCmd := &exector.CMDExector{
				Cmd: "systemctl restart kubelet",
			}
			ret = exectHelper.DoExector(ctx.Done(), restartKubeletCmd)
			if ret.Status != exector.SUCCESS {
				return nil, fmt.Errorf("cannot restart kubelet: %s", ret.String())
			}

			to.Loger().Infof("wait for the node to be ready again. %s", to.NodeInfo.Name)
			waitFunc(time.Duration(env.GetWaitNodeReadTime()*2) * time.Second)

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
					to.Loger().Infof("get node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}

				updateNode := node.DeepCopy()
				for k, v := range to.NodeInfo.Spec.Labels {
					updateNode.Labels[k] = v
				}

				// add free label
				updateNode.Labels[constants.StateLabelKey] = string(v1alpha1.NodeInUse)

				if _, err := to.VirtualK8sClient.CoreV1().Nodes().Update(ctx, updateNode, metav1.UpdateOptions{}); err != nil {
					to.Loger().Infof("add label to node %s failed: %s", to.NodeInfo.Name, err)
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
					to.Loger().Infof("get node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}

				updateNode := node.DeepCopy()
				for k, v := range to.NodeInfo.Spec.Labels {
					updateNode.Labels[k] = v
				}

				// add free label
				updateNode.Labels[constants.StateLabelKey] = string(v1alpha1.NodeFreeState)

				if _, err := to.HostK8sClient.CoreV1().Nodes().Update(ctx, updateNode, metav1.UpdateOptions{}); err != nil {
					to.Loger().Infof("add label to node %s failed: %s", to.NodeInfo.Name, err)
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
					to.Loger().Errorf("get global node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}

				updateGlobalNode := targetGlobalNode.DeepCopy()

				updateGlobalNode.Spec.State = nodeState
				if err := to.HostClient.Update(ctx, updateGlobalNode); err != nil {
					to.Loger().Errorf("update global node %s spec.state failed: %s", updateGlobalNode.Name, err)
					return err
				}
				if isClean {
					updateGlobalNode.Status.VirtualCluster = ""
					if err := to.HostClient.Status().Update(ctx, updateGlobalNode); err != nil {
						to.Loger().Errorf("update global node %s status failed: %s", updateGlobalNode.Name, err)
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
				Cmd: fmt.Sprintf("bash %s unjoin", env.GetExectorShellName()),
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
	return fmt.Sprintf("kubeadm join %s", strings.TrimSpace(strs[1])), nil
}

func getJoinCmdArgs(joinCmdStr string) (string, string, string, error) {
	strs := strings.Split(joinCmdStr, " ")
	if len(strs) != 7 {
		return "", "", "", fmt.Errorf("invalid join cmd str: %s", joinCmdStr)
	}
	return strings.TrimSpace(strs[2]), strings.TrimSpace(strs[4]), strings.TrimSpace(strs[6]), nil
}

func NewJoinNodeToHostCmd() Task {
	return Task{
		Name: "join node to host",
		SubTasks: []Task{
			NewGetJoinNodeToHostCmdTask(),
			NewExecJoinNodeToHostCmdTask(),
			NewWaitNodeReadyTask(true),
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
			// check
			_, err := to.HostK8sClient.CoreV1().Nodes().Get(ctx, to.NodeInfo.Name, metav1.GetOptions{})
			if err == nil {
				to.Loger().Info("node already joined, skip task")
				return nil, nil
			}
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("query node %s failed, the error is %s", to.NodeInfo.Name, err.Error())
			}

			joinCmdStr, ok := args.(string)
			if !ok {
				return nil, fmt.Errorf("get join cmd str failed")
			}
			host, token, certHash, err := getJoinCmdArgs(joinCmdStr)
			if err != nil {
				return nil, err
			}
			joinCmd := &exector.CMDExector{
				Cmd: fmt.Sprintf("bash %s revert %s %s %s", env.GetExectorShellName(), host, token, certHash),
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
