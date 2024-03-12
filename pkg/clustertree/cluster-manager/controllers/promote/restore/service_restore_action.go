package restore

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServiceAction struct {
}

func NewServiceAction() *ServiceAction {
	return &ServiceAction{}
}

func (p *ServiceAction) Resource() []string {
	return []string{"services"}
}

func (p *ServiceAction) Execute(obj *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedService := new(corev1.Service)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, updatedService); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to service")
	}

	if updatedService.Spec.ClusterIP != "None" {
		updatedService.Spec.ClusterIP = ""
		updatedService.Spec.ClusterIPs = nil
	}

	annotations := updatedService.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations["kosmos.io/auto-create-mcs"]; !ok {
		annotations["kosmos.io/auto-create-mcs"] = "true"
	}
	updatedService.SetAnnotations(annotations)

	serviceMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedService)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert pod to unstructured item")
	}
	return &unstructured.Unstructured{Object: serviceMap}, nil
}

func (p *ServiceAction) Revert(fromCluster *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedService := new(corev1.Service)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(fromCluster.Object, updatedService); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to service")
	}

	annotations := updatedService.GetAnnotations()
	if annotations != nil {
		if _, ok := annotations["kosmos.io/auto-create-mcs"]; ok {
			delete(annotations, "kosmos.io/auto-create-mcs")
			updatedService.SetAnnotations(annotations)
			serviceMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedService)
			if err != nil {
				return nil, errors.Wrap(err, "unable to convert service to unstructured item")
			}
			return &unstructured.Unstructured{Object: serviceMap}, nil
		}
	}

	return fromCluster, nil
}
