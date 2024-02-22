package pod

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	leafUtils "github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type StorageHandler interface {
	BeforeCreateInLeaf(context.Context, *RootPodReconciler, *leafUtils.LeafResource, *unstructured.Unstructured, *corev1.Pod, *leafUtils.ClusterNode) error
	BeforeGetInLeaf(ctx context.Context, r *RootPodReconciler, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, _ *leafUtils.ClusterNode) error
}

func NewStorageHandler(gvr schema.GroupVersionResource) (StorageHandler, error) {
	switch gvr.Resource {
	case utils.GVR_CONFIGMAP.Resource:
		return &ConfigMapHandler{}, nil
	case utils.GVR_SECRET.Resource:
		return &SecretHandler{}, nil
	case utils.GVR_PVC.Resource:
		return &PVCHandler{}, nil
	}
	return nil, fmt.Errorf("gvr type is not match when create storage handler")
}

type ConfigMapHandler struct {
}

// BeforeGetInLeaf The name of the host cluster kube-root-ca.crt in the leaf group is master-root-ca.crt
func (c *ConfigMapHandler) BeforeGetInLeaf(ctx context.Context, r *RootPodReconciler, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, _ *leafUtils.ClusterNode) error {
	if unstructuredObj.GetName() == utils.RooTCAConfigMapName {
		unstructuredObj.SetName(utils.MasterRooTCAName)
		klog.V(4).Infof("Modify the name of the configmap for the CA: %s", utils.MasterRooTCAName)
	}
	return nil
}

func (c *ConfigMapHandler) BeforeCreateInLeaf(ctx context.Context, r *RootPodReconciler, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, _ *leafUtils.ClusterNode) error {
	return nil
}

type SecretHandler struct {
}

func (s *SecretHandler) BeforeGetInLeaf(ctx context.Context, r *RootPodReconciler, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, _ *leafUtils.ClusterNode) error {
	return nil
}

func (s *SecretHandler) BeforeCreateInLeaf(ctx context.Context, r *RootPodReconciler, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, _ *leafUtils.ClusterNode) error {
	secretObj := &corev1.Secret{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, secretObj)
	if err != nil {
		panic(err.Error())
	}
	if secretObj.Type == corev1.SecretTypeServiceAccountToken {
		if err := r.createServiceAccountInLeafCluster(ctx, lr, secretObj); err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

type PVCHandler struct {
}

func (v *PVCHandler) BeforeGetInLeaf(_ context.Context, _ *RootPodReconciler, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, cn *leafUtils.ClusterNode) error {
	return nil
}

func (v *PVCHandler) BeforeCreateInLeaf(_ context.Context, _ *RootPodReconciler, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, cn *leafUtils.ClusterNode) error {
	if rootpod == nil || len(rootpod.Spec.NodeName) == 0 {
		return nil
	}
	annotationsMap := unstructuredObj.GetAnnotations()
	if annotationsMap == nil {
		annotationsMap = map[string]string{}
	}
	// TODO: rootpod.Spec.NodeSelector
	if cn.LeafMode == leafUtils.Node {
		annotationsMap[utils.PVCSelectedNodeKey] = rootpod.Spec.NodeName
	}

	unstructuredObj.SetAnnotations(annotationsMap)

	return nil
}
