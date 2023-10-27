package rsmigrate

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

func getClientFromLeafCluster(leafCluster *v1alpha1.Knode) (kubernetes.Interface, kosmosversioned.Interface, error) {
	//generate clientset by leafCluster kubeconfig
	leafClusterKubeconfig := leafCluster.Spec.Kubeconfig
	if len(leafClusterKubeconfig) == 0 {
		return nil, nil, fmt.Errorf("leafcluster's kubeconfig is nil, it's unable to work normally")
	}
	k8sClient, err := utils.NewClientFromBytes(leafClusterKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create kubernetes clientset error: %s ", err)
	}

	kosmosClient, err := utils.NewKosmosClientFromBytes(leafClusterKubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create kubernetes clientset for leafcluster crd error: %s ", err)
	}

	return k8sClient, kosmosClient, nil
}

func completeLeafClusterOptions(leafClusterOptions *LeafClusterOptions, masterClient kosmosversioned.Interface) error {
	//complete leafClusterOptions by leafCluster name
	if leafClusterOptions.LeafClusterName == "" {
		return fmt.Errorf("get leafcluster error: %s ", "leafcluster value can't be empty")
	}
	leafCluster, err := masterClient.KosmosV1alpha1().Knodes().Get(context.TODO(), leafClusterOptions.LeafClusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get leafcluster error: %s", err)
	}
	leafClusterOptions.LeafCluster = leafCluster
	k8sClient, kosmosClient, err := getClientFromLeafCluster(leafClusterOptions.LeafCluster)
	if err != nil {
		return fmt.Errorf("get leafcluster clientset error: %s", err)
	}
	leafClusterOptions.LeafClusterNativeClient = k8sClient
	leafClusterOptions.LeafClusterKosmosClient = kosmosClient

	return nil
}
