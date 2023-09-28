package knodemanager

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	klogv2 "k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/cmd/knode-manager/app/config"
	kosmosv1alpha1 "github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
	k8sadapter "github.com/kosmos.io/kosmos/pkg/knode-manager/adapters/k8s"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/controllers"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils"
	"github.com/kosmos.io/kosmos/pkg/knode-manager/utils/manager"
)

const ComponentName = "pod-controller"

type Knode struct {
	client kubernetes.Interface
	master kubernetes.Interface

	podController  *controllers.PodController
	nodeController *controllers.NodeController
	pvController   *controllers.PVPVCController

	clientInformerFactory kubeinformers.SharedInformerFactory
	masterInformerFactory kubeinformers.SharedInformerFactory
	podInformerFactory    kubeinformers.SharedInformerFactory

	ac *k8sadapter.AdapterConfig
}

type Informers struct {
	informer        kubeinformers.SharedInformerFactory
	podInformer     corev1informers.PodInformer
	nsInformer      corev1informers.NamespaceInformer
	nodeInformer    corev1informers.NodeInformer
	cmInformer      corev1informers.ConfigMapInformer
	secretInformer  corev1informers.SecretInformer
	serviceInformer corev1informers.ServiceInformer
}

func NewInformers(client kubernetes.Interface, defaultResync time.Duration) *Informers {
	informer := kubeinformers.NewSharedInformerFactory(client, defaultResync)
	return &Informers{
		informer:        informer,
		podInformer:     informer.Core().V1().Pods(),
		nsInformer:      informer.Core().V1().Namespaces(),
		nodeInformer:    informer.Core().V1().Nodes(),
		cmInformer:      informer.Core().V1().ConfigMaps(),
		secretInformer:  informer.Core().V1().Secrets(),
		serviceInformer: informer.Core().V1().Services(),
	}
}

func NewKnode(ctx context.Context, knode *kosmosv1alpha1.Knode, cmdConfig *config.Opts) (*Knode, error) {
	if len(knode.Spec.Kubeconfig) == 0 {
		return nil, fmt.Errorf("kubeconfig of knode %s is empty", knode.Name)
	}

	// init master client
	master, err := utils.NewClientFromConfigPath(cmdConfig.KubeConfigPath, func(config *rest.Config) {
		config.QPS = cmdConfig.KubeAPIQPS
		config.Burst = cmdConfig.KubeAPIBurst
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for master cluster: %v", err)
	}

	masterInformers := NewInformers(master, cmdConfig.InformerResyncPeriod)

	podInformerForNodeFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		master,
		cmdConfig.InformerResyncPeriod,
		kubeinformers.WithNamespace(cmdConfig.KubeNamespace),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", knode.Spec.NodeName).String()
		}))

	podInformerForNode := podInformerForNodeFactory.Core().V1().Pods()

	rm, err := manager.NewResourceManager(podInformerForNode.Lister(), masterInformers.secretInformer.Lister(), masterInformers.cmInformer.Lister(), masterInformers.serviceInformer.Lister())
	if err != nil {
		return nil, errors.Wrap(err, "could not create resource manager")
	}

	// init adapter client
	client, err := utils.NewClientFromBytes(knode.Spec.Kubeconfig, func(config *rest.Config) {
		config.QPS = knode.Spec.KubeAPIQPS
		config.Burst = knode.Spec.KubeAPIBurst
	})
	if err != nil {
		return nil, fmt.Errorf("could not build clientset for worker cluster %s: %v", knode.Name, err)
	}

	clientInformers := NewInformers(client, cmdConfig.InformerResyncPeriod)

	ac := &k8sadapter.AdapterConfig{
		Client:            client,
		Master:            master,
		PodInformer:       clientInformers.podInformer,
		NamespaceInformer: clientInformers.nsInformer,
		NodeInformer:      clientInformers.nodeInformer,
		ConfigmapInformer: clientInformers.cmInformer,
		SecretInformer:    clientInformers.secretInformer,
		ServiceInformer:   clientInformers.serviceInformer,
		ResourceManager:   rm,
	}

	var podAdapter adapters.PodHandler
	var nodeAdapter adapters.NodeHandler
	if knode.Spec.Type == kosmosv1alpha1.K8sAdapter {
		podAdapter, err = k8sadapter.NewPodAdapter(ctx, ac, "", true)
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

	eb := record.NewBroadcaster()
	eb.StartLogging(klog.Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: master.CoreV1().Events(cmdConfig.KubeNamespace)})

	pc, err := controllers.NewPodController(controllers.PodConfig{
		PodClient:         master.CoreV1(),
		PodInformer:       podInformerForNode,
		EventRecorder:     eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: path.Join(dummyNode.Name, ComponentName)}),
		PodHandler:        podAdapter,
		ConfigMapInformer: masterInformers.cmInformer,
		SecretInformer:    masterInformers.secretInformer,
		ServiceInformer:   masterInformers.serviceInformer,
		RateLimiterOpts:   cmdConfig.RateLimiterOpts,
	})
	if err != nil {
		return nil, err
	}

	pvController, err := controllers.NewPVPVCController(master, client, masterInformers.informer, clientInformers.informer, knode.Name)
	if err != nil {
		return nil, err
	}

	return &Knode{
		client:                client,
		master:                master,
		clientInformerFactory: clientInformers.informer,
		masterInformerFactory: masterInformers.informer,
		podInformerFactory:    podInformerForNodeFactory,
		ac:                    ac,
		podController:         pc,
		nodeController:        nc,
		pvController:          pvController,
	}, nil
}

func (kn *Knode) Run(ctx context.Context, c *config.Opts) {
	go func() {
		if err := kn.podController.Run(ctx, c.PodSyncWorkers); err != nil && !errors.Is(errors.Cause(err), context.Canceled) {
			klogv2.Error(err)
		}
	}()

	go func() {
		if err := kn.nodeController.Run(ctx); err != nil {
			klogv2.Error(err)
		}
	}()

	go func() {
		if err := kn.pvController.Run(ctx); err != nil {
			klogv2.Error(err)
		}
	}()

	kn.clientInformerFactory.Start(ctx.Done())
	kn.masterInformerFactory.Start(ctx.Done())
	kn.podInformerFactory.Start(ctx.Done())

	<-ctx.Done()
}
