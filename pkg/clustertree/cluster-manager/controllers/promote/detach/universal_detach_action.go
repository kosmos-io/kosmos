package detach

import (
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
)

type UniversalAction struct {
}

func NewUniversalAction() *UniversalAction {
	return &UniversalAction{}
}

func (p *UniversalAction) Resource() []string {
	return []string{"services", "persistentvolumeclaims", "persistentvolumes", "configmaps", "secrets", "serviceaccounts",
		"roles.rbac.authorization.k8s.io", "rolebindings.rbac.authorization.k8s.io"}
}

func (p *UniversalAction) Execute(obj *unstructured.Unstructured, client client.Dynamic, detacher *kubernetesDetacher) error {
	updatedObj := obj.DeepCopy()
	objectMeta, err := meta.Accessor(updatedObj)
	if err != nil {
		return errors.WithStack(err)
	}

	annotations := objectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	var key, val string
	if obj.GetKind() == "Service" {
		key = "kosmos.io/auto-create-mcs"
		val = "true"
	} else {
		key = "kosmos-io/cluster-owners"
		val = detacher.kosmosClusterName
	}

	_, ok := annotations[key]
	if updatedObj.GetOwnerReferences() != nil || !ok {
		annotations[key] = val
		updatedObj.SetAnnotations(annotations)
		updatedObj.SetOwnerReferences(nil)
		patchBytes, err := generatePatch(obj, updatedObj)
		if err != nil {
			return errors.Wrap(err, "error generating patch")
		}
		if patchBytes == nil {
			klog.Warningf("the same obj, %s", objectMeta.GetName())
		}

		_, err = client.Patch(objectMeta.GetName(), patchBytes)
		return err
	}

	return nil
}

//nolint:gosec // No need to check.
func (p *UniversalAction) Revert(obj *unstructured.Unstructured, client client.Dynamic, detacher *kubernetesDetacher) error {
	fromCluster, err := client.Get(obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("resource %s not found. skip undetach", obj.GroupVersionKind().String(), obj.GetName())
			return nil
		} else {
			return errors.Wrapf(err, "get resource %s %s failed.", obj.GroupVersionKind().String(), obj.GetName())
		}
	}

	updatedObj := fromCluster.DeepCopy()
	objectMeta, err := meta.Accessor(updatedObj)
	if err != nil {
		return errors.WithStack(err)
	}

	annotations := objectMeta.GetAnnotations()
	if annotations != nil {
		var key string
		if obj.GetKind() == "Service" {
			key = "kosmos.io/auto-create-mcs"
		} else {
			key = "kosmos-io/cluster-owners"
		}

		if _, ok := annotations[key]; ok {
			delete(annotations, key)
			updatedObj.SetAnnotations(annotations)
			patchBytes, err := generatePatch(fromCluster, updatedObj)
			if err != nil {
				return errors.Wrap(err, "error generating patch")
			}
			if patchBytes == nil {
				klog.Warningf("the same obj, %s", objectMeta.GetName())
				return nil
			}

			_, err = client.Patch(objectMeta.GetName(), patchBytes)
			return err
		}
	}

	return nil
}
