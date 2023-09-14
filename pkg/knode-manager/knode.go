package knodemanager

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	klogv2 "k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
	k8sadapter "github.com/kosmos.io/kosmos/pkg/knode-manager/adapters/k8s"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
)

type Knode struct {
	podController  *controllers.PodController
	nodeController *controllers.NodeController
}

func NewKnode(ctx context.Context, knode *kosmosv1alpha1.Knode, c *config.Opts) (*Knode, error) {
	if len(knode.Spec.Kubeconfig) == 0 {
		return nil, fmt.Errorf("kubeconfig of knode %s is empty", knode.Name)
	}

	mClient, err := utils.NewClientFromConfigPath(c.KubeConfigPath, func(config *rest.Config) {
		config.QPS = c.KubeAPIQPS
		config.Burst = c.KubeAPIBurst
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for master cluster: %v", err)
	}

	wClient, err := utils.NewClientFromBytes(knode.Spec.Kubeconfig, func(config *rest.Config) {
		config.QPS = knode.Spec.KubeAPIQPS
		config.Burst = knode.Spec.KubeAPIBurst
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for worker cluster %s: %v", knode.Name, err)
	}

	var podAdapter adapters.PodHandler
	var nodeAdapter adapters.NodeHandler
	if knode.Spec.Type == kosmosv1alpha1.K8sAdapter {
		initConfig := k8sadapter.PodAdapterConfig{}
		podAdapter, err = k8sadapter.NewPodAdapter(initConfig, "", &k8sadapter.ClientConfig{}, true)
		if err != nil {
			return nil, err
		}

		nodeAdapter, err = k8sadapter.NewNodeAdapter(ctx, knode, wClient, mClient, c)
		if err != nil {
			return nil, err
		}
	}

	dummyNode := controllers.BuildDummyNode(ctx, knode, nodeAdapter)
	nc, err := controllers.NewNodeController(nodeAdapter, mClient, dummyNode)
	if err != nil {
		return nil, err
	}

	pc, err := controllers.NewPodController(controllers.PodConfig{
		PodHandler: podAdapter,
	})
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
