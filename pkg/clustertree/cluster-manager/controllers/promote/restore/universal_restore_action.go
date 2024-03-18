package restore

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type UniversalAction struct {
}

func NewUniversalAction() *UniversalAction {
	return &UniversalAction{}
}

func (p *UniversalAction) Resource() []string {
	return []string{"persistentvolumeclaims", "configmaps", "secrets", "serviceaccounts", "roles.rbac.authorization.k8s.io", "rolebindings.rbac.authorization.k8s.io"}
}

func (p *UniversalAction) Execute(obj *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedObj := obj.DeepCopy()
	objectMeta, err := meta.Accessor(updatedObj)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	annotations := objectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if _, ok := annotations["kosmos-io/cluster-owners"]; !ok {
		annotations["kosmos-io/cluster-owners"] = restorer.kosmosClusterName
		updatedObj.SetAnnotations(annotations)
	}

	return updatedObj, nil
}

func (p *UniversalAction) Revert(fromCluster *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedObj := fromCluster.DeepCopy()
	objectMeta, err := meta.Accessor(updatedObj)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	annotations := objectMeta.GetAnnotations()
	if annotations != nil {
		if _, ok := annotations["kosmos-io/cluster-owners"]; ok {
			delete(annotations, "kosmos-io/cluster-owners")
			updatedObj.SetAnnotations(annotations)
		}
	}

	return updatedObj, nil
}
