package knodemanager

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	klogv2 "k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
	k8sadapter "github.com/kosmos.io/kosmos/pkg/knode-manager/adapters/k8s"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/controllers"
)

type Knode struct {
	podController  *controllers.PodController
	nodeController *controllers.NodeController
}

func NewKnode(_ context.Context, knode *kosmosv1alpha1.Knode, c *config.Opts) (*Knode, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build master kubeconfig: %v", err)
	}
	kubeconfig.QPS, kubeconfig.Burst = c.KubeAPIQPS, c.KubeAPIBurst
	mClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to new master clientset: %v", err)
	}

	var podAdapter adapters.PodHandler
	var nodeAdapter adapters.NodeHandler
	if knode.Spec.Type == kosmosv1alpha1.K8sAdapter {
		podAdapter, err = k8sadapter.NewPodAdapter()
		if err != nil {
			return nil, err
		}
		nodeAdapter, err = k8sadapter.NewNodeAdapter()
		if err != nil {
			return nil, err
		}
	}

	pc, err := controllers.NewPodController(controllers.PodConfig{
		PodHandler: podAdapter,
	})
	if err != nil {
		return nil, err
	}

	nc, err := controllers.NewNodeController(nodeAdapter, mClient)
	if err != nil {
		return nil, err
	}

	return &Knode{
		podController:  pc,
		nodeController: nc,
	}, nil
}

func (kn *Knode) Run(ctx context.Context, c *config.Opts) {
	go func() {
		if err := kn.podController.Run(ctx, c.PodSyncWorkers); err != nil && errors.Cause(err) != context.Canceled {
			klogv2.Fatal(err)
		}
	}()
}
