package restore

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kosmos.io/kosmos/pkg/utils"
)

type PodAction struct {
}

func NewPodAction() *PodAction {
	return &PodAction{}
}

func (p *PodAction) Resource() string {
	return "pods"
}

func (p *PodAction) Execute(obj *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedPod := new(corev1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, updatedPod); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to pod")
	}

	updatedPod.Spec.NodeName = restorer.clusterNodeName
	kosmosTolerationExist := false

	labels := updatedPod.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[utils.KosmosPodLabel] = "true"
	labels["kosmos/promoted"] = "true"

	tolerations := updatedPod.Spec.Tolerations
	if tolerations == nil {
		tolerations = make([]corev1.Toleration, 1)
	}

	for _, toleration := range tolerations {
		if toleration.Key == "kosmos.io/node" {
			kosmosTolerationExist = true
			break
		}
	}

	if !kosmosTolerationExist {
		kosmosNodeToleration := corev1.Toleration{
			Key:      "kosmos.io/node",
			Value:    "true",
			Operator: corev1.TolerationOpEqual,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		updatedPod.Spec.Tolerations = append(updatedPod.Spec.Tolerations, kosmosNodeToleration)
	}

	podMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPod)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert pod to unstructured item")
	}
	return &unstructured.Unstructured{Object: podMap}, nil
}
