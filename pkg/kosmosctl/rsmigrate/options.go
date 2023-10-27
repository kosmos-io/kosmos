package rsmigrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/homedir"
	ctlutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type LeafClusterOptions struct {
	LeafClusterName         string
	LeafCluster             *v1alpha1.Knode
	LeafClusterNativeClient kubernetes.Interface
	//clientset operate leafCluster releted resource
	LeafClusterKosmosClient kosmosversioned.Interface
}

type CommandOptions struct {
	MasterKubeConfig string
	MasterClient     kubernetes.Interface
	//clientset operate leafCluster releted resource
	MasterKosmosClient kosmosversioned.Interface

	SrcLeafClusterOptions *LeafClusterOptions
	Namespace             string
}

func (o *CommandOptions) Validate(cmd *cobra.Command) error {
	return nil
}

func (o *CommandOptions) Complete(f ctlutil.Factory, cmd *cobra.Command) error {
	var err error
	var kubeConfigStream []byte
	// get master kubernetes clientset
	if len(o.MasterKubeConfig) > 0 {
		kubeConfigStream, err = os.ReadFile(o.MasterKubeConfig)
	} else {
		kubeConfigStream, err = os.ReadFile(filepath.Join(homedir.HomeDir(), ".kube", "config"))
	}
	if err != nil {
		return fmt.Errorf("get master kubeconfig failed: %s", err)
	}

	masterClient, err := utils.NewClientFromBytes(kubeConfigStream)
	if err != nil {
		return fmt.Errorf("create master clientset error: %s ", err)
	}
	o.MasterClient = masterClient

	kosmosClient, err := utils.NewKosmosClientFromBytes(kubeConfigStream)
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
