package utils

import (
	"context"
	"fmt"
	"math"
	"time"

	perrors "github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func EnsureCRDFromRootClusterToLeafCluster(ctx context.Context, rootConfig *rest.Config, leafConfig *rest.Config, gvk schema.GroupVersionKind) error {
	//Check whether the CRD exists in the leafCluster
	exists, err := KindExists(leafConfig, gvk)
	if err != nil {
		return err
	}
	//if hava , return
	if exists {
		return nil
	}

	//no，get gvr
	gvr, err := ConvertKindToResource(rootConfig, gvk)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("the resource %s  is not found", gvr.String())
		}
		return err
	}

	//Use gvk to find the crd
	rootClient, err := apiextensionsv1clientset.NewForConfig(rootConfig)
	if err != nil {
		return err
	}
	crd, err := rootClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, gvr.GroupResource().String(), metav1.GetOptions{})
	if err != nil {
		return perrors.Wrap(err, "retrieve crd in host cluster")
	}
	crd.UID = ""
	crd.ResourceVersion = ""
	crd.ManagedFields = nil
	crd.OwnerReferences = nil
	crd.Spec.PreserveUnknownFields = false
	crd.Status = apiextensionsv1.CustomResourceDefinitionStatus{}
	leafClient, err := apiextensionsv1clientset.NewForConfig(leafConfig)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Create crd %s in leaf cluster", gvk.String())
	_, err = leafClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
	if err != nil {
		return perrors.Wrap(err, "create crd in virtual cluster")
	}

	klog.V(4).Infof("Wait for crd %s to become ready in virtual cluster", gvk.String())
	err = wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
		Duration: time.Second,
		Factor:   1.5,
		Steps:    math.MaxInt32,
		Cap:      time.Minute,
	}, func() (bool, error) {
		crd, err := leafClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, gvr.GroupResource().String(), metav1.GetOptions{})
		if err != nil {
			return false, perrors.Wrap(err, "retrieve crd in root cluster")
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1.Established:
				if cond.Status == apiextensionsv1.ConditionTrue {
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return perrors.Wrap(err, "wait for crd to become ready in virtual cluster")
	}
	return nil
}

func ConvertKindToResource(config *rest.Config, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	resources, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	for _, resource := range resources.APIResources {
		if resource.Kind == gvk.Kind {
			return schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: resource.Name,
			}, nil
		}
	}

	return schema.GroupVersionResource{}, nil
}

func KindExists(config *rest.Config, gvk schema.GroupVersionKind) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false, err
	}
	resources, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, resource := range resources.APIResources {
		if resource.Kind == gvk.Kind {
			return true, nil
		}
	}

	return false, nil
}
