package backup

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/kuberesource"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/requests"
)

type PVCAction struct {
}

func NewPVCAction() *PVCAction {
	return &PVCAction{}
}

func (p *PVCAction) Resource() string {
	return "persistentvolumeclaims"
}

func (s *PVCAction) Execute(item runtime.Unstructured, backup *kubernetesBackupper) (runtime.Unstructured, []requests.ResourceIdentifier, error) {
	var pvc corev1.PersistentVolumeClaim

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &pvc); err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert unstructured item to persistent volume claim")
	}

	if pvc.Status.Phase != corev1.ClaimBound || pvc.Spec.VolumeName == "" {
		return item, nil, nil
	}

	pv := requests.ResourceIdentifier{
		GroupResource: kuberesource.PersistentVolumes,
		Name:          pvc.Spec.VolumeName,
	}

	return item, []requests.ResourceIdentifier{pv}, nil
}
