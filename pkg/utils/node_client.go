package utils

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

type ClusterKubeClient struct {
	KubeClient  kubernetes.Interface
	ClusterName string
}

type ClusterKosmosClient struct {
	KosmosClient kosmosversioned.Interface
	ClusterName  string
}

type ClusterDynamicClient struct {
	DynamicClient dynamic.Interface
	ClusterName   string
}

// NewClusterKubeClient create a kube client for a member cluster
func NewClusterKubeClient(config *rest.Config, ClusterName string, opts Opts) (*ClusterKubeClient, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ClusterKubeClient{
		KubeClient:  kubeClient,
		ClusterName: ClusterName,
	}, nil
}

// NewClusterKosmosClient create a dynamic client for a member cluster
func NewClusterKosmosClient(config *rest.Config, ClusterName string, opts Opts) (*ClusterKosmosClient, error) {
	kosmosClient, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ClusterKosmosClient{
		KosmosClient: kosmosClient,
		ClusterName:  ClusterName,
	}, nil
}

// NewClusterDynamicClient create a kosmos crd client for a member cluster
func NewClusterDynamicClient(config *rest.Config, ClusterName string, opts Opts) (*ClusterDynamicClient, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &ClusterDynamicClient{
		DynamicClient: dynamicClient,
		ClusterName:   ClusterName,
	}, nil
}

func BuildConfig(cluster *kosmosv1alpha1.Cluster, opts Opts) (*rest.Config, error) {
	config, err := NewConfigFromBytes(cluster.Spec.Kubeconfig, opts)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetCluster returns the member cluster
func GetCluster(hostClient client.Client, clusterName string) (*kosmosv1alpha1.Cluster, error) {
	cluster := &kosmosv1alpha1.Cluster{}
	if err := hostClient.Get(context.TODO(), types.NamespacedName{Name: clusterName}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}
