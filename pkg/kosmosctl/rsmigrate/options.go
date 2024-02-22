package rsmigrate

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type LeafClusterOptions struct {
	LeafClusterName         string
	LeafCluster             *v1alpha1.Cluster
	LeafClusterNativeClient kubernetes.Interface
	//clientset operate leafCluster releted resource
	LeafClusterKosmosClient kosmosversioned.Interface
}

type CommandOptions struct {
	MasterKubeConfig string
	MasterContext    string
	MasterClient     kubernetes.Interface
	//clientset operate leafCluster releted resource
	MasterKosmosClient kosmosversioned.Interface

	SrcLeafClusterOptions *LeafClusterOptions
	Namespace             string
}

func (o *CommandOptions) Complete(cmd *cobra.Command) error {
	config, err := utils.RestConfig(o.MasterKubeConfig, o.MasterContext)
	if err != nil {
		return fmt.Errorf("get master kubeconfig failed: %s", err)
	}

	masterClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create master clientset error: %s ", err)
	}
	o.MasterClient = masterClient

	kosmosClient, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("get master rest client config error:%s", err)
	}

	o.MasterKosmosClient = kosmosClient

	// get src leafCluster options
	if cmd.Flags().Changed("leafcluster") {
		err := completeLeafClusterOptions(o.SrcLeafClusterOptions, o.MasterKosmosClient)
		if err != nil {
			return err
		}
	}
	return nil
}
