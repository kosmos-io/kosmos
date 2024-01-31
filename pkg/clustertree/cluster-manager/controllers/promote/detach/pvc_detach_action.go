package detach

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
)

// SecretAction is a restore item action for secrets
type PvcAction struct {
	logger logrus.FieldLogger
}

func NewPvcAction() *PvcAction {
	return &PvcAction{}
}

func (p *PvcAction) Resource() string {
	return "persistentvolumeclaims"
}

func (p *PvcAction) Execute(obj *unstructured.Unstructured, client client.Dynamic) error {
	updatedPvc := new(corev1.PersistentVolumeClaim)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, updatedPvc); err != nil {
		return err
	}

	annotations := updatedPvc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations["kosmos.io/global"]; ok {
		return nil
	} else {
		annotations["kosmos.io/global"] = "true"

		pvcMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPvc)
		if err != nil {
			return errors.Wrap(err, "unable to convert pvc to unstructured item")
		}

		patchBytes, err := generatePatch(obj, &unstructured.Unstructured{Object: pvcMap})
		if err != nil {
			return errors.Wrap(err, "error generating patch")
		}
		if patchBytes == nil {
			p.logger.Warnf("the same pvc obj, %s", updatedPvc.Name)
		}

		_, err = client.Patch(updatedPvc.Name, patchBytes)
		return err
	}
}
