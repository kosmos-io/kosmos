package rootpodsyncers

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
	BeforeCreateInLeaf(context.Context, *K8sSyncer, *leafUtils.LeafResource, *unstructured.Unstructured, *corev1.Pod, *leafUtils.ClusterNode) error
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

func (c *ConfigMapHandler) BeforeCreateInLeaf(context.Context, *K8sSyncer, *leafUtils.LeafResource, *unstructured.Unstructured, *corev1.Pod, *leafUtils.ClusterNode) error {
	return nil
}

type SecretHandler struct {
}

func (s *SecretHandler) BeforeCreateInLeaf(ctx context.Context, r *K8sSyncer, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, _ *leafUtils.ClusterNode) error {
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

func (v *PVCHandler) BeforeCreateInLeaf(_ context.Context, _ *K8sSyncer, lr *leafUtils.LeafResource, unstructuredObj *unstructured.Unstructured, rootpod *corev1.Pod, cn *leafUtils.ClusterNode) error {
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
