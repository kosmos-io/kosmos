package task

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
	"github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/exector"
	"github.com/kosmos.io/kosmos/pkg/kubenest/util"
)

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

func NewCleanHostClusterNodeTask() Task {
	return Task{
		Name:  "clean host cluster node",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			targetNode := &v1.Node{}
			if err := to.HostK8sClient.Get(ctx, types.NamespacedName{
				Name: to.NodeInfo.Name,
			}, targetNode); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, fmt.Errorf("get target node %s failed: %s", to.NodeInfo.Name, err)
			}

			if err := to.HostK8sClient.Delete(ctx, targetNode); err != nil {
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
			if err := to.HostK8sClient.Get(ctx, nn, targetCert); err != nil {
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

func NewUpdateNodeLabelsTask() Task {
	return Task{
		Name:  "update new-node labels",
		Retry: true,
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			node, err := to.VirtualK8sClient.CoreV1().Nodes().Get(ctx, to.NodeInfo.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("get node %s failed: %s", to.NodeInfo.Name, err)
			}

			updateNode := node.DeepCopy()
			for k, v := range to.NodeInfo.Labels {
				node.Labels[k] = v
			}

			if _, err := to.VirtualK8sClient.CoreV1().Nodes().Update(ctx, updateNode, metav1.UpdateOptions{}); err != nil {
				return nil, fmt.Errorf("add label to node %s failed: %s", to.NodeInfo.Name, err)
			}
			return nil, nil
		},
	}
}

func NewUpdateNodePoolItemStatusTask(nodeState v1alpha1.NodeState, isClean bool) Task {
	return Task{
		Name: "Update node status in NodePool ",
		Run: func(ctx context.Context, to TaskOpt, _ interface{}) (interface{}, error) {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				targetGlobalNode := v1alpha1.GlobalNode{}

				if err := to.HostK8sClient.Get(ctx, types.NamespacedName{Name: to.NodeInfo.Name}, &targetGlobalNode); err != nil {
					klog.Errorf("get global node %s failed: %s", to.NodeInfo.Name, err)
					return err
				}

				updateGlobalNode := targetGlobalNode.DeepCopy()

				updateGlobalNode.Spec.State = nodeState
				if err := to.HostK8sClient.Update(ctx, updateGlobalNode); err != nil {
					klog.Errorf("update global node %s spec.state failed: %s", updateGlobalNode.Name, err)
					return err
				}
				if isClean {
					updateGlobalNode.Status.VirtualCluster = ""
					if err := to.HostK8sClient.Status().Update(ctx, updateGlobalNode); err != nil {
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
