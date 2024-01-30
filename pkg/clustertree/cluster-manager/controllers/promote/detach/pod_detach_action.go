package detach

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
)

// SecretAction is a restore item action for secrets
type PodAction struct {
}

func NewPodAction() *PodAction {
	return &PodAction{}
}

func (p *PodAction) Resource() string {
	return "pods"
}

func (p *PodAction) Execute(obj *unstructured.Unstructured, client client.Dynamic) error {
	updatedPod := new(corev1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, updatedPod); err != nil {
		return err
	}

	labels := updatedPod.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	if _, ok := labels["kosmos-io/pod"]; ok {
		return nil
	} else {
		labels["kosmos-io/pod"] = "true"
		podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPod)
		if err != nil {
			return errors.Wrap(err, "unable to convert pod to unstructured item")
		}

		patchBytes, err := generatePatch(obj, &unstructured.Unstructured{Object: podMap})
		if err != nil {
			return errors.Wrap(err, "error generating patch")
		}
		if patchBytes == nil {
			klog.Warningf("the same pod obj, %s", updatedPod.Name)
		}

		_, err = client.Patch(updatedPod.Name, patchBytes)
		return err
	}
}
