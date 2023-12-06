package rootpodsyncers

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/cmd/clustertree/cluster-manager/app/options"
	rootpodk8ssyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/root-pod/k8s"
	rootpodopenapisyncers "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/root-pod/serverless"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/runtime"
	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/podutils"
)

const (
	RootPodControllerName = "root-pod-controller"
	RootPodRequeueTime    = 10 * time.Second
)

type RootPodSyncer struct {
	Client             kubernetes.Interface
	GlobalLeafManager  leafUtils.LeafResourceManager
	EnvResourceManager utils.EnvResourceManager
	Options            *options.Options
	DynamicRootClient  dynamic.Interface
}

func (r *RootPodSyncer) GetSyncer(lr *leafUtils.LeafResource) (RootPodSyncerHandle, error) {
	var RootPodSyncer RootPodSyncerHandle
	switch lr.GetLeafType() {
	case leafUtils.LeafTypeK8s:
		RootPodSyncer = &rootpodk8ssyncers.K8sSyncer{
			RootClient:         r.Client,
			GlobalLeafManager:  r.GlobalLeafManager,
			EnvResourceManager: r.EnvResourceManager,
			Options:            r.Options,
			DynamicRootClient:  r.DynamicRootClient,
		}
	case leafUtils.LeafTypeServerless:
		RootPodSyncer = &rootpodopenapisyncers.ServerlessSyncer{
			RootClient: r.Client,
		}
	}
	if RootPodSyncer == nil {
		return nil, fmt.Errorf("not implement, DeletePodInLeafCluster")
	} else {
		return RootPodSyncer, nil
	}
}

func (r *RootPodSyncer) GetPodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName) (*corev1.Pod, error) {
	if syncer, err := r.GetSyncer(lr); err != nil {
		return nil, err
	} else {
		return syncer.GetPodInLeafCluster(ctx, lr, rootnamespacedname)
	}
}

func (r *RootPodSyncer) DeletePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootnamespacedname runtime.NamespacedName, cleanflag bool) error {
	if syncer, err := r.GetSyncer(lr); err != nil {
		return err
	} else {
		return syncer.DeletePodInLeafCluster(ctx, lr, rootnamespacedname, cleanflag)
	}
}

func (r *RootPodSyncer) CreatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, pod *corev1.Pod) error {
	if syncer, err := r.GetSyncer(lr); err != nil {
		return err
	} else {
		return syncer.CreatePodInLeafCluster(ctx, lr, pod)
	}
}
func (r *RootPodSyncer) UpdatePodInLeafCluster(ctx context.Context, lr *leafUtils.LeafResource, rootpod *corev1.Pod, leafpod *corev1.Pod) error {
	if syncer, err := r.GetSyncer(lr); err != nil {
		return err
	} else {
		return syncer.UpdatePodInLeafCluster(ctx, lr, rootpod, leafpod)
	}
}

func (r *RootPodSyncer) Reconcile(ctx context.Context, key runtime.NamespacedName) (runtime.Result, error) {
	cachepod, err := r.Client.CoreV1().Pods(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// TODO: we cannot get leaf pod when we donnot known the node name of pod, so delete all ...
			nodeNames := r.GlobalLeafManager.ListNodes()
			for _, nodeName := range nodeNames {
				lr, err := r.GlobalLeafManager.GetLeafResourceByNodeName(nodeName)
				if err != nil {
					// wait for leaf resource init
					return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
				}
				if err := r.DeletePodInLeafCluster(ctx, lr, key, false); err != nil {
					klog.Errorf("delete pod in leaf error[1]: %v,  %s", err, key)
					return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
				}
			}
			return runtime.Result{}, nil
		}
		klog.Errorf("get %s error: %v", key, err)
		return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
	}

	rootpod := *(cachepod.DeepCopy())

	// node filter
	if len(rootpod.Spec.NodeName) == 0 {
		return runtime.Result{}, nil
	}
	if !strings.HasPrefix(rootpod.Spec.NodeName, utils.KosmosNodePrefix) {
		// ignore the pod who donnot has the annotations "kosmos-io/owned-by-cluster"
		targetNode, err := r.Client.CoreV1().Nodes().Get(ctx, rootpod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
		}

		if targetNode.Annotations == nil {
			return runtime.Result{}, nil
		}

		clusterName := targetNode.Annotations[utils.KosmosNodeOwnedByClusterAnnotations]

		if len(clusterName) == 0 {
			return runtime.Result{}, nil
		}
	}

	// TODO: GlobalLeafResourceManager may not inited....
	// belongs to the current node
	if !r.GlobalLeafManager.HasNode(rootpod.Spec.NodeName) {
		return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
	}

	lr, err := r.GlobalLeafManager.GetLeafResourceByNodeName(rootpod.Spec.NodeName)
	if err != nil {
		// wait for leaf resource init
		return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
	}

	// skip namespace
	if len(lr.Namespace) > 0 && lr.Namespace != rootpod.Namespace {
		return runtime.Result{}, nil
	}

	// delete pod in leaf
	if !rootpod.GetDeletionTimestamp().IsZero() {
		if err := r.DeletePodInLeafCluster(ctx, lr, key, true); err != nil {
			klog.Errorf("delete pod in leaf error[1]: %v,  %s", err, key)
			return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
		return runtime.Result{}, nil
	}

	leafPod, err := r.GetPodInLeafCluster(ctx, lr, key)

	// create pod in leaf
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.CreatePodInLeafCluster(ctx, lr, &rootpod); err != nil {
				klog.Errorf("create pod inleaf error, err: %s", err)
				return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
			} else {
				return runtime.Result{}, nil
			}
		} else {
			klog.Errorf("get pod in leaf error[3]: %v,  %s", err, key)
			return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
	}

	// update pod in leaf
	if podutils.ShouldEnqueue(leafPod, &rootpod) {
		if err := r.UpdatePodInLeafCluster(ctx, lr, &rootpod, leafPod); err != nil {
			return runtime.Result{RequeueAfter: RootPodRequeueTime}, nil
		}
	}

	return runtime.Result{}, nil
}
