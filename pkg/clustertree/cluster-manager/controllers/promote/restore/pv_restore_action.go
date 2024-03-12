package restore

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/kuberesource"
)

type PvAction struct {
}

func NewPvAction() *PvAction {
	return &PvAction{}
}

func (p *PvAction) Resource() []string {
	return []string{"persistentvolumes"}
}

func (p *PvAction) Execute(obj *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedPv := new(corev1.PersistentVolume)

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), updatedPv); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to pv")
	}

	claimRef := updatedPv.Spec.ClaimRef
	if claimRef != nil {
		gvr, resource, err := restorer.discoveryHelper.ResourceFor(kuberesource.PersistentVolumeClaims.WithVersion(""))
		if err != nil {
			return nil, errors.Errorf("Error getting resolved resource for %s", kuberesource.PersistentVolumeClaims)
		}

		client, err := restorer.dynamicFactory.ClientForGroupVersionResource(gvr.GroupVersion(), resource, claimRef.Namespace)
		if err != nil {
			return nil, err
		}

		pvcObj, err := client.Get(claimRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Errorf("Error get pvc %s, %v", claimRef.Name, err)
		}

		claimRef.ResourceVersion = pvcObj.GetResourceVersion()
		claimRef.UID = pvcObj.GetUID()

		pvMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPv)
		if err != nil {
			return nil, errors.Wrap(err, "unable to convert pod to unstructured item")
		}
		return &unstructured.Unstructured{Object: pvMap}, nil
	} else {
		return obj, nil
	}
}

func (p *PvAction) Revert(fromCluster *unstructured.Unstructured, restorer *kubernetesRestorer) (*unstructured.Unstructured, error) {
	updatedPv := new(corev1.PersistentVolume)

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(fromCluster.UnstructuredContent(), updatedPv); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to pv")
	}

	annotations := updatedPv.Annotations
	if annotations != nil {
		if _, ok := annotations["kosmos-io/cluster-owners"]; ok {
			delete(annotations, "kosmos-io/cluster-owners")
			pvMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&updatedPv)
			if err != nil {
				return nil, errors.Wrap(err, "unable to convert pod to unstructured item")
			}
			return &unstructured.Unstructured{Object: pvMap}, nil
		}
	}

	return fromCluster, nil
}
