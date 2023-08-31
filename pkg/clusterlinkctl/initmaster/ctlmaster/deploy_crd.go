package ctlmaster

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (i *CommandInitOption) deployClusterlinkCRDs(crd *apiextensionsv1.CustomResourceDefinition,
	crdName string) error {
	// Create CRD
	_, err := i.ExtensionKubeClientSet.ApiextensionsV1().CustomResourceDefinitions().
		Create(context.Background(), crd, metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create crd %s error : %v ", crdName, err)
		} else {
			klog.Info("clusterlink  CRD already exists, Update it")
			resourceVersion, err := i.ExtensionKubeClientSet.ApiextensionsV1().CustomResourceDefinitions().
				Get(context.Background(), crdName, metav1.GetOptions{})

			if err != nil {
				klog.Errorf("get crd %s %v", crdName, err)
				return fmt.Errorf("get crd %s %v", crdName, err)
			}
			crd.ObjectMeta.ResourceVersion = resourceVersion.ResourceVersion
			if _, err := i.ExtensionKubeClientSet.ApiextensionsV1().CustomResourceDefinitions().
				Update(context.Background(), crd, metav1.UpdateOptions{}); err != nil {
				klog.Errorf("update crd %s error : %v ", crdName, err)
				return fmt.Errorf("update crd %s error : %v ", crdName, err)
			}
		}
	}

	return nil
}

func (i *CommandInitOption) initClusterlinkCRDs() error {
	crdFuncList := map[string](func() (*apiextensionsv1.CustomResourceDefinition, error)){
		"clusternodes.kosmos.io": MakeCRD(ClusterNode),
		"clusters.kosmos.io":     MakeCRD(Cluster),
		"nodeconfigs.kosmos.io":  MakeCRD(NodeConfig),
	}
	for crdName, crdFunc := range crdFuncList {
		crd, err := crdFunc()
		if err != nil {
			return err
		}
		err = i.deployClusterlinkCRDs(crd, crdName)
		if err != nil {
			return err
		}
	}
	return nil
}
