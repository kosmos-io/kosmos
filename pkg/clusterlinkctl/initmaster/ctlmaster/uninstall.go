package ctlmaster

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type KubeResourceToDel interface {
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func (i *CommandInitOption) RunDeInit(parentCommand string) error {

	deployments, err := i.retriveClusterLinkiDP()
	if err != nil {
		klog.Errorf("can't retrive deployment from kubernetes")
		return err
	}

	serviceaccounts, err := i.retriveClusterLinkSA()
	if err != nil {
		klog.Errorf("can't retrive serviceaccounts from kubernetes")
		return err
	}

	resources := append(deployments, serviceaccounts...)

	for _, rs := range resources {
		if i.Namespace == "" || i.Namespace == rs.Namespace {
			if err := rs.ResourceClient.Delete(context.TODO(), rs.Name,
				metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("delete %s %s error : %w", rs.Type, rs.Name, err)
				} else {
					klog.Infof("%s %s is not found", rs.Type, rs.Name)
				}
			} else {
				klog.Infof("%s %s is deletd", rs.Type, rs.Name)
			}
		}
	}

	return nil
}
