package detach

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (p *PodAction) Resource() []string {
	return []string{"pods"}
}

func (p *PodAction) Execute(obj *unstructured.Unstructured, client client.Dynamic, detacher *kubernetesDetacher) error {
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
			return nil
		}

		_, err = client.Patch(updatedPod.Name, patchBytes)
		return err
	}
}

func (p *PodAction) Revert(obj *unstructured.Unstructured, client client.Dynamic, detacher *kubernetesDetacher) error {
	fromCluster, err := client.Get(obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("resource %s %s not found. skip undetach", obj.GroupVersionKind().String(), obj.GetName())
			return nil
		} else {
			return errors.Wrapf(err, "get resource %s %s failed.", obj.GroupVersionKind().String(), obj.GetName())
		}
	}

	updatedPod := new(corev1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(fromCluster.Object, updatedPod); err != nil {
		return err
	}
	labels := updatedPod.GetLabels()
	if labels != nil {
		if _, ok := labels["kosmos-io/pod"]; ok {
			delete(labels, "kosmos-io/pod")
			updatedPod.SetLabels(labels)
			podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPod)
			if err != nil {
				return errors.Wrap(err, "unable to convert pod to unstructured item")
			}
			patchBytes, err := generatePatch(fromCluster, &unstructured.Unstructured{Object: podMap})
			if err != nil {
				return errors.Wrap(err, "error generating patch")
			}
			if patchBytes == nil {
				klog.Warningf("the same pod obj, %s", updatedPod.Name)
				return nil
			}

			_, err = client.Patch(updatedPod.Name, patchBytes)
			return err
		}
	}

	return nil
}
