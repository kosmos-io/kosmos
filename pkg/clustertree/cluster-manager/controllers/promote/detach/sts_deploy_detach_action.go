package detach

import (
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/client"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/promote/utils/kube"
)

type StsDeployAction struct {
}

func NewStsDeployAction() *StsDeployAction {
	return &StsDeployAction{}
}

func (p *StsDeployAction) Resource() []string {
	return []string{"statefulsets.apps", "deployments.apps", "replicasets.apps"}
}

func (p *StsDeployAction) Execute(obj *unstructured.Unstructured, client client.Dynamic, detacher *kubernetesDetacher) error {
	//级联删除sts、deployment、replicaset等
	orphanOption := metav1.DeletePropagationOrphan
	err := client.Delete(obj.GetName(), metav1.DeleteOptions{PropagationPolicy: &orphanOption})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("resource %s not found, skip delete", obj.GetName())
			return nil
		} else {
			return errors.Wrap(err, "DeletePropagationOrphan err")
		}
	}
	return nil
}

func (p *StsDeployAction) Revert(obj *unstructured.Unstructured, client client.Dynamic, detacher *kubernetesDetacher) error {
	newObj := obj.DeepCopy()
	newObj, err := kube.ResetMetadataAndStatus(newObj)
	if err != nil {
		return errors.Wrapf(err, "reset %s %s metadata error", obj.GroupVersionKind().String(), obj.GetName())
	}

	_, err = client.Create(newObj)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Infof("resource %s is already exist. skip create", newObj.GetName())
			return nil
		}
		return errors.Wrap(err, "create resource "+newObj.GetName()+" failed.")
	}
	time.Sleep(5 * time.Second)
	return nil
}
