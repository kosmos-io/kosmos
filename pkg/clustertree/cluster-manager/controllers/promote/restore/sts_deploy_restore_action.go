package restore

import (
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type StsDeployAction struct {
}

func NewStsDeployAction() *StsDeployAction {
	return &StsDeployAction{}
}

func (p *StsDeployAction) Resource() []string {
	return []string{"statefulsets.apps", "deployments.apps"}
}

//nolint:gosec // No need to check.
func (p *StsDeployAction) Execute(obj *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	_ = &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/hostname",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{restorer.kosmosNodeName},
						},
					},
				},
			},
		},
	}

	kosmosToleration := corev1.Toleration{
		Key:      "kosmos.io/node",
		Operator: corev1.TolerationOpEqual,
		Value:    "true",
		Effect:   corev1.TaintEffectNoSchedule,
	}

	var updatedObj interface{}

	if obj.GetKind() == "Deployment" {
		updatedDeploy := new(appsv1.Deployment)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, updatedDeploy); err != nil {
			return nil, errors.Wrap(err, "unable to convert unstructured item to deployment")
		}

		//affinity := updatedDeploy.Spec.Template.Spec.Affinity
		//if affinity == nil {
		//	affinity = &corev1.Affinity{
		//		NodeAffinity: updatedNodeAffinity,
		//	}
		//} else {
		//	updatedDeploy.Spec.Template.Spec.Affinity.NodeAffinity = updatedNodeAffinity
		//}

		tolerations := updatedDeploy.Spec.Template.Spec.Tolerations
		if tolerations == nil {
			tolerations = make([]corev1.Toleration, 1)
			tolerations[0] = kosmosToleration
		} else {
			kosmosExist := false
			for _, toleration := range tolerations {
				if toleration.Key == kosmosToleration.Key {
					kosmosExist = true
					break
				}
			}

			if !kosmosExist {
				updatedDeploy.Spec.Template.Spec.Tolerations = append(tolerations, kosmosToleration)
			}
		}
		updatedObj = updatedDeploy
	} else if obj.GetKind() == "StatefulSet" {
		updatedSts := new(appsv1.StatefulSet)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, updatedSts); err != nil {
			return nil, errors.Wrap(err, "unable to convert unstructured item to statefulset")
		}

		//affinity := updatedSts.Spec.Template.Spec.Affinity
		//if affinity == nil {
		//	affinity = &corev1.Affinity{
		//		NodeAffinity: updatedNodeAffinity,
		//	}
		//} else {
		//	updatedSts.Spec.Template.Spec.Affinity.NodeAffinity = updatedNodeAffinity
		//}

		tolerations := updatedSts.Spec.Template.Spec.Tolerations
		if tolerations == nil {
			tolerations = make([]corev1.Toleration, 1)
			tolerations[0] = kosmosToleration
		} else {
			kosmosExist := false
			for _, toleration := range tolerations {
				if toleration.Key == kosmosToleration.Key {
					kosmosExist = true
					break
				}
			}

			if !kosmosExist {
				updatedSts.Spec.Template.Spec.Tolerations = append(tolerations, kosmosToleration)
			}
		}
		updatedObj = updatedSts
	} else {
		return nil, errors.Errorf("unknow obj kind %s", obj.GetKind())
	}

	stsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedObj)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert sts/deploy to unstructured item")
	}
	return &unstructured.Unstructured{Object: stsMap}, nil
}

func (p *StsDeployAction) Revert(fromCluster *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	return fromCluster, nil
}
