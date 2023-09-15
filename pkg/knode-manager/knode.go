package knodemanager

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	klogv2 "k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
	k8sadapter "github.com/kosmos.io/kosmos/pkg/knode-manager/adapters/k8s"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils/manager"
)

type Knode struct {
	client kubernetes.Interface
	master kubernetes.Interface

	podController  *controllers.PodController
	nodeController *controllers.NodeController

	informerFactory kubeinformers.SharedInformerFactory

	ac *k8sadapter.AdapterConfig
}

func NewKnode(ctx context.Context, knode *kosmosv1alpha1.Knode, cmdConfig *config.Opts) (*Knode, error) {
	if len(knode.Spec.Kubeconfig) == 0 {
		return nil, fmt.Errorf("kubeconfig of knode %s is empty", knode.Name)
	}

	master, err := utils.NewClientFromConfigPath(cmdConfig.KubeConfigPath, func(config *rest.Config) {
		config.QPS = cmdConfig.KubeAPIQPS
		config.Burst = cmdConfig.KubeAPIBurst
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for master cluster: %v", err)
	}

	client, err := utils.NewClientFromBytes(knode.Spec.Kubeconfig, func(config *rest.Config) {
		config.QPS = knode.Spec.KubeAPIQPS
		config.Burst = knode.Spec.KubeAPIBurst
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for worker cluster %s: %v", knode.Name, err)
	}

	informer := kubeinformers.NewSharedInformerFactory(client, 0)
	podInformer := informer.Core().V1().Pods()
	nsInformer := informer.Core().V1().Namespaces()
	nodeInformer := informer.Core().V1().Nodes()
	cmInformer := informer.Core().V1().ConfigMaps()
	secretInformer := informer.Core().V1().Secrets()
	serviceInformer := informer.Core().V1().Services()

	rm, err := manager.NewResourceManager(podInformer.Lister(), secretInformer.Lister(), cmInformer.Lister(), serviceInformer.Lister())
	if err != nil {
		return nil, errors.Wrap(err, "could not create resource manager")
	}

	ac := &k8sadapter.AdapterConfig{
		Client:            client,
		Master:            master,
		PodInformer:       podInformer,
		NamespaceInformer: nsInformer,
		NodeInformer:      nodeInformer,
		ConfigmapInformer: cmInformer,
		SecretInformer:    secretInformer,
		ServiceInformer:   serviceInformer,
		ResourceManager:   rm,
	}

	var podAdapter adapters.PodHandler
	var nodeAdapter adapters.NodeHandler
	if knode.Spec.Type == kosmosv1alpha1.K8sAdapter {
		podAdapter, err = k8sadapter.NewPodAdapter(ctx, ac, "", &k8sadapter.ClientConfig{}, true)
		if err != nil {
			return nil, err
		}
		nodeAdapter, err = k8sadapter.NewNodeAdapter(ctx, knode, ac, cmdConfig)
		if err != nil {
			return nil, err
		}
	}

	dummyNode := controllers.BuildDummyNode(ctx, knode, nodeAdapter)
	nc, err := controllers.NewNodeController(nodeAdapter, master, dummyNode)
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
		client:          client,
		master:          master,
		informerFactory: informer,
		ac:              ac,
		podController:   pc,
		nodeController:  nc,
	}, nil
}

func (kn *Knode) Run(ctx context.Context, c *config.Opts) {
	kn.informerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(),
		kn.ac.NodeInformer.Informer().HasSynced,
		kn.ac.PodInformer.Informer().HasSynced,
		kn.ac.ConfigmapInformer.Informer().HasSynced,
		kn.ac.NamespaceInformer.Informer().HasSynced,
		kn.ac.SecretInformer.Informer().HasSynced,
	) {
		klogv2.Fatal("nodesInformer waitForCacheSync failed")
	}

	go func() {
		if err := kn.podController.Run(ctx, c.PodSyncWorkers); err != nil && !errors.Is(errors.Cause(err), context.Canceled) {
			klogv2.Fatal(err)
		}
	}()

	go func() {
		if err := kn.nodeController.Run(ctx); err != nil {
			klogv2.Fatal(err)
		}
	}()

	<-ctx.Done()
}
