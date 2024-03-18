package restore

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodAction struct {
}

func NewPodAction() *PodAction {
	return &PodAction{}
}

func (p *PodAction) Resource() []string {
	return []string{"pods"}
}

func (p *PodAction) Execute(obj *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedPod := new(corev1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, updatedPod); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to pod")
	}

	updatedPod.Spec.NodeName = restorer.kosmosNodeName

	kosmosNodeToleration := corev1.Toleration{
		Key:      "kosmos.io/node",
		Value:    "true",
		Operator: corev1.TolerationOpEqual,
		Effect:   corev1.TaintEffectNoSchedule,
	}
	tolerations := updatedPod.Spec.Tolerations
	if tolerations == nil {
		tolerations = make([]corev1.Toleration, 1)
		tolerations[0] = kosmosNodeToleration
	} else {
		kosmosTolerationExist := false
		for _, toleration := range tolerations {
			if toleration.Key == "kosmos.io/node" {
				kosmosTolerationExist = true
				break
			}
		}
		if !kosmosTolerationExist {
			updatedPod.Spec.Tolerations = append(updatedPod.Spec.Tolerations, kosmosNodeToleration)
		}
	}

	labels := updatedPod.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["kosmos-io/pod"] = "true"
	labels["kosmos-io/synced"] = "true"
	updatedPod.SetLabels(labels)

	podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPod)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert pod to unstructured item")
	}
	return &unstructured.Unstructured{Object: podMap}, nil
}

func (p *PodAction) Revert(fromCluster *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedPod := new(corev1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(fromCluster.Object, updatedPod); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to pod")
	}

	labels := updatedPod.GetLabels()
	if labels != nil {
		if _, ok := labels["kosmos-io/pod"]; ok {
			delete(labels, "kosmos-io/pod")
			updatedPod.SetLabels(labels)
			podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPod)
			if err != nil {
				return nil, errors.Wrap(err, "unable to convert pod to unstructured item")
			}
			return &unstructured.Unstructured{Object: podMap}, nil
		}
	}

	return fromCluster, nil
}
