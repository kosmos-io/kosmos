package floaterclient

import (
	"context"
	"fmt"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	clusterlinkv1alpha1 "github.com/kosmos.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
)

const (
	ClusterGroup    = "clusterlink.io"
	ClusterVersion  = "v1alpha1"
	ClusterResource = "clusters"
	ClusterKind     = "Cluster"

	ClusterNameIndex = "Name"
)

type HostClusterHelper struct {
	Kubeconfig    string
	dynamicClient *dynamic.DynamicClient
}

func (h *HostClusterHelper) Complete() error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", h.Kubeconfig)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %s", err.Error())
	}
	c, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	h.dynamicClient = c
	return nil
}

func (h *HostClusterHelper) GetClusterInfo(clusterName string) (*rest.Config, *kubernetes.Clientset, map[string]string, error) {

	gvr := schema.GroupVersionResource{
		Group:    ClusterGroup,
		Version:  ClusterVersion,
		Resource: ClusterResource,
	}
	listClusters, err := h.dynamicClient.Resource(gvr).List(context.TODO(), meta.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	clusterList := &clusterlinkv1alpha1.ClusterList{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(listClusters.UnstructuredContent(), clusterList); err != nil {
		return nil, nil, nil, err
	}

	/*
			for _, c := range clusterList.Items {
				if clusterName == c.GetName() {
					clientConfig, err := clientcmd.NewClientConfigFromBytes(c.Spec.Kubeconfig)
					if err != nil {
						return nil, nil, nil, err
					}
					config, err := clientConfig.ClientConfig()
					if err != nil {
						return nil, nil, nil, err
					}
					clusterClientSet, err := kubernetes.NewForConfig(config)
					if err != nil {
						return nil, nil, nil, err
					}
					return config, clusterClientSet, c.Spec.CIDRsMap, nil
				}
		}
	*/
	return nil, nil, nil, fmt.Errorf("cannot find cluster: %s", clusterName)

}
